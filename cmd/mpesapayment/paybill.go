package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gidyon/micro/v2/pkg/middleware/grpc/auth"
	"github.com/gidyon/micro/v2/utils/errs"
	mpesa "github.com/gidyon/mpesapayments/internal/c2b/v1"
	c2b "github.com/gidyon/mpesapayments/pkg/api/c2b/v1"
	"github.com/gidyon/mpesapayments/pkg/payload"
	"github.com/gidyon/mpesapayments/pkg/utils/httputils"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

type gateway struct {
	C2BServer c2b.LipaNaMPESAServer
	ctxExt    context.Context
	*Options
}

// NewPayBillGateway creates a new mpesa gateway
func NewPayBillGateway(ctx context.Context, opt *Options) (http.Handler, error) {
	err := validateOptions(opt)
	if err != nil {
		return nil, err
	}

	gw := &gateway{
		C2BServer: opt.MpesaAPI,
		Options:   opt,
	}

	// Generate token
	token, err := gw.AuthAPI.GenToken(
		ctx, &auth.Payload{Group: auth.DefaultAdminGroup()}, time.Now().Add(10*365*24*time.Hour))
	if err != nil {
		return nil, fmt.Errorf("failed to generate auth token: %v", err)
	}

	md := metadata.Pairs(auth.Header(), fmt.Sprintf("%s %s", auth.Scheme(), token))

	ctxExt := metadata.NewIncomingContext(ctx, md)

	// Authenticate the token
	gw.ctxExt, err = gw.AuthAPI.AuthorizeFunc(ctxExt)
	if err != nil {
		return nil, err
	}

	return gw, nil
}

func (gw *gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if gw.DisableMpesaService {
		http.Error(w, "receiving LNM transactions disabled", http.StatusServiceUnavailable)
		return
	}

	gw.Logger.Infoln("received mpesa transaction transaction")

	httputils.DumpRequest(r, "Mpesa C2B Payload")

	if r.Method != http.MethodPost {
		http.Error(w, "bad method; only POST allowed", http.StatusBadRequest)
		return
	}

	var (
		err          error
		mpesaPayload = &payload.MpesaPayload{}
	)

	// Marshaling
	switch ctype := r.Header.Get("content-type"); ctype {
	case "application/json", "application/json;charset=UTF-8":
		err = json.NewDecoder(r.Body).Decode(&mpesaPayload)
		if err != nil {
			gw.Logger.Errorf("error while json unmarshaling: %v", err)
			http.Error(w, "decoding json failed: "+err.Error(), http.StatusBadRequest)
			return
		}
	default:
		gw.Logger.Errorf("unknown content type: %v", ctype)
		http.Error(w, "only application/json content type allowed", http.StatusBadRequest)
		return
	}

	// Validation
	switch {
	case mpesaPayload == nil:
		err = fmt.Errorf("nil mpesa transaction")
	case mpesaPayload.BusinessShortCode == "":
		err = fmt.Errorf("missing business short code")
	case mpesaPayload.BillRefNumber == "":
		gw.Logger.Warningln("missing account number")
		mpesaPayload.BillRefNumber = "TILL_PAYMENT"
	case mpesaPayload.MSISDN == "":
		err = fmt.Errorf("missing transaction msisdn")
	case mpesaPayload.TransactionType == "":
		err = fmt.Errorf("missing transaction type")
	case mpesaPayload.TransAmount == "":
		err = fmt.Errorf("missing transaction amount")
	case mpesaPayload.TransTime == "":
		err = errs.MissingField("missing transaction time")
	}
	if err != nil {
		gw.Logger.Errorf("validation error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	transactionTime, err := getTransactionTime(mpesaPayload.TransTime)
	if err != nil {
		gw.Logger.Errorf("failed to get tx time: %v", err)
		http.Error(w, "failed to convert transaction time to unix timestamp: "+err.Error(), http.StatusBadRequest)
		return
	}

	transactionAmount, err := strconv.ParseFloat(mpesaPayload.TransAmount, 32)
	if err != nil {
		gw.Logger.Errorf("failed convert amount: %v", err)
		http.Error(w, "failed to convert transaction amount to float: "+err.Error(), http.StatusBadRequest)
		return
	}

	businessShortCode, err := strconv.ParseInt(mpesaPayload.BusinessShortCode, 10, 32)
	if err != nil {
		gw.Logger.Errorf("failed to parse business short code: %v", err)
		http.Error(w, "failed to convert business short code to integer: "+err.Error(), http.StatusBadRequest)
		return
	}

	orgBalance, err := strconv.ParseFloat(mpesaPayload.OrgAccountBalance, 32)
	if err != nil {
		gw.Logger.Errorf("failed to parse org balance: %v", err)
		http.Error(w, "failed to convert business short code to integer: "+err.Error(), http.StatusBadRequest)
		return
	}

	C2BPB := &c2b.C2BPayment{
		TransactionId:          mpesaPayload.TransID,
		TransactionType:        mpesaPayload.TransactionType,
		TransactionTimeSeconds: transactionTime.Unix(),
		Msisdn:                 mpesaPayload.MSISDN,
		Names:                  fmt.Sprintf("%s %s", mpesaPayload.FirstName, mpesaPayload.LastName),
		RefNumber:              mpesaPayload.BillRefNumber,
		Amount:                 float32(transactionAmount),
		OrgBalance:             float32(orgBalance),
		BusinessShortCode:      int32(businessShortCode),
		Processed:              false,
	}

	// Save to database
	_, err = gw.C2BServer.CreateC2BPayment(
		gw.ctxExt, &c2b.CreateC2BPaymentRequest{
			MpesaPayment: C2BPB,
		})
	if err != nil {
		gw.Logger.Errorf("failed to save mpesa payment: %v", err)
		bs, err := proto.Marshal(C2BPB)
		if err == nil {
			gw.RedisDB.LPush(r.Context(), mpesa.FailedTxList, bs)
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = w.Write([]byte("mpesa transaction processed"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func getTransactionTime(transactionTimeStr string) (time.Time, error) {
	// 20200816204116
	if len(transactionTimeStr) != 14 {
		return time.Now(), nil
	}

	timeRFC3339Str := fmt.Sprintf(
		"%s-%s-%sT%s:%s:%s+00:00",
		transactionTimeStr[:4],    // year
		transactionTimeStr[4:6],   // month
		transactionTimeStr[6:8],   // day
		transactionTimeStr[8:10],  // hour
		transactionTimeStr[10:12], // minutes
		transactionTimeStr[12:],   // seconds
	)

	return time.Parse(time.RFC3339, timeRFC3339Str)
}
