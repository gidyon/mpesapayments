package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"encoding/json"

	"github.com/gidyon/micro/v2/pkg/middleware/grpc/auth"
	b2c_model "github.com/gidyon/mpesapayments/internal/b2c"
	b2capp "github.com/gidyon/mpesapayments/internal/b2c/v1"
	b2capp_v2 "github.com/gidyon/mpesapayments/internal/b2c/v2"
	b2c "github.com/gidyon/mpesapayments/pkg/api/b2c/v1"
	b2c_v2 "github.com/gidyon/mpesapayments/pkg/api/b2c/v2"
	"github.com/gidyon/mpesapayments/pkg/utils/httputils"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"

	"github.com/gidyon/mpesapayments/pkg/payload"
	"google.golang.org/protobuf/proto"
)

type b2cGateway struct {
	*Options
	ctxExt context.Context
}

// NewB2CGateway creates a b2c gateway for receiving b2cPayloads
func NewB2CGateway(ctx context.Context, opt *Options) (*b2cGateway, error) {
	err := validateOptions(opt)
	if err != nil {
		return nil, err
	}

	gw := &b2cGateway{
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
		http.Error(w, "receiving b2c b2cPayloads disabled", http.StatusServiceUnavailable)
		return
	}

	var (
		source = r.URL.Query().Get(b2capp.SourceKey)
	)

	if source == "DARAJA" {
		code, err := gw.fromSaf(w, r)
		if err != nil {
			gw.Logger.Errorln("Error serving incoming B2C Transaction: %s", err)
			http.Error(w, "request handler failed", code)
			return
		}
	} else {
		gw.fromOnfon(w, r)
	}
}

func (gw *b2cGateway) fromSaf(w http.ResponseWriter, r *http.Request) (int, error) {

	httputils.DumpRequest(r, "Incoming Mpesa B2C Payload")

	if r.Method != http.MethodPost {
		return http.StatusBadRequest, fmt.Errorf("bad method; only POST allowed; received %v method", r.Method)
	}

	var (
		b2cPayload = &payload.Transaction{}
		err        error
	)

	switch strings.ToLower(r.Header.Get("content-type")) {
	case "application/json", "application/json;charset=utf-8", "application/json;charset=utf8":
		err = json.NewDecoder(r.Body).Decode(b2cPayload)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("decoding json failed: %w", err)
		}
	default:
		ctype := r.Header.Get("content-type")
		return http.StatusBadRequest, fmt.Errorf("incorrect content type: %s", ctype)
	}

	switch {
	case b2cPayload == nil:
		err = fmt.Errorf("nil mpesa b2cPayload")
	case b2cPayload.Result.ConversationID == "":
		err = fmt.Errorf("missing conversation id")
	case b2cPayload.Result.OriginatorConversationID == "":
		err = fmt.Errorf("missing originator id")
	case b2cPayload.Result.ResultDesc == "":
		err = fmt.Errorf("missing description")
	}
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("request body validation error: %s", err)
	}

	tReq := &b2c.TransferFundsRequest{}

	ctx := r.Context()

	res, err := gw.RedisDB.Get(ctx, b2capp.GetMpesaRequestKey(b2cPayload.ConversationID())).Result()
	switch {
	case err == nil:
		err = proto.Unmarshal([]byte(res), tReq)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("failed to unmarshal transfer request: %v", err)
		}
	}

	pb := &b2c.B2CPayment{
		PaymentId:                "",
		InitiatorId:              tReq.InitiatorId,
		OrgShortCode:             fmt.Sprint(tReq.ShortCode),
		Msisdn:                   b2cPayload.MSISDN(),
		TransactionReference:     tReq.GetTransactionReference(),
		CustomerReference:        tReq.CustomerReference,
		CustomerNames:            tReq.CustomerNames,
		ReceiverPartyPublicName:  b2cPayload.ReceiverPartyPublicName(),
		TransactionType:          tReq.CommandId.String(),
		TransactionId:            b2cPayload.TransactionReceipt(),
		ConversationId:           b2cPayload.Result.ConversationID,
		OriginatorConversationId: b2cPayload.Result.OriginatorConversationID,
		ResultCode:               fmt.Sprint(b2cPayload.Result.ResultCode),
		ResultDescription:        b2cPayload.Result.ResultDesc,
		TransactionTimestamp:     b2cPayload.TransactionCompletedDateTime().UTC().Unix(),
		Amount:                   float32(b2cPayload.TransactionAmount()),
		WorkingAccountFunds:      float32(b2cPayload.B2CWorkingAccountAvailableFunds()),
		UtilityAccountFunds:      float32(b2cPayload.B2CUtilityAccountAvailableFunds()),
		MpesaCharges:             float32(b2cPayload.B2CChargesPaidAccountAvailableFunds()),
		OnfonCharges:             gw.B2CTransactionCharges,
		RecipientRegistered:      b2cPayload.B2CRecipientIsRegisteredCustomer(),
		Succeeded:                b2cPayload.Succeeded(),
		Processed:                false,
		CreateDate:               "",
	}

	pb, err = gw.B2CAPI.CreateB2CPayment(gw.ctxExt, &b2c.CreateB2CPaymentRequest{
		Payment: pb,
	})
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to create b2c payment: %v", err)
	}

	if tReq.Publish {
		publish := func() {
			_, err = gw.B2CAPI.PublishB2CPayment(gw.ctxExt, &b2c.PublishB2CPaymentRequest{
				PublishMessage: &b2c.PublishMessage{
					InitiatorId:    tReq.InitiatorId,
					PaymentId:      pb.PaymentId,
					MpesaReceiptId: pb.TransactionId,
					PhoneNumber:    b2cPayload.MSISDN(),
					PublishInfo:    tReq.PublishMessage,
					Payment:        pb,
				},
			})
			if err != nil {
				gw.Logger.Warningf("failed to publish message: %v", err)
			}
		}
		if tReq.GetPublishMessage().GetOnlyOnSuccess() {
			if pb.Succeeded {
				publish()
			}
		} else {
			publish()
		}
	}

	_, err = w.Write([]byte("mpesa b2c b2cPayload processed"))
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}

func (gw *b2cGateway) ServeHttpV2(w http.ResponseWriter, r *http.Request) {
	if gw.DisableB2CService {
		http.Error(w, "receiving b2c b2cPayloads disabled", http.StatusServiceUnavailable)
		return
	}

	var (
		source = r.URL.Query().Get(b2capp.SourceKey)
	)

	if source == "DARAJA" {
		code, err := gw.fromSafV2(w, r)
		if err != nil {
			gw.Logger.Errorln("Error serving incoming (v2) B2C Transaction: %s", err)
			http.Error(w, "request handler failed", code)
			return
		}
	} else {
		gw.fromOnfon(w, r)
	}
}

func (gw *b2cGateway) fromSafV2(w http.ResponseWriter, r *http.Request) (int, error) {

	httputils.DumpRequest(r, "Incoming Mpesa B2C Payload v2")

	if r.Method != http.MethodPost {
		return http.StatusBadRequest, fmt.Errorf("bad method; only POST allowed; received %v method", r.Method)
	}

	var (
		b2cPayload = &payload.Transaction{}
		tReq       = &b2c_v2.TransferFundsRequest{}
		db         = &b2c_model.Payment{}
		succeeded  = "YES"
		status     = b2c_v2.B2CStatus_B2C_SUCCESS.String()
		err        error
	)

	// Marshal incoming payload
	{
		switch strings.ToLower(r.Header.Get("content-type")) {
		case "application/json", "application/json;charset=utf-8", "application/json;charset=utf8":
			err = json.NewDecoder(r.Body).Decode(b2cPayload)
			if err != nil {
				return http.StatusBadRequest, fmt.Errorf("decoding json failed: %w", err)
			}
		default:
			ctype := r.Header.Get("content-type")
			return http.StatusBadRequest, fmt.Errorf("incorrect content type: %s", ctype)
		}
	}

	// Validate incoming payload
	{
		switch {
		case b2cPayload == nil:
			err = fmt.Errorf("nil mpesa b2cPayload")
		case b2cPayload.Result.ConversationID == "":
			err = fmt.Errorf("missing conversation id")
		case b2cPayload.Result.OriginatorConversationID == "":
			err = fmt.Errorf("missing originator id")
		case b2cPayload.Result.ResultDesc == "":
			err = fmt.Errorf("missing description")
		}
		if err != nil {
			return http.StatusBadRequest, err
		}
	}

	if b2cPayload.Result.ResultCode != 0 {
		succeeded = "NO"
		status = b2c_v2.B2CStatus_B2C_FAILED.String()
	}

	// Get tranfer funds request
	{
		ctx := r.Context()
		res, err := gw.RedisDB.Get(ctx, b2capp.GetMpesaRequestKey(b2cPayload.ConversationID())).Result()
		switch {
		case err == nil:
			err = proto.Unmarshal([]byte(res), tReq)
			if err != nil {
				gw.Logger.Errorln("Failed to unmarshal transfer funds request: ", err)
			}
		}
	}

	err = gw.SQLDB.First(db, "conversation_id = ?", b2cPayload.ConversationID()).Error
	switch {
	case err == nil:
		// Update STK b2cPayload
		{
			err = gw.SQLDB.Model(db).
				Updates(map[string]interface{}{
					"result_code":           fmt.Sprint(b2cPayload.Result.ResultCode),
					"result_description":    b2cPayload.Result.ResultDesc,
					"working_account_funds": float32(b2cPayload.B2CWorkingAccountAvailableFunds()),
					"utility_account_funds": float32(b2cPayload.B2CUtilityAccountAvailableFunds()),
					"mpesa_charges":         float32(b2cPayload.B2CChargesPaidAccountAvailableFunds()),
					"recipient_registered":  b2cPayload.B2CRecipientIsRegisteredCustomer(),
					"mpesa_receipt_id":      b2cPayload.TransactionReceipt(),
					"transaction_time":      sql.NullTime{Valid: true, Time: b2cPayload.TransactionCompletedDateTime().UTC()},
					"receiver_public_name":  b2cPayload.ReceiverPartyPublicName(),
					"status":                status,
					"succeeded":             succeeded,
				}).Error
			if err != nil {
				return http.StatusInternalServerError, fmt.Errorf("failed to update b2c: %v", err)
			}
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		// Create B2C
		{
			db = &b2c_model.Payment{
				ID:                            0,
				InitiatorID:                   tReq.GetInitiatorId(),
				InitiatorTransactionReference: tReq.GetInitiatorTransactionReference(),
				InitiatorCustomerReference:    tReq.GetInitiatorCustomerReference(),
				InitiatorCustomerNames:        tReq.GetInitiatorCustomerNames(),
				Msisdn:                        b2cPayload.MSISDN(),
				OrgShortCode:                  tReq.ShortCode,
				CommandId:                     tReq.CommandId.String(),
				Amount:                        float32(b2cPayload.TransactionAmount()),
				ConversationID:                b2cPayload.ConversationID(),
				OriginatorConversationID:      b2cPayload.OriginatorConversationID(),
				ResponseDescription:           "",
				ResponseCode:                  "",
				ResultCode:                    fmt.Sprint(b2cPayload.Result.ResultCode),
				ResultDescription:             b2cPayload.Result.ResultDesc,
				WorkingAccountFunds:           float32(b2cPayload.B2CWorkingAccountAvailableFunds()),
				UtilityAccountFunds:           float32(b2cPayload.B2CUtilityAccountAvailableFunds()),
				MpesaCharges:                  float32(b2cPayload.B2CChargesPaidAccountAvailableFunds()),
				OnfonCharges:                  gw.B2CTransactionCharges,
				RecipientRegistered:           b2cPayload.B2CRecipientIsRegisteredCustomer(),
				MpesaReceiptId: sql.NullString{
					Valid:  b2cPayload.TransactionReceipt() != "",
					String: b2cPayload.TransactionReceipt(),
				},
				ReceiverPublicName: b2cPayload.ReceiverPartyPublicName(),
				Status:             status,
				Source:             "",
				Tag:                "",
				Succeeded:          succeeded,
				Processed:          "NO",
				TransactionTime:    sql.NullTime{Valid: true, Time: b2cPayload.TransactionCompletedDateTime().UTC()},
				CreatedAt:          time.Time{},
			}
			err = gw.SQLDB.Create(db).Error
			if err != nil {
				return http.StatusInternalServerError, fmt.Errorf("failed to create b2c b2cPayload: %v", err)
			}
		}
	default:
		gw.Logger.Errorln(err)
		return http.StatusInternalServerError, errors.New("failed to create b2c b2cPayload")
	}

	pb, err := b2capp_v2.B2CPaymentPB(db)
	if err != nil {
		gw.Logger.Errorln(err)
		return http.StatusInternalServerError, errors.New("failed to get b2c proto")
	}

	if tReq.Publish {
		publish := func() {
			_, err = gw.B2CV2API.PublishB2CPayment(gw.ctxExt, &b2c_v2.PublishB2CPaymentRequest{
				PublishMessage: &b2c_v2.PublishMessage{
					InitiatorId:    tReq.InitiatorId,
					TransactionId:  pb.TransactionId,
					MpesaReceiptId: pb.MpesaReceiptId,
					Msisdn:         b2cPayload.MSISDN(),
					PublishInfo:    tReq.PublishMessage,
					Payment:        pb,
				},
			})
			if err != nil {
				gw.Logger.Warningf("failed to publish message: %v", err)
			} else {
				gw.Logger.Infoln("B2C has been published on channel ", tReq.GetPublishMessage().GetChannelName())
			}
		}
		if tReq.GetPublishMessage().GetOnlyOnSuccess() {
			if pb.Succeeded {
				publish()
			}
		} else {
			publish()
		}
	}

	_, err = w.Write([]byte("mpesa b2c b2cPayload processed"))
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}

func (gw *b2cGateway) fromOnfon(w http.ResponseWriter, r *http.Request) {

	// gw.Logger.Infoln("received b2c b2cPayload from onfon")

	// httputils.DumpRequest(r, "Safaricom B2C Payload")

	// // Must be POST request
	// if r.Method != http.MethodPost {
	// 	gw.Logger.Infof("only POST allowed; received %v http method", r.Method)
	// 	http.Error(w, fmt.Sprintf("bad method; only POST allowed; received %v method", r.Method), http.StatusBadRequest)
	// 	return
	// }

	// var (
	// 	b2cPayload = &payload.IncomingTransactionOnfon{}
	// 	ctx         = r.Context()
	// 	err         error
	// )

	// switch r.Header.Get("content-type") {
	// case "application/json", "application/json;charset=UTF-8":
	// 	// Marshaling
	// 	err = json.NewDecoder(r.Body).Decode(b2cPayload)
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
	// case b2cPayload == nil:
	// 	err = fmt.Errorf("nil mpesa b2cPayload")
	// case b2cPayload.ConversationID == "":
	// 	err = fmt.Errorf("missing conversation id")
	// case b2cPayload.OriginatorConversationID == "":
	// 	err = fmt.Errorf("missing originator id")
	// case b2cPayload.ResultDesc == "":
	// 	err = fmt.Errorf("missing description")
	// }
	// if err != nil {
	// 	gw.Logger.Errorf("validation error: %v", err)
	// 	http.Error(w, err.Error(), http.StatusBadRequest)
	// 	return
	// }

	// initiator := &b2c.InitiatorPayload{}

	// // Get initiator
	// key := b2capp.GetInitiatorKey(b2cPayload.MSISDN())

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
	// 	Msisdn:                   b2cPayload.MSISDN(),
	// 	ReceiverPartyPublicName:  b2cPayload.ReceiverPartyPublicName,
	// 	TransactionType:          initiator.TransactionType,
	// 	TransactionId:            b2cPayload.TransactionID,
	// 	ConversationId:           b2cPayload.ConversationID,
	// 	OriginatorConversationId: b2cPayload.OriginatorConversationID,
	// 	ResultCode:               fmt.Sprint(b2cPayload.ResultCode),
	// 	ResultDescription:        b2cPayload.ResultDesc,
	// 	TransactionTimestamp:     b2cPayload.CompletedDateTime().Unix(),
	// 	Amount:                   float32(b2cPayload.Amount()),
	// 	WorkingAccountFunds:      float32(b2cPayload.B2CUtilityAccountAvailableFundsV2()),
	// 	RecipientRegistered:      b2cPayload.B2CRecipientIsRegisteredCustomerV2(),
	// 	OnfonCharges:             gw.B2CTransactionCharges,
	// 	Succeeded:                b2cPayload.Succeeded(),
	// }

	// if initiator.PublishLocal && initiator.RequestId != "" {
	// 	// Marshal
	// 	bs, err := proto.Marshal(pb)
	// 	if err != nil {
	// 		http.Error(w, "marshaling b2c payload not succeesful", http.StatusInternalServerError)
	// 		gw.Logger.Errorln("marshaling b2c payload not succeesful: %s", err)
	// 		return
	// 	}

	// 	// Set b2cPayload in cache
	// 	err = gw.RedisDB.Set(ctx, initiator.RequestId, bs, 5*time.Minute).Err()
	// 	if err != nil {
	// 		http.Error(w, "failed to set b2c b2cPayload in cache", http.StatusInternalServerError)
	// 		gw.Logger.Errorln("failed to set b2c b2cPayload in cache: %s", err)
	// 		return
	// 	}

	// 	// Publish the b2cPayload for local goroutines
	// 	err = gw.RedisDB.Publish(ctx, gw.B2CLocalTopic, initiator.RequestId).Err()
	// 	if err != nil {
	// 		http.Error(w, "failed to publish b2c to local listeners", http.StatusInternalServerError)
	// 		gw.Logger.Errorln("failed to publish b2c b2cPayload to local listeners: %s", err)
	// 		return
	// 	}
	// }

	// if !initiator.DropTransaction {
	// 	// We save the b2cPayload
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

	// _, err = w.Write([]byte("mpesa b2c b2cPayload processed"))
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// }
}
