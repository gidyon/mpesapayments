package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	mpesa "bitbucket.org/gideonkamau/mpesa-tracking-portal/internal/mpesapayment"
	"bitbucket.org/gideonkamau/mpesa-tracking-portal/pkg/api/mpesapayment"
	"github.com/gidyon/services/pkg/auth"
	"github.com/gidyon/services/pkg/utils/errs"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

type stkGateway struct {
	authAPI            auth.Interface
	mpesaPaymentServer mpesapayment.LipaNaMPESAServer
	*Options
}

// NewSTKGateway creates a new mpesa stkGateway
func NewSTKGateway(ctx context.Context, opt *Options) (http.Handler, error) {
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
		err = errs.NilObject("Logger")
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

	gw := &stkGateway{
		authAPI:            authAPI,
		mpesaPaymentServer: opt.MpesaAPI,
		Options:            opt,
	}

	return gw, nil
}

func (gw *stkGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error

	gw.Logger.Infoln("received request from safaricom mpesa")

	// Must be POST request
	if r.Method != http.MethodPost {
		gw.Logger.Infoln("only post allowed")
		http.Error(w, "bad method; only POST allowed", http.StatusBadRequest)
		return
	}

	stkPayload := &STKPayload{}

	switch r.Header.Get("content-type") {
	case "application/json":
		// Marshaling
		err = json.NewDecoder(r.Body).Decode(stkPayload)
		if err != nil {
			gw.Logger.Errorf("error is %v", err)
			http.Error(w, "decoding json failed: "+err.Error(), http.StatusBadRequest)
			return
		}
	default:
		gw.Logger.Warningln("incorrect  content type: %v", r.Header.Get("content-type"))
		return
	}

	// Validation
	switch {
	case stkPayload == nil:
		err = fmt.Errorf("nil mpesa transaction")
	case stkPayload.Body.STKCallback.CheckoutRequestID == "":
		err = fmt.Errorf("missing checkout id")
	case stkPayload.Body.STKCallback.MerchantRequestID == "":
		err = fmt.Errorf("missing merchant id")
	case stkPayload.Body.STKCallback.ResultDesc == "":
		err = fmt.Errorf("missing description")
	}
	if err != nil {
		gw.Logger.Errorf("error is %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if stkPayload.Body.STKCallback.ResultCode != 0 {
		gw.Logger.Errorln("stk mpesa transaction failed")
		w.Write([]byte("stk mpesa transaction failed"))
		return
	}

	mpesaPaymentPB := &mpesapayment.MPESAPayment{
		TxId:              stkPayload.Body.STKCallback.CheckoutRequestID,
		TxType:            "STK_PUSH",
		TxTimestamp:       stkPayload.Body.STKCallback.CallbackMetadata.GetTransTime().Unix(),
		Msisdn:            stkPayload.Body.STKCallback.CallbackMetadata.PhoneNumber(),
		Names:             stkPayload.Body.STKCallback.CallbackMetadata.PhoneNumber(),
		TxRefNumber:       stkPayload.Body.STKCallback.CallbackMetadata.MpesaReceiptNumber(),
		TxAmount:          stkPayload.Body.STKCallback.CallbackMetadata.GetAmount(),
		BusinessShortCode: 0,
		Processed:         true,
	}

	token, err := gw.authAPI.GenToken(r.Context(), &auth.Payload{Group: auth.AdminGroup()}, time.Now().Add(time.Minute))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		gw.Logger.Errorln(err.Error())
		return
	}

	md := metadata.Pairs(auth.Header(), fmt.Sprintf("%s %s", auth.Scheme(), token))

	// Save to database
	_, err = gw.mpesaPaymentServer.CreateMPESAPayment(metadata.NewIncomingContext(r.Context(), md), &mpesapayment.CreateMPESAPaymentRequest{
		MpesaPayment: mpesaPaymentPB,
	})
	if err != nil {
		bs, err2 := proto.Marshal(mpesaPaymentPB)
		if err2 == nil {
			gw.RedisDB.LPush(mpesa.FailedTxList, bs)
		}
		gw.Logger.Errorln(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write([]byte("mpesa transaction processed"))
}

// STKPayload sent from stk push
type STKPayload struct {
	Body struct {
		STKCallback struct {
			MerchantRequestID string       `json:"MerchantRequestID,omitempty"`
			CheckoutRequestID string       `json:"CheckoutRequestID,omitempty"`
			ResultCode        int          `json:"ResultCode,omitempty"`
			ResultDesc        string       `json:"ResultDesc,omitempty"`
			CallbackMetadata  CallbackMeta `json:"CallbackMetadata,omitempty"`
		} `json:"stkCallback,omitempty"`
	} `json:"Body,omitempty"`
}

// CallbackMeta is response body for successful response
type CallbackMeta struct {
	Item []struct {
		Name  string      `json:"Name,omitempty"`
		Value interface{} `json:"Value,omitempty"`
	} `json:"Item,omitempty"`
}

// GetTransTime returns the transaction time
func (c *CallbackMeta) GetTransTime() time.Time {
	if len(c.Item) != 5 {
		return time.Now()
	}

	t, err := getTransactionTime(fmt.Sprint(c.Item[3].Value))
	if err != nil {
		t = time.Now()
	}
	return t
}

// GetAmount returns the transaction amount
func (c *CallbackMeta) GetAmount() float32 {
	if len(c.Item) != 5 {
		return 0
	}

	v, ok := c.Item[0].Value.(float64)
	if !ok {
		return 0
	}
	return float32(v)
}

// PhoneNumber returns the phone number
func (c *CallbackMeta) PhoneNumber() string {
	if len(c.Item) != 5 {
		return ""
	}

	v, _ := c.Item[4].Value.(float64)
	return fmt.Sprintf("%.0f", v)
}

// MpesaReceiptNumber returns the receipt number
func (c *CallbackMeta) MpesaReceiptNumber() string {
	if len(c.Item) != 5 {
		return ""
	}

	return fmt.Sprint(c.Item[1].Value)
}

// {
// 	"Body":
// 	{"stkCallback":
// 	 {
// 	  "MerchantRequestID": "21605-295434-4",
// 	  "CheckoutRequestID": "ws_CO_04112017184930742",
// 	  "ResultCode": 0,
// 	  "ResultDesc": "The service request is processed successfully.",
// 	  "CallbackMetadata":
// 	   {
// 		"Item":
// 		[
// 		{
// 		  "Name": "Amount",
// 		  "Value": 1
// 		},
// 		{
// 		  "Name": "MpesaReceiptNumber",
// 		  "Value": "LK451H35OP"
// 		},
// 		{
// 		  "Name": "Balance"
// 		},
// 		{
// 		  "Name": "TransactionDate",
// 		  "Value": 20171104184944
// 		 },
// 		{
// 		  "Name": "PhoneNumber",
// 		  "Value": 254727894083
// 		}
// 		]
// 	   }
// 	 }
// 	}
//    }
