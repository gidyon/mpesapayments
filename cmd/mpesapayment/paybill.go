package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	mpesa "github.com/gidyon/mpesapayments/internal/mpesapayment"
	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"github.com/gidyon/mpesapayments/pkg/api/stk"
	"github.com/gidyon/services/pkg/auth"
	"github.com/gidyon/services/pkg/utils/errs"
	"github.com/go-redis/redis"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// Options contains parameters passed to NewUSSDGateway
type Options struct {
	SQLDB         *gorm.DB
	RedisDB       *redis.Client
	Logger        grpclog.LoggerV2
	JWTSigningKey []byte
	MpesaAPI      mpesapayment.LipaNaMPESAServer
	StkAPI        stk.StkPushAPIServer
}

type gateway struct {
	authAPI            auth.Interface
	mpesaPaymentServer mpesapayment.LipaNaMPESAServer
	*Options
}

// NewPayBillGateway creates a new mpesa gateway
func NewPayBillGateway(ctx context.Context, opt *Options) (http.Handler, error) {
	// Validate
	var err error
	switch {
	case ctx == nil:
		err = errs.NilObject("context")
	case opt == nil:
		err = errs.NilObject("options")
	case opt.SQLDB == nil:
		err = errs.NilObject("sqlDB")
	case opt.RedisDB == nil:
		err = errs.NilObject("redisDB")
	case opt.Logger == nil:
		err = errs.NilObject("logger")
	case opt.JWTSigningKey == nil:
		err = errs.NilObject("jwt key")
	case opt.MpesaAPI == nil:
		err = errs.NilObject("mpesa server")
	}
	if err != nil {
		return nil, err
	}

	// Authentication API
	authAPI, err := auth.NewAPI(opt.JWTSigningKey, "USSD Channel API", "users")
	if err != nil {
		return nil, err
	}

	gw := &gateway{
		authAPI:            authAPI,
		mpesaPaymentServer: opt.MpesaAPI,
		Options:            opt,
	}

	gw.printToken()

	return gw, nil
}

func (gw *gateway) printToken() {
	token, err := gw.authAPI.GenToken(context.Background(), &auth.Payload{Group: auth.AdminGroup()}, time.Now().Add(time.Hour*365))
	if err != nil {
		gw.Logger.Errorf("failed to generate auth token: %v", err)
		return
	}
	gw.Logger.Infof("jwt is %s", token)
}

func (gw *gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	gw.Logger.Infoln("received mpesa transaction transaction")

	var err error

	if r.Method != http.MethodPost {
		http.Error(w, "bad method; only POST allowed", http.StatusBadRequest)
		return
	}

	mpesaPayload := &MpesaPayload{}

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
		err = fmt.Errorf("missing account number")
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

	mpesaPaymentPB := &mpesapayment.MPESAPayment{
		TxId:              mpesaPayload.TransID,
		TxType:            "PAY_BILL",
		TxTimestamp:       transactionTime.Unix(),
		Msisdn:            mpesaPayload.MSISDN,
		Names:             fmt.Sprintf("%s %s", mpesaPayload.FirstName, mpesaPayload.LastName),
		TxRefNumber:       mpesaPayload.BillRefNumber,
		TxAmount:          float32(transactionAmount),
		OrgBalance:        float32(orgBalance),
		BusinessShortCode: int32(businessShortCode),
		Processed:         false,
	}

	token, err := gw.authAPI.GenToken(r.Context(), &auth.Payload{Group: auth.AdminGroup()}, time.Now().Add(time.Minute))
	if err != nil {
		gw.Logger.Errorf("failed to generate auth token: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	md := metadata.Pairs(auth.Header(), fmt.Sprintf("%s %s", auth.Scheme(), token))

	ctxExt := metadata.NewIncomingContext(r.Context(), md)

	// Save to database
	createRes, err := gw.mpesaPaymentServer.CreateMPESAPayment(ctxExt, &mpesapayment.CreateMPESAPaymentRequest{
		MpesaPayment: mpesaPaymentPB,
	})
	if err != nil {
		gw.Logger.Errorf("failed to save mpesa payment: %v", err)
		bs, err2 := proto.Marshal(mpesaPaymentPB)
		if err2 == nil {
			gw.Logger.Errorf("failed to push tx to redis: %v", err)
			gw.RedisDB.LPush(mpesa.FailedTxList, bs)
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go func() {
		// Publish the transaction
		_, err := gw.MpesaAPI.PublishMpesaPayment(ctxExt, &mpesapayment.PublishMpesaPaymentRequest{
			PaymentId:   createRes.PaymentId,
			InitiatorId: "",
		})
		if err != nil {
			gw.Logger.Errorf("failed to publish lnm payment with id: %s", createRes.PaymentId)
			return
		}
	}()

	w.Write([]byte("mpesa transaction processed"))
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

// MpesaPayload contains mpesa transaction payload for paybill
type MpesaPayload struct {
	TransactionType   string
	TransID           string
	TransTime         string
	TransAmount       string
	BusinessShortCode string
	BillRefNumber     string
	InvoiceNumber     string
	OrgAccountBalance string
	ThirdPartyTransID string
	MSISDN            string
	FirstName         string
	MiddleName        string
	LastName          string
}

// [
// 	BillRefNumber:pwo
// 	BusinessShortCode:4045065
// 	FirstName:GIDEON
// 	InvoiceNumber:
// 	LastName:NG'ANG'A
// 	MSISDN:254716484395
// 	MiddleName:KAMAU
// 	OrgAccountBalance:26.00
// 	ThirdPartyTransID:
// 	TransAmount:2.00
// 	TransID:OIU61LFLVC
// 	TransTime:20200930171653
// 	TransactionType:Pay Bill
// ]
