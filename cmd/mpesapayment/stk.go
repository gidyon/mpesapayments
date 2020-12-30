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
	"github.com/gidyon/mpesapayments/pkg/payload"
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

	stkPayload := &payload.STKPayload{}

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
	stkPayloadPB.InitiatorId, err = gw.RedisDB.Get(r.Context(), stkapp.GetMpesaSTKPushKey(stkPayloadPB.PhoneNumber, gw.RedisKeyPrefix)).Result()
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
