package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gidyon/micro/v2/pkg/middleware/grpc/auth"
	b2capp "github.com/gidyon/mpesapayments/internal/b2c"
	"github.com/gidyon/mpesapayments/pkg/api/b2c"
	"github.com/gidyon/mpesapayments/pkg/payload"
	"google.golang.org/grpc/metadata"
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

func addPrefix(key, prefix string) string {
	return fmt.Sprintf("%s:%s", prefix, key)
}

func (gw *b2cGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if gw.DisableSTKService {
		http.Error(w, "receiving stk transactions disabled", http.StatusServiceUnavailable)
		return
	}

	var (
		requestID       = r.URL.Query().Get("request_id")
		shortCode       = r.URL.Query().Get("short_code")
		msisdn          = r.URL.Query().Get("msisdn")
		txType          = r.URL.Query().Get("tx_type")
		publishLocal    = r.URL.Query().Get("publish_local") != ""
		publishOnCreate = r.URL.Query().Get("publish_global") != ""
		drop            = r.URL.Query().Get("drop") != ""
		ctx             = r.Context()
		err             error
	)

	gw.Logger.Infoln("received b2c transaction from mpesa")

	// Must be POST request
	if r.Method != http.MethodPost {
		gw.Logger.Infoln("only post allowed")
		http.Error(w, "bad method; only POST allowed", http.StatusBadRequest)
		return
	}

	transaction := &payload.Transaction{}

	switch r.Header.Get("content-type") {
	case "application/json", "application/json;charset=UTF-8":
		// Marshaling
		err = json.NewDecoder(r.Body).Decode(transaction)
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
		gw.Logger.Errorf("validation error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Transaction payload
	transactionPB := &b2c.B2CPayment{
		OrgShortCode:             shortCode,
		Msisdn:                   msisdn,
		ReceiverPartyPublicName:  transaction.ReceiverPartyPublicName(),
		TransactionType:          txType,
		ConversationId:           transaction.Result.ConversationID,
		OriginatorConversationId: transaction.Result.OriginatorConversationID,
		ResultCode:               fmt.Sprint(transaction.Result.ResultCode),
		ResultDescription:        transaction.Result.ResultDesc,
		TransactionTimestamp:     transaction.TransactionCompletedDateTime().Unix(),
		Amount:                   float32(transaction.TransactionAmount()),
		WorkingAccountFunds:      float32(transaction.B2CWorkingAccountAvailableFunds()),
		UtilityAccountFunds:      float32(transaction.B2CUtilityAccountAvailableFunds()),
		ChargesPaidFunds:         float32(transaction.B2CChargesPaidAccountAvailableFunds()),
		RecipientRegistered:      transaction.B2CRecipientIsRegisteredCustomer(),
		Succeeded:                transaction.Succeeded(),
	}

	if publishLocal && requestID != "" {
		// Marshal
		bs, err := proto.Marshal(transactionPB)
		if err != nil {
			http.Error(w, "marshaling b2c payload not succeesful", http.StatusInternalServerError)
			gw.Logger.Errorln("marshaling b2c payload not succeesful: %s", err)
			return
		}

		// Set transaction in cache
		err = gw.RedisDB.Set(ctx, addPrefix(requestID, gw.RedisKeyPrefix), bs, 5*time.Minute).Err()
		if err != nil {
			http.Error(w, "failed to set b2c transaction in cache", http.StatusInternalServerError)
			gw.Logger.Errorln("failed to set b2c transaction in cache: %s", err)
			return
		}

		// Publish the transaction for local goroutines
		err = gw.RedisDB.Publish(ctx, gw.B2CLocalTopic, requestID).Err()
		if err != nil {
			http.Error(w, "failed to publish b2c to local listener", http.StatusInternalServerError)
			gw.Logger.Errorln("failed to publish b2c transaction to local listener: %s", err)
			return
		}
	}

	if !drop {
		// We save the transaction
		_, err = gw.b2cAPI.CreateB2CPayment(gw.ctxExt, &b2c.CreateB2CPaymentRequest{
			Payment: transactionPB,
			Publish: publishOnCreate,
		})
		gw.Logger.Errorf("failed to create b2c payment: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)

		// Save in cache for later processing
		bs, err := proto.Marshal(transactionPB)
		if err == nil {
			if !gw.DisablePublishing {
				gw.RedisDB.LPush(r.Context(), b2capp.FailedTxList, bs)
			}
		}

		return
	}

	w.Write([]byte("mpesa b2c transaction processed"))
}
