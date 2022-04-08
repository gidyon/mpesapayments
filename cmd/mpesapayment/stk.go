package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gidyon/micro/v2/pkg/middleware/grpc/auth"
	stkapp "github.com/gidyon/mpesapayments/internal/stk/v1"
	stk "github.com/gidyon/mpesapayments/pkg/api/stk/v1"
	"github.com/gidyon/mpesapayments/pkg/payload"
	"github.com/gidyon/mpesapayments/pkg/utils/httputils"
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

func (gw *stkGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code, err := gw.serveHTTP(w, r)
	if err != nil {
		gw.Logger.Errorf("Error serving incoming Stk Transaction %v", err)
		http.Error(w, "request handler failed", code)
	}
}

func (gw *stkGateway) serveHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	if gw.DisableSTKService {
		http.Error(w, "receiving stk transactions disabled", http.StatusServiceUnavailable)
		return http.StatusServiceUnavailable, errors.New("receiving stk transactions disabled")
	}

	httputils.DumpRequest(r, "Incoming Mpesa STK Payload")

	if r.Method != http.MethodPost {
		return http.StatusBadRequest, fmt.Errorf("bad method; only POST allowed; received %v method", r.Method)
	}

	var (
		err        error
		stkPayload = &payload.STKPayload{}
	)

	switch r.Header.Get("content-type") {
	case "application/json", "application/json;charset=UTF-8":
		err = json.NewDecoder(r.Body).Decode(stkPayload)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("decoding json failed: %w", err)
		}
	default:
		return http.StatusBadRequest, fmt.Errorf("incorrect content type: %v", r.Header.Get("content-type"))
	}

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
		return http.StatusBadRequest, fmt.Errorf("request body validation error: %v", err)
	}

	pb := &stk.StkTransaction{
		InitiatorId:          "",
		TransactionId:        stkPayload.Body.STKCallback.CallbackMetadata.MpesaReceiptNumber(),
		MerchantRequestId:    stkPayload.Body.STKCallback.MerchantRequestID,
		CheckoutRequestId:    stkPayload.Body.STKCallback.CheckoutRequestID,
		ResultCode:           fmt.Sprint(stkPayload.Body.STKCallback.ResultCode),
		ResultDesc:           stkPayload.Body.STKCallback.ResultDesc,
		Amount:               fmt.Sprintf("%.2f", stkPayload.Body.STKCallback.CallbackMetadata.GetAmount()),
		MpesaReceiptId:       stkPayload.Body.STKCallback.CallbackMetadata.MpesaReceiptNumber(),
		Balance:              stkPayload.Body.STKCallback.CallbackMetadata.Balance(),
		PhoneNumber:          stkPayload.Body.STKCallback.CallbackMetadata.PhoneNumber(),
		Succeeded:            stkPayload.Body.STKCallback.ResultCode == 0,
		Processed:            false,
		TransactionTimestamp: stkPayload.Body.STKCallback.CallbackMetadata.GetTransTime().Unix(),
		CreateTimestamp:      0,
	}

	initReq := &stk.InitiateSTKPushRequest{}

	ctx := r.Context()

	bs, err := gw.RedisDB.Get(ctx, stkapp.GetMpesaRequestKey(stkPayload.Body.STKCallback.MerchantRequestID)).Result()
	switch {
	case err == nil:
		err = proto.Unmarshal([]byte(bs), initReq)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("failed to unmarshal initiate stk request: %v", err)
		}
	}

	pb.InitiatorId = initReq.InitiatorId

	pb.AccountReference = initReq.AccountReference
	pb.TransactionDesc = initReq.TransactionDesc
	pb.ShortCode = initReq.GetPublishMessage().GetPayload()["short_code"]
	if !pb.Succeeded {
		pb.PhoneNumber = firstVal(pb.PhoneNumber, initReq.GetPhone())
		pb.Amount = firstVal(pb.Amount, fmt.Sprint(initReq.GetAmount()))
	}

	res, err := gw.stkAPI.CreateStkTransaction(
		gw.ctxExt, &stk.CreateStkTransactionRequest{Payload: pb},
	)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to save stk transaction: %v", err)
	}

	if initReq.Publish {
		publish := func() {
			_, err = gw.stkAPI.PublishStkTransaction(gw.ctxExt, &stk.PublishStkTransactionRequest{
				PublishMessage: &stk.PublishMessage{
					InitiatorId:     initReq.InitiatorId,
					TransactionId:   res.TransactionId,
					MpesaReceiptId:  res.MpesaReceiptId,
					PhoneNumber:     res.PhoneNumber,
					PublishInfo:     initReq.PublishMessage,
					TransactionInfo: res,
				},
			})
			if err != nil {
				gw.Logger.Warningf("failed to publish message: %v", err)
			} else {
				gw.Logger.Infoln("stk published ", firstVal(initReq.GetPublishMessage().GetChannelName()))
			}
		}
		if initReq.GetPublishMessage().GetOnlyOnSuccess() {
			if pb.Succeeded {
				publish()
			}
		} else {
			publish()
		}
	}

	_, err = w.Write([]byte("mpesa stk processed"))
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}
