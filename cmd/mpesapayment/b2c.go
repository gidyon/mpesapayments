package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"encoding/json"
	"errors"

	"github.com/gidyon/micro/v2/pkg/middleware/grpc/auth"
	b2capp "github.com/gidyon/mpesapayments/internal/b2c/v1"
	b2c "github.com/gidyon/mpesapayments/pkg/api/b2c/v1"
	"github.com/gidyon/mpesapayments/pkg/utils/httputils"
	"google.golang.org/grpc/metadata"

	"github.com/gidyon/mpesapayments/pkg/payload"
	"github.com/go-redis/redis/v8"
	"google.golang.org/protobuf/proto"
)

type b2cGateway struct {
	b2cAPI b2c.B2CAPIServer
	ctxExt context.Context
	*Options
}

// NewB2CGateway creates a b2c gateway for receiving transactions
func NewB2CGateway(ctx context.Context, opt *Options) (http.Handler, error) {
	err := validateOptions(opt)
	if err != nil {
		return nil, err
	}

	gw := &b2cGateway{
		b2cAPI:  opt.B2CAPI,
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

func (gw *b2cGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if gw.DisableB2CService {
		http.Error(w, "receiving b2c transactions disabled", http.StatusServiceUnavailable)
		return
	}

	var (
		source = r.URL.Query().Get(b2capp.SourceKey)
	)

	if source == "DARAJA" {
		code, err := gw.fromSaf(w, r)
		if err != nil {
			http.Error(w, "request handler failed", code)
			gw.Logger.Errorln("error handling request from safaricom: %s", err)
			return
		}
	} else {
		gw.fromOnfon(w, r)
	}
}

func (gw *b2cGateway) fromSaf(w http.ResponseWriter, r *http.Request) (int, error) {

	httputils.DumpRequest(r, "Incoming Mpesa B2C Payload")

	// Must be POST request
	if r.Method != http.MethodPost {
		return http.StatusBadRequest, fmt.Errorf("bad method; only POST allowed; received %v method", r.Method)
	}

	var (
		transaction = &payload.Transaction{}
		err         error
	)

	// Request must have content-type as application/json
	switch strings.ToLower(r.Header.Get("content-type")) {
	case "application/json", "application/json;charset=utf-8", "application/json;charset=utf8":
		err = json.NewDecoder(r.Body).Decode(transaction)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("decoding json failed: %w", err)
		}
	default:
		ctype := r.Header.Get("content-type")
		return http.StatusBadRequest, fmt.Errorf("incorrect content type: %s", ctype)
	}

	// Validation of request body payload
	switch {
	case transaction == nil:
		err = fmt.Errorf("nil mpesa transaction")
	case transaction.Result.ConversationID == "":
		err = fmt.Errorf("missing conversation id")
	case transaction.Result.OriginatorConversationID == "":
		err = fmt.Errorf("missing originator id")
	case transaction.Result.ResultDesc == "":
		err = fmt.Errorf("missing description")
	}
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("validation error: %s", err)
	}

	initReq := &b2c.TransferFundsRequest{}

	ctx := r.Context()

	res, err := gw.RedisDB.Get(ctx, b2capp.GetMpesaRequestKey(transaction.ConversationID())).Result()
	switch {
	case err == nil:
		err = proto.Unmarshal([]byte(res), initReq)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("failed to unmarshal initiator: %v", err)
		}
	case errors.Is(err, redis.Nil):
	default:
	}

	// Transaction payload
	pb := &b2c.B2CPayment{
		InitiatorId:              initReq.InitiatorId,
		OrgShortCode:             fmt.Sprint(initReq.ShortCode),
		Msisdn:                   transaction.MSISDN(),
		ReceiverPartyPublicName:  transaction.ReceiverPartyPublicName(),
		TransactionType:          initReq.CommandId.String(),
		TransactionId:            transaction.TransactionReceipt(),
		ConversationId:           transaction.Result.ConversationID,
		OriginatorConversationId: transaction.Result.OriginatorConversationID,
		ResultCode:               fmt.Sprint(transaction.Result.ResultCode),
		ResultDescription:        transaction.Result.ResultDesc,
		TransactionTimestamp:     transaction.TransactionCompletedDateTime().UTC().Unix(),
		Amount:                   float32(transaction.TransactionAmount()),
		WorkingAccountFunds:      float32(transaction.B2CWorkingAccountAvailableFunds()),
		UtilityAccountFunds:      float32(transaction.B2CUtilityAccountAvailableFunds()),
		MpesaCharges:             float32(transaction.B2CChargesPaidAccountAvailableFunds()),
		OnfonCharges:             gw.B2CTransactionCharges,
		RecipientRegistered:      transaction.B2CRecipientIsRegisteredCustomer(),
		Succeeded:                transaction.Succeeded(),
		Processed:                false,
	}

	// We save the transaction
	pb, err = gw.b2cAPI.CreateB2CPayment(gw.ctxExt, &b2c.CreateB2CPaymentRequest{
		Payment: pb,
	})
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to create b2c payment: %v", err)
	}

	if initReq.Publish {
		publish := func() {
			_, err = gw.b2cAPI.PublishB2CPayment(gw.ctxExt, &b2c.PublishB2CPaymentRequest{
				PaymentId:   pb.PaymentId,
				InitiatorId: initReq.InitiatorId,
			})
			if err != nil {
				gw.Logger.Warningf("failed to publish message: %v", err)
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

	_, err = w.Write([]byte("mpesa b2c transaction processed"))
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}

func (gw *b2cGateway) fromOnfon(w http.ResponseWriter, r *http.Request) {

	// gw.Logger.Infoln("received b2c transaction from onfon")

	// httputils.DumpRequest(r, "Safaricom B2C Payload")

	// // Must be POST request
	// if r.Method != http.MethodPost {
	// 	gw.Logger.Infof("only POST allowed; received %v http method", r.Method)
	// 	http.Error(w, fmt.Sprintf("bad method; only POST allowed; received %v method", r.Method), http.StatusBadRequest)
	// 	return
	// }

	// var (
	// 	transaction = &payload.IncomingTransactionOnfon{}
	// 	ctx         = r.Context()
	// 	err         error
	// )

	// switch r.Header.Get("content-type") {
	// case "application/json", "application/json;charset=UTF-8":
	// 	// Marshaling
	// 	err = json.NewDecoder(r.Body).Decode(transaction)
	// 	if err != nil {
	// 		gw.Logger.Errorf("error decoding json in response: %v", err)
	// 		http.Error(w, "decoding json failed: "+err.Error(), http.StatusBadRequest)
	// 		return
	// 	}
	// default:
	// 	ctype := r.Header.Get("content-type")
	// 	http.Error(w, fmt.Sprintf("incorrect content-type: %s", ctype), http.StatusBadRequest)
	// 	gw.Logger.Warningln("incorrect content type: %s", ctype)
	// 	return
	// }

	// // Validation
	// switch {
	// case transaction == nil:
	// 	err = fmt.Errorf("nil mpesa transaction")
	// case transaction.ConversationID == "":
	// 	err = fmt.Errorf("missing conversation id")
	// case transaction.OriginatorConversationID == "":
	// 	err = fmt.Errorf("missing originator id")
	// case transaction.ResultDesc == "":
	// 	err = fmt.Errorf("missing description")
	// }
	// if err != nil {
	// 	gw.Logger.Errorf("validation error: %v", err)
	// 	http.Error(w, err.Error(), http.StatusBadRequest)
	// 	return
	// }

	// initiator := &b2c.InitiatorPayload{}

	// // Get initiator
	// key := b2capp.GetInitiatorKey(transaction.MSISDN())

	// res, err := gw.RedisDB.Get(ctx, key).Result()
	// switch {
	// case err == nil:
	// 	err = proto.Unmarshal([]byte(res), initiator)
	// 	if err != nil {
	// 		gw.Logger.Errorf("failed to unmarshal initiator: %v", err)
	// 		http.Error(w, err.Error(), http.StatusBadRequest)
	// 		return
	// 	}
	// case errors.Is(err, redis.Nil):
	// default:
	// 	gw.Logger.Errorf("failed to get initiator from cache: %v", err)
	// 	http.Error(w, err.Error(), http.StatusBadRequest)
	// 	return
	// }

	// if initiator.PublishLocal {
	// 	// Tell local consumers that they can unsubscribe
	// 	defer func() {
	// 		gw.RedisDB.Publish(ctx, gw.Options.B2CLocalTopic, initiator.RequestId)
	// 	}()
	// }

	// // Transaction payload
	// pb := &b2c.B2CPayment{
	// 	InitiatorId:              initiator.InitiatorId,
	// 	OrgShortCode:             initiator.ShortCode,
	// 	Msisdn:                   transaction.MSISDN(),
	// 	ReceiverPartyPublicName:  transaction.ReceiverPartyPublicName,
	// 	TransactionType:          initiator.TransactionType,
	// 	TransactionId:            transaction.TransactionID,
	// 	ConversationId:           transaction.ConversationID,
	// 	OriginatorConversationId: transaction.OriginatorConversationID,
	// 	ResultCode:               fmt.Sprint(transaction.ResultCode),
	// 	ResultDescription:        transaction.ResultDesc,
	// 	TransactionTimestamp:     transaction.CompletedDateTime().Unix(),
	// 	Amount:                   float32(transaction.Amount()),
	// 	WorkingAccountFunds:      float32(transaction.B2CUtilityAccountAvailableFundsV2()),
	// 	RecipientRegistered:      transaction.B2CRecipientIsRegisteredCustomerV2(),
	// 	OnfonCharges:             gw.B2CTransactionCharges,
	// 	Succeeded:                transaction.Succeeded(),
	// }

	// if initiator.PublishLocal && initiator.RequestId != "" {
	// 	// Marshal
	// 	bs, err := proto.Marshal(pb)
	// 	if err != nil {
	// 		http.Error(w, "marshaling b2c payload not succeesful", http.StatusInternalServerError)
	// 		gw.Logger.Errorln("marshaling b2c payload not succeesful: %s", err)
	// 		return
	// 	}

	// 	// Set transaction in cache
	// 	err = gw.RedisDB.Set(ctx, initiator.RequestId, bs, 5*time.Minute).Err()
	// 	if err != nil {
	// 		http.Error(w, "failed to set b2c transaction in cache", http.StatusInternalServerError)
	// 		gw.Logger.Errorln("failed to set b2c transaction in cache: %s", err)
	// 		return
	// 	}

	// 	// Publish the transaction for local goroutines
	// 	err = gw.RedisDB.Publish(ctx, gw.B2CLocalTopic, initiator.RequestId).Err()
	// 	if err != nil {
	// 		http.Error(w, "failed to publish b2c to local listeners", http.StatusInternalServerError)
	// 		gw.Logger.Errorln("failed to publish b2c transaction to local listeners: %s", err)
	// 		return
	// 	}
	// }

	// if !initiator.DropTransaction {
	// 	// We save the transaction
	// 	_, err = gw.b2cAPI.CreateB2CPayment(gw.ctxExt, &b2c.CreateB2CPaymentRequest{
	// 		Payment: pb,
	// 		Publish: initiator.PublishOnCreate,
	// 	})
	// 	if err != nil {
	// 		gw.Logger.Errorf("failed to create b2c payment: %v", err)
	// 		http.Error(w, err.Error(), http.StatusInternalServerError)

	// 		// Save in cache for later processing
	// 		bs, err := proto.Marshal(pb)
	// 		if err == nil {
	// 			gw.RedisDB.LPush(r.Context(), b2capp.FailedTxList, bs)
	// 		}
	// 		return
	// 	}
	// }

	// _, err = w.Write([]byte("mpesa b2c transaction processed"))
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// }
}
