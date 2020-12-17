package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gidyon/micro/pkg/grpc/auth"
	stkapp "github.com/gidyon/mpesapayments/internal/stk"
	"github.com/gidyon/mpesapayments/pkg/api/stk"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

type stkGateway struct {
	stkAPI stk.StkPushAPIServer
	ctxExt context.Context
	*Options
}

// NewSTKGateway creates a new mpesa stkGateway
func NewSTKGateway(ctx context.Context, opt *Options) (http.Handler, error) {
	err := validateOptions(opt)
	if err != nil {
		return nil, err
	}

	gw := &stkGateway{
		stkAPI:  opt.StkAPI,
		Options: opt,
	}

	// Generate token
	token, err := gw.AuthAPI.GenToken(
		ctx, &auth.Payload{Group: auth.AdminGroup()}, time.Now().Add(10*365*24*time.Hour))
	if err != nil {
		return nil, fmt.Errorf("failed to generate auth token: %v", err)
	}

	md := metadata.Pairs(auth.Header(), fmt.Sprintf("%s %s", auth.Scheme(), token))

	ctxExt := metadata.NewIncomingContext(ctx, md)

	// Authenticate the token
	gw.ctxExt, err = gw.AuthAPI.AuthFunc(ctxExt)
	if err != nil {
		return nil, err
	}

	return gw, nil
}

func (gw *stkGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if gw.DisableSTKService {
		http.Error(w, "receiving stk transactions disabled", http.StatusServiceUnavailable)
		return
	}

	var err error

	gw.Logger.Infoln("received stk request from mpesa")

	// Must be POST request
	if r.Method != http.MethodPost {
		gw.Logger.Infoln("only post allowed")
		http.Error(w, "bad method; only POST allowed", http.StatusBadRequest)
		return
	}

	stkPayload := &STKPayload{}

	switch r.Header.Get("content-type") {
	case "application/json", "application/json;charset=UTF-8":
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
		gw.Logger.Errorf("validation error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	stkPayloadPB := &stk.StkPayload{
		MerchantRequestId:  stkPayload.Body.STKCallback.MerchantRequestID,
		CheckoutRequestId:  stkPayload.Body.STKCallback.CheckoutRequestID,
		ResultCode:         fmt.Sprint(stkPayload.Body.STKCallback.ResultCode),
		ResultDesc:         stkPayload.Body.STKCallback.ResultDesc,
		Amount:             fmt.Sprintf("%.2f", stkPayload.Body.STKCallback.CallbackMetadata.GetAmount()),
		MpesaReceiptNumber: stkPayload.Body.STKCallback.CallbackMetadata.MpesaReceiptNumber(),
		TransactionDate:    stkPayload.Body.STKCallback.CallbackMetadata.GetTransTime().UTC().String(),
		PhoneNumber:        stkPayload.Body.STKCallback.CallbackMetadata.PhoneNumber(),
		Succeeded:          stkPayload.Body.STKCallback.ResultCode == 0,
	}

	// Save only if the transaction was successful
	if !stkPayloadPB.Succeeded {
		w.Write([]byte("stk transaction not successful"))
		gw.Logger.Warningf("stk not successful: %s", stkPayloadPB.ResultDesc)
		return
	}

	// Update initiator id
	stkPayloadPB.InitiatorId, err = gw.RedisDB.Get(r.Context(), stkapp.GetMpesaSTKPushKey(stkPayloadPB.PhoneNumber)).Result()
	if err != nil {
		gw.Logger.Warningf("failed to get initiator id for stk: %v", err)
	}

	// Save to database
	_, err = gw.stkAPI.CreateStkPayload(
		gw.ctxExt, &stk.CreateStkPayloadRequest{Payload: stkPayloadPB},
	)
	if err != nil {
		gw.Logger.Errorf("failed to create stk payload: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)

		bs, err := proto.Marshal(stkPayloadPB)
		if err == nil {
			if !gw.DisablePublishing {
				gw.RedisDB.LPush(r.Context(), stkapp.FailedTxList, bs)
			}
		}

		return
	}

	w.Write([]byte("mpesa stk processed"))
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

// GetAmount returns the transaction amount
func (c *CallbackMeta) GetAmount() float32 {
	itemsLen := len(c.Item)
	if itemsLen != 5 && itemsLen != 4 {
		return 0
	}

	v, ok := c.Item[0].Value.(float64)
	if !ok {
		return 0
	}

	return float32(v)
}

// MpesaReceiptNumber returns the receipt number
func (c *CallbackMeta) MpesaReceiptNumber() string {
	itemsLen := len(c.Item)
	if itemsLen != 5 && itemsLen != 4 {
		return ""
	}

	return fmt.Sprint(c.Item[1].Value)
}

// GetTransTime returns the transaction time
func (c *CallbackMeta) GetTransTime() time.Time {
	itemsLen := len(c.Item)
	if itemsLen != 5 && itemsLen != 4 {
		return time.Now()
	}

	var (
		t   time.Time
		err error
	)

	switch itemsLen {
	case 4:
		t, err = getTransactionTime(fmt.Sprint(c.Item[2].Value))
		if err != nil {
			t = time.Now()
		}
	case 5:
		t, err = getTransactionTime(fmt.Sprint(c.Item[3].Value))
		if err != nil {
			t = time.Now()
		}
	}

	return t
}

// PhoneNumber returns the phone number
func (c *CallbackMeta) PhoneNumber() string {
	itemsLen := len(c.Item)
	if itemsLen != 5 && itemsLen != 4 {
		return ""
	}

	var v float64

	switch itemsLen {
	case 4:
		v, _ = c.Item[3].Value.(float64)
	case 5:
		v, _ = c.Item[4].Value.(float64)
	}

	return fmt.Sprintf("%.0f", v)
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
