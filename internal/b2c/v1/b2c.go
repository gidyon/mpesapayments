package b2c

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gidyon/micro/v2/pkg/middleware/grpc/auth"
	"github.com/gidyon/micro/v2/utils/errs"
	b2c "github.com/gidyon/mpesapayments/pkg/api/b2c/v1"
	c2b "github.com/gidyon/mpesapayments/pkg/api/c2b/v1"
	"github.com/gidyon/mpesapayments/pkg/payload"
	"github.com/gidyon/mpesapayments/pkg/utils/httputils"
	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
)

const SourceKey = "source"

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

type b2cAPIServer struct {
	b2c.UnimplementedB2CAPIServer
	ctxAdmin context.Context
	*Options
}

// Options contains options for starting b2c service
type Options struct {
	PublishChannel     string
	QueryBalanceURL    string
	B2CURL             string
	ReversalURL        string
	SQLDB              *gorm.DB
	RedisDB            *redis.Client
	Logger             grpclog.LoggerV2
	AuthAPI            auth.API
	HTTPClient         httpClient
	OptionsB2C         *OptionsB2C
	TransactionCharges float32
}

// ValidateOptions validates options required by stk service
func ValidateOptions(opt *Options) error {
	var err error
	switch {
	case opt == nil:
		err = errs.NilObject("options")
	case opt.SQLDB == nil:
		err = errs.NilObject("sql db")
	case opt.RedisDB == nil:
		err = errs.NilObject("redis db")
	case opt.Logger == nil:
		err = errs.NilObject("logger")
	case opt.AuthAPI == nil:
		err = errs.NilObject("auth API")
	case opt.HTTPClient == nil:
		err = errs.NilObject("http client")
	case opt.OptionsB2C == nil:
		err = errs.NilObject("b2c options")
	case opt.PublishChannel == "":
		err = errs.MissingField("publish channel")
	case opt.QueryBalanceURL == "":
		err = errs.MissingField("query balance url")
	case opt.B2CURL == "":
		err = errs.MissingField("b2c url")
	case opt.ReversalURL == "":
		err = errs.MissingField("reversal url")
	}

	return err
}

// OptionsB2C contains options for doing b2c with mpesa
type OptionsB2C struct {
	ConsumerKey                string
	ConsumerSecret             string
	AccessTokenURL             string
	QueueTimeOutURL            string
	ResultURL                  string
	InitiatorUsername          string
	InitiatorPassword          string
	InitiatorEncryptedPassword string
	PublicKeyCertificateFile   string
	accessToken                string
	basicToken                 string
}

// ValidateOptionsB2C validates b2c options
func ValidateOptionsB2C(opt *OptionsB2C) error {
	var err error
	switch {
	case opt == nil:
		err = errs.NilObject("b2c options")
	case opt.AccessTokenURL == "":
		err = errs.MissingField("access token url")
	case opt.ConsumerKey == "":
		err = errs.MissingField("consumer key")
	case opt.ConsumerSecret == "":
		err = errs.MissingField("consumer secret")
	case opt.InitiatorUsername == "":
		err = errs.MissingField("initiator username")
	case opt.InitiatorPassword == "" && opt.InitiatorEncryptedPassword == "":
		err = errs.MissingField("initiator password")
	case opt.QueueTimeOutURL == "":
		err = errs.MissingField("queue timeout url")
	case opt.ResultURL == "":
		err = errs.MissingField("result url")
	}
	return err
}

// NewB2CAPI creates a B2C API for mpesa
func NewB2CAPI(ctx context.Context, opt *Options) (b2c.B2CAPIServer, error) {
	// Validation
	var err error
	switch {
	case ctx == nil:
		return nil, errs.NilObject("context")
	default:
		err = ValidateOptions(opt)
		if err != nil {
			return nil, err
		}
		err = ValidateOptionsB2C(opt.OptionsB2C)
		if err != nil {
			return nil, err
		}
	}

	// Update Basic Token
	opt.OptionsB2C.basicToken = base64.StdEncoding.EncodeToString([]byte(
		opt.OptionsB2C.ConsumerKey + ":" + opt.OptionsB2C.ConsumerSecret,
	))

	// Generate jwt for API
	token, err := opt.AuthAPI.GenToken(
		ctx, &auth.Payload{Group: auth.DefaultAdminGroup()}, time.Now().Add(10*365*24*time.Hour))
	if err != nil {
		return nil, fmt.Errorf("failed to generate auth token: %v", err)
	}

	md := metadata.Pairs(auth.Header(), fmt.Sprintf("%s %s", auth.Scheme(), token))

	ctxAdmin := metadata.NewIncomingContext(ctx, md)

	// Authorize the jwt
	ctxAdmin, err = opt.AuthAPI.AuthorizeFunc(ctxAdmin)
	if err != nil {
		return nil, err
	}

	b2cAPI := &b2cAPIServer{
		Options:  opt,
		ctxAdmin: ctxAdmin,
	}

	b2cAPI.Logger.Infof("Publishing to b2c consumers on channel: %v", b2cAPI.PublishChannel)

	b2cTable = os.Getenv("B2C_TRANSACTIONS_TABLE")

	btcStatsTable = os.Getenv("B2C_STATS_TABLE")

	// Auto migration
	// if !b2cAPI.SQLDB.Migrator().HasTable(&Payment{}) {
	err = b2cAPI.SQLDB.Migrator().AutoMigrate(&Payment{})
	if err != nil {
		return nil, err
	}
	// }

	if !b2cAPI.SQLDB.Migrator().HasTable(&DailyStat{}) {
		err = b2cAPI.SQLDB.Migrator().AutoMigrate(&DailyStat{})
		if err != nil {
			return nil, err
		}
	}

	// Worker for updating access token
	go b2cAPI.updateAccessTokenWorker(ctx, 30*time.Minute)

	// Worker to generate daily statistics
	go b2cAPI.dailyDailyStatWorker(ctx)

	return b2cAPI, nil
}

func (b2cAPI *b2cAPIServer) CreateB2CPayment(
	ctx context.Context, createReq *b2c.CreateB2CPaymentRequest,
) (*b2c.B2CPayment, error) {
	// Authorization
	_, err := b2cAPI.AuthAPI.AuthorizeAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case createReq == nil:
		return nil, errs.NilObject("create request")
	default:
		err = ValidatePayment(createReq.Payment)
		if err != nil {
			return nil, err
		}
	}

	// Get model
	db, err := B2CPaymentDB(createReq.Payment)
	if err != nil {
		return nil, err
	}

	// Save payment
	err = b2cAPI.SQLDB.Create(db).Error
	if err != nil {
		b2cAPI.Logger.Errorln(err)
		return nil, errs.WrapMessage(codes.Internal, "failed to create")
	}

	return B2CPaymentPB(db)
}

func (b2cAPI *b2cAPIServer) GetB2CPayment(
	ctx context.Context, req *b2c.GetB2CPaymentRequest,
) (*b2c.B2CPayment, error) {
	// Validation
	var (
		key int
		err error
	)
	switch {
	case req == nil:
		return nil, errs.NilObject("get request")
	case req.PaymentId == "":
		return nil, errs.MissingField("payment id")
	default:
		key, _ = strconv.Atoi(req.PaymentId)
	}

	db := &Payment{}

	if key != 0 {
		err = b2cAPI.SQLDB.First(db, "id=?", key).Error
	} else {
		err = b2cAPI.SQLDB.First(db, "transaction_id=?", req.PaymentId).Error
	}
	switch {
	case err == nil:
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, errs.DoesNotExist("b2c transaction", req.PaymentId)
	default:
		b2cAPI.Logger.Errorln(err)
		return nil, errs.WrapMessage(codes.Internal, "failed to get b2c payment")
	}

	return B2CPaymentPB(db)
}

func (b2cAPI *b2cAPIServer) QueryTransactionStatus(
	ctx context.Context, req *b2c.QueryTransactionStatusRequest,
) (*b2c.QueryResponse, error) {
	return &b2c.QueryResponse{}, nil
}

// GetMpesaRequestKey is key that initiates data
func GetMpesaRequestKey(requestId string) string {
	return fmt.Sprintf("b2c:%s", requestId)
}

// GetMpesaSTKPushKey retrives key storing initiator key
func GetMpesaB2CPushKey(msisdn string) string {
	return fmt.Sprintf("b2crequests:%s", msisdn)
}

func (b2cAPI *b2cAPIServer) TransferFunds(
	ctx context.Context, req *b2c.TransferFundsRequest,
) (*b2c.TransferFundsResponse, error) {
	// Authorize request
	_, err := b2cAPI.AuthAPI.AuthorizeAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Validate request
	switch {
	case req == nil:
		return nil, errs.NilObject("transfer request")
	case req.Amount == 0:
		return nil, errs.MissingField("amount")
	case req.CommandId == b2c.TransferFundsRequest_COMMANDID_UNSPECIFIED:
		return nil, errs.MissingField("command id")
	case req.Msisdn == 0:
		return nil, errs.MissingField("msisdn")
	case req.ShortCode == 0:
		return nil, errs.MissingField("short code")
	case req.Remarks == "":
		return nil, errs.MissingField("remarks")
	}

	cacheKey := GetMpesaB2CPushKey(fmt.Sprint(req.Msisdn))

	// Check whether another transaction is under way
	exists, err := b2cAPI.RedisDB.Exists(ctx, cacheKey).Result()
	if err != nil {
		return nil, errs.RedisCmdFailed(err, "exists")
	}

	if exists == 1 {
		return &b2c.TransferFundsResponse{
			Progress: false,
			Message:  "Another B2C Transaction is currently underway. Please wait.",
		}, nil
	}

	// Save key in cache for 100 seconds
	err = b2cAPI.RedisDB.Set(ctx, cacheKey, "active", 100*time.Second).Err()
	if err != nil {
		b2cAPI.RedisDB.Del(ctx, cacheKey)
		return nil, errs.RedisCmdFailed(err, "set")
	}

	var commandID string
	switch req.CommandId {
	case b2c.TransferFundsRequest_BUSINESS_PAYMENT:
		commandID = "BusinessPayment"
	case b2c.TransferFundsRequest_PROMOTION_PAYMENT:
		commandID = "PromotionPayment"
	case b2c.TransferFundsRequest_SALARY_PAYMENT:
		commandID = "SalaryPayment"
	}

	// Transfer funds payload
	queryBalPayload := &payload.B2CRequest{
		InitiatorName:      b2cAPI.Options.OptionsB2C.InitiatorUsername,
		SecurityCredential: b2cAPI.OptionsB2C.InitiatorEncryptedPassword,
		CommandID:          commandID,
		Amount:             fmt.Sprint(req.Amount),
		PartyA:             fmt.Sprint(req.ShortCode),
		PartyB:             req.Msisdn,
		Remarks:            req.Remarks,
		QueueTimeOutURL:    fmt.Sprintf("%s?%s=DARAJA", b2cAPI.OptionsB2C.QueueTimeOutURL, SourceKey),
		ResultURL:          fmt.Sprintf("%s?%s=DARAJA", b2cAPI.OptionsB2C.ResultURL, SourceKey),
		Occassion:          req.Occassion,
	}

	if req.PublishMessage == nil {
		req.PublishMessage = &b2c.PublishInfo{
			Payload: map[string]string{},
		}
	}
	if req.PublishMessage.Payload == nil {
		req.PublishMessage.Payload = map[string]string{}
	}

	bs, err := json.Marshal(queryBalPayload)
	if err != nil {
		return nil, errs.FromJSONMarshal(err, "b2cPayload")
	}

	hReq, err := http.NewRequest(http.MethodPost, b2cAPI.B2CURL, bytes.NewReader(bs))
	if err != nil {
		return nil, errs.WrapMessage(codes.Internal, "failed to create transfer funds request")
	}

	hReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", b2cAPI.OptionsB2C.accessToken))
	hReq.Header.Set("Content-Type", "application/json")

	httputils.DumpRequest(hReq, "TransferFunds Request")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		res, err := b2cAPI.HTTPClient.Do(hReq)
		if err != nil {
			b2cAPI.Logger.Errorf("failed to post transfer request to mpesa API: %v", err)
			return
		}

		httputils.DumpResponse(res, "TransferFunds Response")

		apiRes := &payload.GenericAPIResponse{}

		err = json.NewDecoder(res.Body).Decode(&apiRes.Response)
		if err != nil && err != io.EOF {
			b2cAPI.Logger.Errorf("failed to decode mpesa response: %v", err)
			return
		}

		errMsg, ok := apiRes.Response["errorMessage"]
		if ok {
			b2cAPI.Logger.Errorf("error happened while sending transfer funds request: %v", errMsg)
			return
		}

		switch strings.ToLower(res.Header.Get("content-type")) {
		case "application/json", "application/json;charset=utf-8":
			// Marshal req
			bs, err := proto.Marshal(req)
			if err != nil {
				b2cAPI.Logger.Errorln(err)
				return
			}

			requestId := GetMpesaRequestKey(fmt.Sprint(apiRes.Response["ConversationID"]))

			err = b2cAPI.RedisDB.Set(ctx, requestId, bs, time.Minute*30).Err()
			if err != nil {
				b2cAPI.Logger.Errorln(err)
				return
			}

			b2cAPI.RedisDB.Del(ctx, cacheKey)
		default:
		}
	}()

	return &b2c.TransferFundsResponse{
		Progress: true,
		Message:  "Processing. Disbursement will come",
	}, nil
}

func (b2cAPI *b2cAPIServer) QueryAccountBalance(
	ctx context.Context, req *b2c.QueryAccountBalanceRequest,
) (*b2c.QueryAccountBalanceResponse, error) {
	// Authorize request
	_, err := b2cAPI.AuthAPI.AuthorizeGroup(ctx, b2cAPI.AuthAPI.AdminGroups()...)
	if err != nil {
		return nil, err
	}

	// Validate request
	switch {
	case req == nil:
		return nil, errs.NilObject("query request")
	case req.PartyA == 0:
		return nil, errs.MissingField("party")
	case req.InitiatorId == "":
		return nil, errs.MissingField("initiator id")
	case req.Remarks == "":
		return nil, errs.MissingField("remarks")
	case req.IdentifierType == b2c.QueryAccountBalanceRequest_QUERY_ACCOUNT_UNSPECIFIED:
		return nil, errs.MissingField("identifier type")
	}

	// Send the request to safaricom
	queryBalPayload := &payload.AccountBalanceRequest{
		CommandID:          "AccountBalance",
		PartyA:             int32(req.PartyA),
		IdentifierType:     int32(req.IdentifierType),
		Remarks:            req.Remarks,
		Initiator:          b2cAPI.OptionsB2C.InitiatorUsername,
		SecurityCredential: b2cAPI.OptionsB2C.InitiatorEncryptedPassword,
		QueueTimeOutURL:    b2cAPI.OptionsB2C.QueueTimeOutURL,
		ResultURL:          b2cAPI.OptionsB2C.ResultURL,
	}

	// Json Marshal
	bs, err := json.Marshal(queryBalPayload)
	if err != nil {
		return nil, errs.FromJSONMarshal(err, "b2cPayload")
	}

	// Create request
	hReq, err := http.NewRequest(http.MethodPost, b2cAPI.QueryBalanceURL, bytes.NewReader(bs))
	if err != nil {
		return nil, errs.WrapMessage(codes.Internal, "failed to create request to query balance")
	}

	// Update headers
	hReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", b2cAPI.OptionsB2C.accessToken))
	hReq.Header.Set("Content-Type", "application/json")

	httputils.DumpRequest(hReq, "QueryAccountBalance Request")

	// Post to MPESA API
	res, err := b2cAPI.HTTPClient.Do(hReq)
	if err != nil {
		return nil, errs.WrapError(err)
	}

	httputils.DumpResponse(res, "QueryAccountBalance Response")

	apiRes := &payload.GenericAPIResponse{}

	err = json.NewDecoder(res.Body).Decode(&apiRes.Response)
	if err != nil && err != io.EOF {
		return nil, errs.WrapError(err)
	}

	if !apiRes.Succeeded() {
		return nil, errs.WrapMessage(codes.Unknown, apiRes.Error())
	}

	return &b2c.QueryAccountBalanceResponse{
		Party: req.PartyA,
		// WorkingAccountFunds: transaction.WorkingAccountFunds,
		// UtilityAccountFunds: transaction.UtilityAccountFunds,
		// ChargesPaidFunds:    transaction.MpesaCharges,
		Completed: req.Synchronous,
	}, nil
}

func (b2cAPI *b2cAPIServer) ReverseTransaction(
	ctx context.Context, reverseReq *b2c.ReverseTransactionRequest,
) (*emptypb.Empty, error) {
	// Authorize request
	_, err := b2cAPI.AuthAPI.AuthorizeAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Validate request
	switch {
	case reverseReq == nil:
		return nil, errs.NilObject("reverse request")
	case reverseReq.ReceiverType == 0:
		return nil, errs.MissingField("receiver type")
	case reverseReq.ShortCode == 0:
		return nil, errs.MissingField("short code")
	case reverseReq.TransactionId == "":
		return nil, errs.MissingField("transaction id")
	case reverseReq.Remarks == "":
		return nil, errs.MissingField("remarks")
	}

	// Send the request to mpesa API
	reverseRequest := &payload.ReversalRequest{
		CommandID:              "TransactionReversal",
		ReceiverParty:          reverseReq.ShortCode,
		ReceiverIdentifierType: reverseReq.ReceiverType,
		Remarks:                reverseReq.Remarks,
		Initiator:              b2cAPI.Options.OptionsB2C.InitiatorUsername,
		SecurityCredential:     b2cAPI.OptionsB2C.InitiatorEncryptedPassword,
		TransactionID:          reverseReq.TransactionId,
		Occassion:              reverseReq.Occassion,
	}

	// Json Marshal
	bs, err := json.Marshal(reverseRequest)
	if err != nil {
		return nil, errs.FromJSONMarshal(err, "b2cPayload")
	}

	// Create request
	req, err := http.NewRequest(http.MethodPost, b2cAPI.ReversalURL, bytes.NewReader(bs))
	if err != nil {
		return nil, errs.WrapMessage(codes.Internal, "failed to create new request")
	}

	// Update headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", b2cAPI.OptionsB2C.accessToken))
	req.Header.Set("Content-Type", "application/json")

	httputils.DumpRequest(req, "ReverseTransaction Request")

	// Post to MPESA API
	res, err := b2cAPI.HTTPClient.Do(req)
	if err != nil {
		return nil, errs.WrapError(err)
	}

	httputils.DumpResponse(res, "ReverseTransaction Response")

	apiRes := &payload.GenericAPIResponse{}

	err = json.NewDecoder(res.Body).Decode(&apiRes.Response)
	if err != nil && err != io.EOF {
		return nil, errs.WrapError(err)
	}

	if !apiRes.Succeeded() {
		return nil, errs.WrapMessage(codes.Unknown, apiRes.Error())
	}

	return &emptypb.Empty{}, nil
}

// ValidatePayment validates b2c payment
func ValidatePayment(paymentPB *b2c.B2CPayment) error {
	var err error
	switch {
	case paymentPB == nil:
		err = errs.NilObject("payment pb")
	case paymentPB.ResultDescription == "":
		err = errs.MissingField("result description")
		// case paymentPB.ReceiverPartyPublicName == "":
		// 	err = errs.MissingField("receiver public name")
		// case paymentPB.TransactionTimestamp == 0:
		// 	err = errs.MissingField("transaction timestamp")
	}
	return err
}

const defaultPageSize = 20

func getTime(dateStr string) (time.Time, error) {
	// 2020y 08m 16d 20h 41m 16s
	// "2006-01-02T15:04:05Z07:00"

	timeRFC3339Str := fmt.Sprintf("%sT00:00:00Z", dateStr)

	t, err := time.Parse(time.RFC3339, timeRFC3339Str)
	if err != nil {
		return time.Time{}, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "failed to parse date to time")
	}

	return t, nil
}

func (b2cAPI *b2cAPIServer) ListB2CPayments(
	ctx context.Context, req *b2c.ListB2CPaymentsRequest,
) (*b2c.ListB2CPaymentsResponse, error) {
	// Authorization
	payload, err := b2cAPI.AuthAPI.GetJwtPayload(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case req == nil:
		return nil, errs.NilObject("list request")
	case req.PageSize < 0:
		return nil, errs.IncorrectVal("page size")
	}

	pageSize := req.GetPageSize()
	if pageSize > defaultPageSize {
		if !b2cAPI.AuthAPI.IsAdmin(payload.Group) {
			pageSize = defaultPageSize
		}
	} else if pageSize == 0 {
		pageSize = defaultPageSize
	}

	var key string

	pageToken := req.GetPageToken()
	if pageToken != "" {
		bs, err := base64.StdEncoding.DecodeString(req.GetPageToken())
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "failed to parse page token")
		}
		key = string(bs)
	}

	db := b2cAPI.SQLDB.Model(&Payment{}).Limit(int(pageSize + 1))
	if key != "" {
		switch req.GetFilter().GetOrderField() {
		case b2c.OrderField_PAYMENT_ID:
			db = db.Where("id<?", key).Order("id DESC")
		case b2c.OrderField_TRANSACTION_TIMESTAMP:
			db = db.Where("transaction_time<?", key).Order("transaction_time DESC")
		}
	} else {
		switch req.GetFilter().GetOrderField() {
		case b2c.OrderField_PAYMENT_ID:
			db = db.Order("id DESC")
		case b2c.OrderField_TRANSACTION_TIMESTAMP:
			db = db.Order("transaction_time DESC")
		}
	}

	// Apply filters
	if req.Filter != nil {
		startTimestamp := req.Filter.GetStartTimestamp()
		endTimestamp := req.Filter.GetEndTimestamp()

		if endTimestamp > startTimestamp {
			switch req.GetFilter().GetOrderField() {
			case b2c.OrderField_PAYMENT_ID:
				db = db.Where("created_at BETWEEN ? AND ?", time.Unix(startTimestamp, 0), time.Unix(endTimestamp, 0))
			case b2c.OrderField_TRANSACTION_TIMESTAMP:
				db = db.Where("transaction_time BETWEEN ? AND ?", time.Unix(startTimestamp, 0), time.Unix(endTimestamp, 0))
			}
		} else if req.Filter.TxDate != "" {
			t, err := getTime(req.Filter.TxDate)
			if err != nil {
				return nil, err
			}
			switch req.GetFilter().GetOrderField() {
			case b2c.OrderField_PAYMENT_ID:
				db = db.Where("created_at BETWEEN ? AND ?", t, t.Add(time.Hour*24))
			case b2c.OrderField_TRANSACTION_TIMESTAMP:
				db = db.Where("transaction_time BETWEEN ? AND ?", t, t.Add(time.Hour*24))
			}
		}

		if req.Filter.InitiatorId != "" {
			if req.Filter.UseLikeInitiator {
				db = db.Where("initiator_id LIKE ?", "%"+req.Filter.InitiatorId+"%")
			} else {
				db = db.Where("initiator_id = ?", req.Filter.InitiatorId)
			}
		}

		if len(req.Filter.Msisdns) > 0 {
			db = db.Where("msisdn IN(?)", req.Filter.Msisdns)
		}

		if req.Filter.ProcessState != c2b.ProcessedState_PROCESS_STATE_UNSPECIFIED {
			switch req.Filter.ProcessState {
			case c2b.ProcessedState_PROCESS_STATE_UNSPECIFIED:
			case c2b.ProcessedState_NOT_PROCESSED:
				db = db.Where("processed=false")
			case c2b.ProcessedState_PROCESSED:
				db = db.Where("processed=true")
			}
		}
	}

	var collectionCount int64

	if pageToken == "" {
		err = db.Count(&collectionCount).Error
		if err != nil {
			b2cAPI.Logger.Errorln(err)
			return nil, errs.WrapMessage(codes.Internal, "failed to get count of b2c txs")
		}
	}

	txs := make([]*Payment, 0, pageSize+1)

	err = db.Find(&txs).Error
	switch {
	case err == nil:
	default:
		b2cAPI.Logger.Errorln(err)
		return nil, errs.WrapMessage(codes.Internal, "failed to get b2c transactions")
	}

	pbs := make([]*b2c.B2CPayment, 0, len(txs))

	for i, db := range txs {
		pb, err := B2CPaymentPB(db)
		if err != nil {
			return nil, err
		}

		if i == int(pageSize) {
			break
		}

		pbs = append(pbs, pb)
		switch req.GetFilter().GetOrderField() {
		case b2c.OrderField_PAYMENT_ID:
			key = fmt.Sprint(db.ID)
		case b2c.OrderField_TRANSACTION_TIMESTAMP:
			key = db.TransactionTime.UTC().Format(time.RFC3339)
		}
	}

	var token string
	if len(txs) > int(pageSize) {
		// Next page token
		token = base64.StdEncoding.EncodeToString([]byte(key))
	}

	return &b2c.ListB2CPaymentsResponse{
		NextPageToken:   token,
		B2CPayments:     pbs,
		CollectionCount: collectionCount,
	}, nil
}

func (b2cAPI *b2cAPIServer) ProcessB2CPayment(
	ctx context.Context, req *b2c.ProcessB2CPaymentRequest,
) (*emptypb.Empty, error) {
	// Authorization
	_, err := b2cAPI.AuthAPI.AuthorizeAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	var key int
	switch {
	case req == nil:
		return nil, errs.NilObject("process request")
	case req.PaymentId == "":
		return nil, errs.MissingField("payment id")
	default:
		key, _ = strconv.Atoi(req.PaymentId)
	}

	if key != 0 {
		err = b2cAPI.SQLDB.Model(&Payment{}).Unscoped().Where("id=?", key).
			Update("processed", req.Processed).Error
	} else {
		err = b2cAPI.SQLDB.Model(&Payment{}).Unscoped().Where("transaction_id=?", req.PaymentId).
			Update("processed", req.Processed).Error
	}
	switch {
	case err == nil:
	default:
		return nil, errs.FailedToUpdate("b2c transaction", err)
	}

	return &emptypb.Empty{}, nil
}

func firstVal(A ...string) string {
	for _, s := range A {
		if s != "" {
			return s
		}
	}
	return ""
}

func (b2cAPI *b2cAPIServer) PublishB2CPayment(
	ctx context.Context, req *b2c.PublishB2CPaymentRequest,
) (*emptypb.Empty, error) {
	// Authentication
	_, err := b2cAPI.AuthAPI.AuthorizeAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case req == nil:
		return nil, errs.NilObject("publish request")
	case req.PublishMessage == nil:
		return nil, errs.MissingField("publish message")
	}

	pb := req.GetPublishMessage().GetPayment()

	// Get payment if missing
	if pb.GetPaymentId() != "" && req.GetPublishMessage().GetPaymentId() != "" {
		pb, err = b2cAPI.GetB2CPayment(ctx, &b2c.GetB2CPaymentRequest{
			PaymentId: req.GetPublishMessage().GetPaymentId(),
		})
		if err != nil {
			b2cAPI.Logger.Errorln(err)
		}
	}

	// Update payment value
	req.PublishMessage.Payment = pb

	// Marshal publish message
	bs, err := proto.Marshal(req.PublishMessage)
	if err != nil {
		return nil, errs.FromProtoMarshal(err, "publish message")
	}

	// Channel name in request override default channel name
	channel := firstVal(req.GetPublishMessage().GetPublishInfo().GetChannelName(), b2cAPI.PublishChannel)

	// Publish based on state
	switch req.ProcessedState {
	case c2b.ProcessedState_PROCESS_STATE_UNSPECIFIED:
		err = b2cAPI.RedisDB.Publish(ctx, channel, bs).Err()
		if err != nil {
			return nil, errs.RedisCmdFailed(err, "PUBSUB")
		}
	case c2b.ProcessedState_NOT_PROCESSED:
		// Publish only if the processed state is false
		if !pb.Processed {
			err = b2cAPI.RedisDB.Publish(ctx, channel, bs).Err()
			if err != nil {
				return nil, errs.RedisCmdFailed(err, "PUBSUB")
			}
		}
	case c2b.ProcessedState_PROCESSED:
		// Publish only if the processed state is true
		if pb.Processed {
			err = b2cAPI.RedisDB.Publish(ctx, channel, bs).Err()
			if err != nil {
				return nil, errs.RedisCmdFailed(err, "PUBSUB")
			}
		}
	}

	return &emptypb.Empty{}, nil
}

func (b2cAPI *b2cAPIServer) ListDailyStats(
	ctx context.Context, req *b2c.ListDailyStatsRequest,
) (*b2c.StatsResponse, error) {
	// Validation
	switch {
	case req == nil:
		return nil, errs.NilObject("list request")
	}

	var (
		pageSize      = req.GetPageSize()
		pageToken     = req.GetPageToken()
		orgShortCodes = req.GetFilter().GetOrganizationShortCodes()
		ID            uint
		err           error
	)

	if pageSize <= 0 || pageSize > defaultPageSize {
		pageSize = defaultPageSize
	}

	if pageToken != "" {
		bs, err := base64.StdEncoding.DecodeString(req.GetPageToken())
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "failed to parse page token")
		}
		v, err := strconv.ParseUint(string(bs), 10, 64)
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "incorrect page token")
		}
		ID = uint(v)
	}

	db := b2cAPI.SQLDB.Limit(int(pageSize + 1)).Order("id DESC")
	if ID != 0 {
		db = db.Where("id<?", ID)
	}

	// Apply filters
	if len(orgShortCodes) > 0 {
		db = db.Where("short_code IN(?)", orgShortCodes)
	}
	if req.GetFilter().GetStartTimeSeconds() < req.GetFilter().GetEndTimeSeconds() {
		db = db.Where("created_at BETWEEN ? AND ?", req.GetFilter().GetStartTimeSeconds(), req.GetFilter().GetEndTimeSeconds())
	} else if len(req.GetFilter().GetTxDates()) > 0 {
		db = db.Where("date IN (?)", req.GetFilter().GetTxDates())
	}

	stats := make([]*DailyStat, 0, pageSize+1)

	err = db.Find(&stats).Error
	switch {
	case err == nil:
	default:
		return nil, errs.FailedToFind("c2b stat", err)
	}

	dailyStatsPB := make([]*b2c.DailyStat, 0, len(stats))

	for i, stat := range stats {
		statPB, err := GetDailyStatPB(stat)
		if err != nil {
			return nil, err
		}

		// Ignore the last element
		if i == int(pageSize) {
			break
		}

		dailyStatsPB = append(dailyStatsPB, statPB)
		ID = stat.ID
	}

	var token string
	if len(stats) > int(pageSize) {
		// Next page token
		token = base64.StdEncoding.EncodeToString([]byte(fmt.Sprint(ID)))
	}

	return &b2c.StatsResponse{
		Stats:         dailyStatsPB,
		NextPageToken: token,
	}, nil
}
