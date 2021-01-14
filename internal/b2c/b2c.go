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
	"strconv"
	"sync"
	"time"

	"github.com/gidyon/micro/v2/pkg/middleware/grpc/auth"
	"github.com/gidyon/micro/v2/utils/errs"
	"github.com/gidyon/mpesapayments/pkg/api/b2c"
	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"github.com/gidyon/mpesapayments/pkg/payload"
	"github.com/go-redis/redis/v8"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/speps/go-hashids"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
)

const (
	// FailedTxList is redis list for failed mpesa transactions
	FailedTxList            = "mpesa:b2c:failedtx:list"
	failedTxListv2          = "mpesa:b2c:failedtx:list"
	unprocessedTxList       = "mpesa:b2c:failedtx:list:failed"
	pendingConfirmationSet  = "mpesa:b2c:pendingtx:set"
	pendingConfirmationList = "mpesa:b2c:pendingtx:list"
	publishChannel          = "mpesa:b2c:pubsub"
	bulkInsertSize          = 1000

	// InitiatorID ...
	InitiatorID = "initiator_id"
	// RequestIDQuery ...
	RequestIDQuery = "request_id"
	// ShortCodeQuery ...
	ShortCodeQuery = "short_code"
	// MSISDNQuery ...
	MSISDNQuery = "msisdn"
	// PublishLocalQuery ...
	PublishLocalQuery = "publish_local"
	// PublishGlobalQuery ...
	PublishGlobalQuery = "publish_global"
	// DropQuery ...
	DropQuery = "drop"
)

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

type pubsub struct {
	mu   *sync.RWMutex // guards subs
	subs map[string]chan struct{}
}

func (pb *pubsub) subcribe(subscription string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.subs[subscription] = make(chan struct{}, 1)
}

func (pb *pubsub) unsubcribe(subscription string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if _, ok := pb.subs[subscription]; ok {
		close(pb.subs[subscription])
		delete(pb.subs, subscription)
	}
}

func (pb *pubsub) wait(subscription string) <-chan struct{} {
	pb.mu.RLock()
	ch, ok := pb.subs[subscription]
	pb.mu.RUnlock()

	if ok {
		<-ch
	}

	return ch
}

func (pb *pubsub) release(subscription string) {
	pb.mu.Lock()
	delete(pb.subs, subscription)
	pb.mu.Unlock()
}

type incomingPayment struct {
	payment *Payment
	publish bool
}

type b2cAPIServer struct {
	b2c.UnimplementedB2CAPIServer
	insertChan    chan *incomingPayment
	insertTimeOut time.Duration
	*Options
	*pubsub
	ctxAdmin context.Context
}

// Options contains options for starting b2c service
type Options struct {
	RedisKeyPrefix   string
	PublishChannel   string
	B2CLocalTopic    string
	QueryBalanceURL  string
	B2CURL           string
	ReversalURL      string
	SQLDB            *gorm.DB
	RedisDB          *redis.Client
	Logger           grpclog.LoggerV2
	AuthAPI          auth.API
	PaginationHasher *hashids.HashID
	HTTPClient       httpClient
	OptionsB2C       *OptionsB2C
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
	case opt.PaginationHasher == nil:
		err = errs.NilObject("pagination hasher")
	case opt.HTTPClient == nil:
		err = errs.NilObject("http client")
	case opt.OptionsB2C == nil:
		err = errs.NilObject("b2c options")
	case opt.RedisKeyPrefix == "":
		err = errs.MissingField("keys prefix")
	case opt.PublishChannel == "":
		err = errs.MissingField("publish channel")
	case opt.B2CLocalTopic == "":
		err = errs.MissingField("b2c local channel")
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

// ValidateOptionsB2C validates stk options
func ValidateOptionsB2C(opt *OptionsB2C) error {
	var err error
	switch {
	case opt == nil:
		err = errs.NilObject("stk options")
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
		err = errs.MissingField("queue timeuout url")
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
		insertChan:    make(chan *incomingPayment, bulkInsertSize),
		insertTimeOut: time.Duration(5 * time.Second),
		Options:       opt,
		pubsub:        &pubsub{mu: &sync.RWMutex{}, subs: map[string]chan struct{}{}},
		ctxAdmin:      ctxAdmin,
	}

	b2cAPI.Logger.Infof("Publishing to b2c consumers on channel: %v", AddPrefix(b2cAPI.PublishChannel, b2cAPI.RedisKeyPrefix))

	// Auto migration
	if !b2cAPI.SQLDB.Migrator().HasTable(&Payment{}) {
		err = b2cAPI.SQLDB.Migrator().AutoMigrate(&Payment{})
		if err != nil {
			return nil, err
		}
	}

	// Worker for updating access token
	go b2cAPI.updateAccessTokenWorker(ctx, 30*time.Minute)

	// Start worker to insert mpesa transactions
	go b2cAPI.insertWorker(ctx)

	// Start worker to reconcile subscriptions
	go b2cAPI.subscriptionsWorker(ctx)

	return b2cAPI, nil
}

type queryOptions struct {
	initiatorID          string
	requestID            string
	msisdn               string
	shortCode            string
	publishLocalChannel  string
	publishGlobalChannel string
	dropTransaction      bool
}

func (b2cAPI *b2cAPIServer) QueryTransactionStatus(
	ctx context.Context, req *b2c.QueryTransactionStatusRequest,
) (*b2c.QueryResponse, error) {
	return &b2c.QueryResponse{}, nil
}

func addQueryParams(opt *queryOptions, url string) string {
	url = url + "?oops=oops"
	if opt.initiatorID != "" {
		url += fmt.Sprintf("&%s=%s", InitiatorID, opt.initiatorID)
	}
	if opt.requestID != "" {
		url += fmt.Sprintf("&%s=%s", RequestIDQuery, opt.requestID)
	}
	if opt.msisdn != "" {
		url += fmt.Sprintf("&%s=%s", MSISDNQuery, opt.msisdn)
	}
	if opt.shortCode != "" {
		url += fmt.Sprintf("&%s=%s", ShortCodeQuery, opt.shortCode)
	}
	if opt.publishGlobalChannel != "" {
		url += fmt.Sprintf("&%s=%s", PublishGlobalQuery, opt.publishGlobalChannel)
	}
	if opt.publishLocalChannel != "" {
		url += fmt.Sprintf("&%s=%s", PublishLocalQuery, opt.publishLocalChannel)
	}
	if opt.dropTransaction {
		url += fmt.Sprintf("&%s=%s", DropQuery, "true")
	}
	return url
}

func firstVal(vals ...string) string {
	for _, val := range vals {
		if val != "" {
			return val
		}
	}
	return ""
}

// AddPrefix adds a prefix to the key
func AddPrefix(key, prefix string) string {
	return fmt.Sprintf("%s:%s", prefix, key)
}

func (b2cAPI *b2cAPIServer) QueryAccountBalance(
	ctx context.Context, queryReq *b2c.QueryAccountBalanceRequest,
) (*b2c.QueryAccountBalanceResponse, error) {
	// Authorize request
	_, err := b2cAPI.AuthAPI.AuthorizeAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Validate request
	switch {
	case queryReq == nil:
		return nil, errs.NilObject("query request")
	case queryReq.PartyA == 0:
		return nil, errs.MissingField("party")
	case queryReq.IdentifierType == b2c.QueryAccountBalanceRequest_QUERY_ACCOUNT_UNSPECIFIED:
		return nil, errs.MissingField("identifier type")
	}

	requestID := firstVal(queryReq.RequestId, uuid.New().String())

	// Subscribe
	b2cAPI.subcribe(requestID)
	defer b2cAPI.release(requestID)

	queryOptions := &queryOptions{
		initiatorID:          queryReq.InitiatorId,
		requestID:            requestID,
		msisdn:               fmt.Sprint(queryReq.PartyA),
		shortCode:            fmt.Sprint(queryReq.PartyA),
		publishLocalChannel:  b2cAPI.B2CLocalTopic,
		publishGlobalChannel: b2cAPI.PublishChannel,
		dropTransaction:      true,
	}

	// Send the request to safaricom
	queryBalPayload := &payload.AccountBalanceRequest{
		CommandID:          "AccountBalance",
		PartyA:             int32(queryReq.PartyA),
		IdentifierType:     int32(queryReq.IdentifierType),
		Remarks:            queryReq.Remarks,
		Initiator:          b2cAPI.OptionsB2C.InitiatorUsername,
		SecurityCredential: b2cAPI.OptionsB2C.InitiatorEncryptedPassword,
		QueueTimeOutURL:    addQueryParams(queryOptions, b2cAPI.OptionsB2C.QueueTimeOutURL),
		ResultURL:          addQueryParams(queryOptions, b2cAPI.OptionsB2C.ResultURL),
	}

	// Json Marshal
	bs, err := json.Marshal(queryBalPayload)
	if err != nil {
		return nil, errs.FromJSONMarshal(err, "stkPayload")
	}

	// Create request
	req, err := http.NewRequest(http.MethodPost, b2cAPI.QueryBalanceURL, bytes.NewReader(bs))
	if err != nil {
		return nil, errs.WrapMessage(codes.Internal, "failed to create request to query balance")
	}

	// Update headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", b2cAPI.OptionsB2C.accessToken))
	req.Header.Set("Content-Type", "application/json")

	// Post to MPESA API
	res, err := b2cAPI.HTTPClient.Do(req)
	if err != nil {
		return nil, errs.WrapError(err)
	}

	apiRes := &payload.GenericAPIResponse{}

	err = json.NewDecoder(res.Body).Decode(&apiRes)
	if err != nil && err != io.EOF {
		b2cAPI.Logger.Errorln(err)
		return nil, errs.WrapError(err)
	}

	if apiRes.Succeeded() {
		return nil, errs.WrapMessage(codes.Unknown, apiRes.Error())
	}

	// Wait for response from mpesa server for not more than a minute
	ctxWait, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	select {
	case <-ctxWait.Done():
		return nil, errs.WrapMessage(codes.DeadlineExceeded, "request to mpesa took too long")
	case <-b2cAPI.wait(requestID):
	}

	// Get from cache
	str, err := b2cAPI.RedisDB.Get(ctx, AddPrefix(requestID, b2cAPI.RedisKeyPrefix)).Result()
	if err != nil {
		return nil, errs.RedisCmdFailed(err, "get")
	}

	transaction := &b2c.B2CPayment{}

	err = proto.Unmarshal([]byte(str), transaction)
	if err != nil {
		return nil, errs.FromProtoUnMarshal(err, "b2cpayment")
	}

	return &b2c.QueryAccountBalanceResponse{
		Party:               queryReq.PartyA,
		WorkingAccountFunds: transaction.WorkingAccountFunds,
		UtilityAccountFunds: transaction.UtilityAccountFunds,
		ChargesPaidFunds:    transaction.ChargesPaidFunds,
	}, nil
}

func (b2cAPI *b2cAPIServer) TransferFunds(
	ctx context.Context, transferReq *b2c.TransferFundsRequest,
) (*emptypb.Empty, error) {
	// Authorize request
	_, err := b2cAPI.AuthAPI.AuthorizeAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Validate request
	switch {
	case transferReq == nil:
		return nil, errs.NilObject("transfer request")
	case transferReq.Amount == 0:
		return nil, errs.MissingField("amount")
	case transferReq.CommandId == b2c.TransferFundsRequest_COMMANDID_UNSPECIFIED:
		return nil, errs.MissingField("command id")
	case transferReq.Msisdn == 0:
		return nil, errs.MissingField("msisdn")
	case transferReq.ShortCode == 0:
		return nil, errs.MissingField("short code")
	case transferReq.Remarks == "":
		return nil, errs.MissingField("remarks")
	}

	requestID := firstVal(transferReq.RequestId, uuid.New().String())

	// Subscribe
	b2cAPI.subcribe(requestID)
	defer b2cAPI.release(requestID)

	queryOptions := &queryOptions{
		initiatorID:          transferReq.InitiatorId,
		requestID:            requestID,
		msisdn:               fmt.Sprint(transferReq.Msisdn),
		shortCode:            fmt.Sprint(transferReq.ShortCode),
		publishGlobalChannel: b2cAPI.PublishChannel,
		publishLocalChannel:  b2cAPI.B2CLocalTopic,
		dropTransaction:      false,
	}

	var commandID string
	switch transferReq.CommandId {
	case b2c.TransferFundsRequest_BUSINESS_PAYMENT:
		commandID = "BusinessPayment"
	case b2c.TransferFundsRequest_PROMOTION_PAYMENT:
		commandID = "PromotionPayment"
	case b2c.TransferFundsRequest_SALARY_PAYMENT:
		commandID = "SalaryPayment"
	}

	// Send the request to mpesa API
	queryBalPayload := &payload.B2CRequest{
		InititorName:       b2cAPI.Options.OptionsB2C.InitiatorUsername,
		SecurityCredential: b2cAPI.OptionsB2C.InitiatorEncryptedPassword,
		CommandID:          commandID,
		Amount:             float64(transferReq.Amount),
		PartyA:             transferReq.ShortCode,
		PartyB:             transferReq.Msisdn,
		Remarks:            transferReq.Remarks,
		QueueTimeOutURL:    addQueryParams(queryOptions, b2cAPI.OptionsB2C.QueueTimeOutURL),
		ResultURL:          addQueryParams(queryOptions, b2cAPI.OptionsB2C.ResultURL),
		Occassion:          transferReq.Occassion,
	}

	// Json Marshal
	bs, err := json.Marshal(queryBalPayload)
	if err != nil {
		return nil, errs.FromJSONMarshal(err, "stkPayload")
	}

	// Create request
	req, err := http.NewRequest(http.MethodPost, b2cAPI.B2CURL, bytes.NewReader(bs))
	if err != nil {
		return nil, errs.WrapMessage(codes.Internal, "failed to create new request")
	}

	// Update headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", b2cAPI.OptionsB2C.accessToken))
	req.Header.Set("Content-Type", "application/json")

	// Post to MPESA API
	res, err := b2cAPI.HTTPClient.Do(req)
	if err != nil {
		return nil, errs.WrapError(err)
	}

	apiRes := &payload.GenericAPIResponse{}

	err = json.NewDecoder(res.Body).Decode(&apiRes)
	if err != nil && err != io.EOF {
		return nil, errs.WrapError(err)
	}

	if apiRes.Succeeded() {
		return nil, errs.WrapMessage(codes.Unknown, apiRes.Error())
	}

	// Wait for response from mpesa server for not more than a minute
	ctxWait, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	select {
	case <-ctxWait.Done():
		return nil, errs.WrapMessage(codes.DeadlineExceeded, "request to mpesa took too long")
	case <-b2cAPI.wait(requestID):
	}

	return &emptypb.Empty{}, nil
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
	}

	requestID := firstVal(reverseReq.RequestId, uuid.New().String())

	// Subscribe
	b2cAPI.subcribe(requestID)
	defer b2cAPI.release(requestID)

	queryOptions := &queryOptions{
		initiatorID:          reverseReq.InitiatorId,
		requestID:            requestID,
		shortCode:            fmt.Sprint(reverseReq.ShortCode),
		publishGlobalChannel: b2cAPI.PublishChannel,
		publishLocalChannel:  b2cAPI.B2CLocalTopic,
		dropTransaction:      true,
	}

	// Send the request to mpesa API
	reverseRequest := &payload.ReversalRequest{
		CommandID:              "TransactionReversal",
		ReceiverParty:          reverseReq.ShortCode,
		ReceiverIdentifierType: reverseReq.ReceiverType,
		Remarks:                reverseReq.Remarks,
		Initiator:              b2cAPI.Options.OptionsB2C.InitiatorUsername,
		SecurityCredential:     b2cAPI.OptionsB2C.InitiatorEncryptedPassword,
		QueueTimeOutURL:        addQueryParams(queryOptions, b2cAPI.OptionsB2C.QueueTimeOutURL),
		ResultURL:              addQueryParams(queryOptions, b2cAPI.OptionsB2C.ResultURL),
		TransactionID:          reverseReq.TransactionId,
		Occassion:              reverseReq.Occassion,
	}

	// Json Marshal
	bs, err := json.Marshal(reverseRequest)
	if err != nil {
		return nil, errs.FromJSONMarshal(err, "stkPayload")
	}

	// Create request
	req, err := http.NewRequest(http.MethodPost, b2cAPI.ReversalURL, bytes.NewReader(bs))
	if err != nil {
		return nil, errs.WrapMessage(codes.Internal, "failed to create new request")
	}

	// Update headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", b2cAPI.OptionsB2C.accessToken))
	req.Header.Set("Content-Type", "application/json")

	// Post to MPESA API
	res, err := b2cAPI.HTTPClient.Do(req)
	if err != nil {
		return nil, errs.WrapError(err)
	}

	apiRes := &payload.GenericAPIResponse{}

	err = json.NewDecoder(res.Body).Decode(&apiRes)
	if err != nil && err != io.EOF {
		return nil, errs.WrapError(err)
	}

	if !apiRes.Succeeded() {
		return nil, errs.WrapMessage(codes.Unknown, apiRes.Error())
	}

	// Wait for response from mpesa server for not more than a minute
	ctxWait, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	select {
	case <-ctxWait.Done():
		return nil, errs.WrapMessage(codes.DeadlineExceeded, "request to mpesa took too long")
	case <-b2cAPI.wait(requestID):
	}

	return &emptypb.Empty{}, nil
}

// ValidatePayment validates b2c payment
func ValidatePayment(paymentPB *b2c.B2CPayment) error {
	var err error
	switch {
	case paymentPB == nil:
		err = errs.NilObject("payment pb")
	case paymentPB.ReceiverPartyPublicName == "":
		err = errs.MissingField("receiver public name")
	case paymentPB.ResultDescription == "":
		err = errs.MissingField("result description")
	case paymentPB.TransactionTimestamp == 0:
		err = errs.MissingField("transaction timestamp")
	}
	return err
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
		return nil, errs.NilObject("CreateStkPayloadRequest")
	default:
		err = ValidatePayment(createReq.Payment)
		if err != nil {
			return nil, err
		}
	}

	paymentDB, err := GetB2CPaymentDB(createReq.Payment)
	if err != nil {
		return nil, err
	}

	// Save payment via channel
	go func() {
		b2cAPI.insertChan <- &incomingPayment{
			payment: paymentDB,
			publish: createReq.Publish,
		}
	}()

	return createReq.Payment, nil
}

func (b2cAPI *b2cAPIServer) GetB2CPayment(
	ctx context.Context, getReq *b2c.GetB2CPaymentRequest,
) (*b2c.B2CPayment, error) {
	// Authorization
	err := b2cAPI.AuthAPI.AuthenticateRequest(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	var paymentID int
	switch {
	case getReq == nil:
		return nil, errs.NilObject("get request")
	case getReq.PaymentId == "":
		return nil, errs.MissingField("payment id")
	default:
		paymentID, _ = strconv.Atoi(getReq.PaymentId)
	}

	paymentDB := &Payment{}

	if paymentID != 0 {
		err = b2cAPI.SQLDB.First(paymentDB, "payment_id=?", paymentID).Error
	} else {
		err = b2cAPI.SQLDB.First(paymentDB, "transaction_id=?", getReq.PaymentId).Error
	}
	switch {
	case err == nil:
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, errs.DoesNotExist("mpesa transaction", getReq.PaymentId)
	}

	return GetB2CPaymentPB(paymentDB)
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
	ctx context.Context, listReq *b2c.ListB2CPaymentsRequest,
) (*b2c.ListB2CPaymentsResponse, error) {
	// Authorization
	payload, err := b2cAPI.AuthAPI.AuthenticateRequestV2(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case listReq == nil:
		return nil, errs.NilObject("list request")
	case listReq.PageSize < 0:
		return nil, errs.IncorrectVal("page size")
	}

	pageSize := listReq.GetPageSize()
	if pageSize > defaultPageSize {
		if !b2cAPI.AuthAPI.IsAdmin(payload.Group) {
			pageSize = defaultPageSize
		}
	}

	var paymentID uint

	pageToken := listReq.GetPageToken()
	if pageToken != "" {
		ids, err := b2cAPI.PaginationHasher.DecodeInt64WithError(listReq.GetPageToken())
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(
				codes.InvalidArgument, err, "failed to parse page token",
			)
		}
		paymentID = uint(ids[0])
	}

	transactions := make([]*Payment, 0, pageSize+1)

	db := b2cAPI.SQLDB.Limit(int(pageSize) + 1).Order("payment_id DESC")

	// Apply payment id filter
	if paymentID != 0 {
		db = db.Where("payment_id<=?", paymentID)
	}

	// Apply filters
	if listReq.Filter != nil {
		startTimestamp := listReq.Filter.GetStartTimestamp()
		endTimestamp := listReq.Filter.GetEndTimestamp()

		if endTimestamp > startTimestamp {
			db = db.Where("create_timestamp BETWEEN ? AND ?", startTimestamp, endTimestamp)
		} else if listReq.Filter.TxDate != "" {
			t, err := getTime(listReq.Filter.TxDate)
			if err != nil {
				return nil, err
			}
			db = db.Where("create_timestamp BETWEEN ? AND ?", t.Unix(), t.Add(time.Hour*24).Unix())
		}

		if listReq.Filter.InitiatorId != "" {
			db = db.Where("initiator_id = ?", listReq.Filter.InitiatorId)
		}

		if len(listReq.Filter.Msisdns) > 0 {
			db = db.Where("msisdn IN(?)", listReq.Filter.Msisdns)
		}

		if listReq.Filter.ProcessState != mpesapayment.ProcessedState_PROCESS_STATE_UNSPECIFIED {
			switch listReq.Filter.ProcessState {
			case mpesapayment.ProcessedState_PROCESS_STATE_UNSPECIFIED:
			case mpesapayment.ProcessedState_NOT_PROCESSED:
				db = db.Where("processed=false")
			case mpesapayment.ProcessedState_PROCESSED:
				db = db.Where("processed=true")
			}
		}
	}

	err = db.Find(&transactions).Error
	switch {
	case err == nil:
	default:
		return nil, errs.FailedToFind("mpesa transactions", err)
	}

	transactionsPB := make([]*b2c.B2CPayment, 0, len(transactions))

	for i, paymentDB := range transactions {
		paymentPaymenPB, err := GetB2CPaymentPB(paymentDB)
		if err != nil {
			return nil, err
		}

		if i == int(pageSize) {
			break
		}

		transactionsPB = append(transactionsPB, paymentPaymenPB)
		paymentID = paymentDB.PaymentID
	}

	var token string
	if len(transactions) > int(pageSize) {
		// Next page token
		token, err = b2cAPI.PaginationHasher.EncodeInt64([]int64{int64(paymentID)})
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "failed to generate next page token")
		}
	}

	return &b2c.ListB2CPaymentsResponse{
		NextPageToken: token,
		B2CPayments:   transactionsPB,
	}, nil
}

func (b2cAPI *b2cAPIServer) ProcessB2CPayment(
	ctx context.Context, processReq *b2c.ProcessB2CPaymentRequest,
) (*emptypb.Empty, error) {
	// Authorization
	_, err := b2cAPI.AuthAPI.AuthorizeAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	var paymentID int
	switch {
	case processReq == nil:
		return nil, errs.NilObject("process request")
	case processReq.PaymentId == "":
		return nil, errs.MissingField("payment id")
	default:
		paymentID, _ = strconv.Atoi(processReq.PaymentId)
	}

	if paymentID != 0 {
		err = b2cAPI.SQLDB.Model(&Payment{}).Unscoped().Where("payment_id=?", paymentID).
			Update("processed", processReq.Processed).Error
	} else {
		err = b2cAPI.SQLDB.Model(&Payment{}).Unscoped().Where("transaction_id=?", processReq.PaymentId).
			Update("processed", processReq.Processed).Error
	}
	switch {
	case err == nil:
	default:
		return nil, errs.FailedToUpdate("b2c transaction", err)
	}

	return &emptypb.Empty{}, nil
}
