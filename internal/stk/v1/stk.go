package stk

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
	c2b "github.com/gidyon/mpesapayments/pkg/api/c2b/v1"
	stk "github.com/gidyon/mpesapayments/pkg/api/stk/v1"
	"github.com/gidyon/mpesapayments/pkg/utils/httputils"
	redis "github.com/go-redis/redis/v8"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
)

const (
	// FailedTxList is redis list for failed mpesa transactions
	FailedTxList   = "mpesa:stk:failedtx:list"
	publishChannel = "mpesa:stk:pubsub"
	bulkInsertSize = 1000
)

type incomingPayment struct {
	payment *STKTransaction
}

// HTTPClient makes mocking test easier
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type stkAPIServer struct {
	stk.UnimplementedStkPushAPIServer
	mpesaAPI      c2b.LipaNaMPESAServer
	insertChan    chan *incomingPayment
	insertTimeOut time.Duration
	ctxAdmin      context.Context
	*Options
}

// Options contain parameters passed for creating stk service
type Options struct {
	SQLDB                     *gorm.DB
	RedisDB                   *redis.Client
	Logger                    grpclog.LoggerV2
	AuthAPI                   auth.API
	OptionsSTK                *OptionsSTK
	HTTPClient                HTTPClient
	UpdateAccessTokenDuration time.Duration
	WorkerDuration            time.Duration
	InitiatorExpireDuration   time.Duration
	PublishChannel            string
	DisableMpesaService       bool
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
	case opt.OptionsSTK == nil:
		err = errs.NilObject("stk options")
	}
	return err
}

// OptionsSTK contains options for sending push stk
type OptionsSTK struct {
	AccessTokenURL    string
	PassKey           string
	ConsumerKey       string
	ConsumerSecret    string
	BusinessShortCode string
	AccountReference  string
	Timestamp         string
	CallBackURL       string
	PostURL           string
	QueryURL          string
	password          string
	accessToken       string
	basicToken        string
}

// ValidateOptionsSTK validates stk options
func ValidateOptionsSTK(opt *OptionsSTK) error {
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
	case opt.BusinessShortCode == "":
		err = errs.MissingField("business short code")
	case opt.AccountReference == "":
		err = errs.MissingField("account reference")
	case opt.Timestamp == "":
		err = errs.MissingField("timestamp")
	case opt.PassKey == "":
		err = errs.MissingField("pass key")
	case opt.CallBackURL == "":
		err = errs.MissingField("callback url")
	case opt.PostURL == "":
		err = errs.MissingField("post url")
	case opt.QueryURL == "":
	case opt.accessToken == "":
	case opt.basicToken == "":
	}
	return err
}

// NewStkAPI creates a singleton instance of mpesa stk API
func NewStkAPI(
	ctx context.Context, opt *Options, mpesaAPI c2b.LipaNaMPESAServer,
) (_ stk.StkPushAPIServer, err error) {

	defer func() {
		if err != nil {
			err = errs.WrapErrorWithMsgFunc("Failed to start stk client API service")(err)
		}
	}()

	// Validation
	switch {
	case ctx == nil:
		return nil, errs.NilObject("context")
	default:
		err = ValidateOptions(opt)
		if err != nil {
			return nil, err
		}
		if mpesaAPI == nil && !opt.DisableMpesaService {
			return nil, errs.MissingField("mpesa API")
		}
		err = ValidateOptionsSTK(opt.OptionsSTK)
		if err != nil {
			return nil, err
		}
	}

	// Update Basic Token
	opt.OptionsSTK.basicToken = base64.StdEncoding.EncodeToString([]byte(
		opt.OptionsSTK.ConsumerKey + ":" + opt.OptionsSTK.ConsumerSecret,
	))

	// Update Password
	opt.OptionsSTK.password = base64.StdEncoding.EncodeToString([]byte(
		opt.OptionsSTK.BusinessShortCode + opt.OptionsSTK.PassKey + opt.OptionsSTK.Timestamp,
	))

	// Update publish channel
	if opt.PublishChannel == "" {
		opt.PublishChannel = publishChannel
	}

	// Update expiration duration for stk payloads
	if opt.InitiatorExpireDuration == 0 {
		opt.InitiatorExpireDuration = 7 * 24 * time.Hour
	}

	// Generate jwt for API
	token, err := opt.AuthAPI.GenToken(
		ctx, &auth.Payload{Group: auth.DefaultAdminGroup()}, time.Now().Add(10*365*24*time.Hour))
	if err != nil {
		return nil, fmt.Errorf("failed to generate auth token: %v", err)
	}

	md := metadata.Pairs(auth.Header(), fmt.Sprintf("%s %s", auth.Scheme(), token))

	ctxAdmin := metadata.NewIncomingContext(ctx, md)

	// Authenticate the jwt
	ctxAdmin, err = opt.AuthAPI.AuthorizeFunc(ctxAdmin)
	if err != nil {
		return nil, err
	}

	// API server
	stkAPI := &stkAPIServer{
		mpesaAPI:      mpesaAPI,
		Options:       opt,
		insertChan:    make(chan *incomingPayment, bulkInsertSize),
		insertTimeOut: time.Duration(5 * time.Second),
		ctxAdmin:      ctxAdmin,
	}

	tablePrefix = os.Getenv("STK_TABLE_PREFIX")

	stkAPI.Logger.Infof("Publishing to stk consumers on channel: %v", stkAPI.PublishChannel)

	// Auto migration
	if !stkAPI.SQLDB.Migrator().HasTable(&STKTransaction{}) {
		err = stkAPI.SQLDB.Migrator().AutoMigrate(&STKTransaction{})
		if err != nil {
			return nil, err
		}
	}

	dur := time.Minute * 45
	if opt.UpdateAccessTokenDuration > 0 {
		dur = opt.UpdateAccessTokenDuration
	}

	// Worker for updating access token
	go stkAPI.updateAccessTokenWorker(ctx, dur)

	// worker for handling inserts
	// go stkAPI.insertWorker(ctx)
	_ = stkAPI.insertWorker

	return stkAPI, nil
}

// ValidateStkTransaction validates STK transaction
func ValidateStkTransaction(payload *stk.StkTransaction) error {
	var err error
	switch {
	case payload == nil:
		err = errs.NilObject("stk payload")
	case payload.PhoneNumber == "" || payload.PhoneNumber == "0":
		err = errs.MissingField("phone number")
	case payload.Amount == "":
		err = errs.MissingField("transaction amount")
	case payload.ResultCode == "":
		err = errs.MissingField("result code")
	case payload.ResultDesc == "":
		err = errs.MissingField("result description")
	case payload.TransactionTimestamp == 0:
		err = errs.MissingField("transaction time")
	}
	return err
}

// GetMpesaSTKPushKey retrives key storing initiator key
func GetMpesaSTKPushKey(msisdn string) string {
	return fmt.Sprintf("stkpush:%s", msisdn)
}

// GetMpesaRequestKey is key that initiates data
func GetMpesaRequestKey(requestId string) string {
	return fmt.Sprintf("stk:%s", requestId)
}

func firstVal(A ...string) string {
	for _, s := range A {
		if s != "" {
			return s
		}
	}
	return ""
}

func (stkAPI *stkAPIServer) InitiateSTKPush(
	ctx context.Context, req *stk.InitiateSTKPushRequest,
) (*stk.InitiateSTKPushResponse, error) {
	// Validation
	switch {
	case req == nil:
		return nil, errs.NilObject("request")
	case req.InitiatorId == "":
		return nil, errs.MissingField("inititator id")
	case req.Phone == "":
		return nil, errs.MissingField("phone")
	case req.Amount <= 0:
		return nil, errs.MissingField("amount")
	}

	cacheKey := GetMpesaSTKPushKey(req.Phone)

	// Check whether another transaction is under way
	exists, err := stkAPI.RedisDB.Exists(ctx, cacheKey).Result()
	if err != nil {
		return nil, errs.RedisCmdFailed(err, "exists")
	}

	if exists == 1 {
		return &stk.InitiateSTKPushResponse{
			Progress: false,
			Message:  "Another MPESA STK is currently underway. Please wait.",
		}, nil
	}

	// Save key in cache for 100 seconds
	err = stkAPI.RedisDB.Set(ctx, cacheKey, "active", 100*time.Second).Err()
	if err != nil {
		stkAPI.RedisDB.Del(ctx, cacheKey)
		return nil, errs.RedisCmdFailed(err, "set")
	}

	// Correct phone number
	phoneNumber := strings.TrimSpace(req.Phone)
	phoneNumber = strings.TrimPrefix(phoneNumber, "+")
	phoneNumber = strings.TrimPrefix(phoneNumber, "0")
	if strings.HasPrefix(phoneNumber, "7") {
		phoneNumber = "254" + phoneNumber
	}

	// STK Payload
	stkBody := &STKRequestBody{
		BusinessShortCode: stkAPI.OptionsSTK.BusinessShortCode,
		Password:          stkAPI.OptionsSTK.password,
		Timestamp:         stkAPI.OptionsSTK.Timestamp,
		TransactionType:   "CustomerPayBillOnline",
		Amount:            fmt.Sprint(req.Amount),
		PartyA:            phoneNumber,
		PartyB:            stkAPI.OptionsSTK.BusinessShortCode,
		PhoneNumber:       phoneNumber,
		CallBackURL:       stkAPI.OptionsSTK.CallBackURL,
		AccountReference:  firstVal(req.AccountReference, stkAPI.OptionsSTK.AccountReference),
		TransactionDesc:   firstVal(req.TransactionDesc, "NA"),
	}

	if req.PublishMessage == nil {
		req.PublishMessage = &stk.PublishInfo{
			Payload: map[string]string{},
		}
	}
	if req.PublishMessage.Payload == nil {
		req.PublishMessage.Payload = map[string]string{}
	}
	req.PublishMessage.Payload["short_code"] = stkAPI.OptionsSTK.BusinessShortCode

	// Json Marshal
	bs, err := json.Marshal(stkBody)
	if err != nil {
		stkAPI.RedisDB.Del(ctx, cacheKey)
		return nil, errs.FromJSONMarshal(err, "stkBody")
	}

	// Create request
	reqHtpp, err := http.NewRequest(http.MethodPost, stkAPI.OptionsSTK.PostURL, bytes.NewReader(bs))
	if err != nil {
		stkAPI.RedisDB.Del(ctx, cacheKey)
		return nil, errs.WrapMessage(codes.Internal, "failed to create new request")
	}

	// Update headers
	reqHtpp.Header.Set("Authorization", fmt.Sprintf("Bearer %s", stkAPI.OptionsSTK.accessToken))
	reqHtpp.Header.Set("Content-Type", "application/json")

	httputils.DumpRequest(reqHtpp, "INITIATE STK REQUEST")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Post to MPESA API
		res, err := stkAPI.HTTPClient.Do(reqHtpp)
		if err != nil {
			stkAPI.Logger.Errorf("failed to post stk payload to mpesa API: %v", err)
			return
		}

		httputils.DumpResponse(res, "INITIATE STK RESPONSE")

		resData := make(map[string]interface{})

		err = json.NewDecoder(res.Body).Decode(&resData)
		if err != nil && err != io.EOF {
			stkAPI.Logger.Errorf("failed to decode mpesa response: %v", err)
			return
		}

		// Check for error
		errMsg, ok := resData["errorMessage"]
		if ok {
			stkAPI.Logger.Errorf("error happened while sending stk push: %v", errMsg)
			return
		}

		// Marshal req
		bs, err := proto.Marshal(req)
		if err != nil {
			stkAPI.Logger.Errorln(err)
			return
		}

		requestId := GetMpesaRequestKey(fmt.Sprint(resData["MerchantRequestID"]))

		// Save payload to cache
		err = stkAPI.RedisDB.Set(ctx, requestId, bs, time.Minute*30).Err()
		if err != nil {
			stkAPI.Logger.Errorln(err)
			return
		}
	}()

	return &stk.InitiateSTKPushResponse{
		Progress: true,
		Message:  "Processing. Pay popup will come shortly",
	}, nil
}

func (stkAPI *stkAPIServer) CreateStkTransaction(
	ctx context.Context, req *stk.CreateStkTransactionRequest,
) (*stk.StkTransaction, error) {
	// Authorization
	_, err := stkAPI.AuthAPI.AuthorizeAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case req == nil:
		return nil, errs.NilObject("CreateStkTransactionRequest")
	default:
		err = ValidateStkTransaction(req.Payload)
		if err != nil {
			return nil, err
		}
	}

	db, err := STKTransactionModel(req.Payload)
	if err != nil {
		return nil, err
	}

	err = stkAPI.SQLDB.Create(db).Error
	if err != nil {
		stkAPI.Logger.Errorln(err)
		return nil, errs.WrapMessage(codes.Internal, "failed to create stk transaction")
	}

	return STKTransactionPB(db)
}

func (stkAPI *stkAPIServer) GetStkTransaction(
	ctx context.Context, req *stk.GetStkTransactionRequest,
) (*stk.StkTransaction, error) {
	var err error

	// Validation
	var ID int
	switch {
	case req == nil:
		return nil, errs.NilObject("GetStkTransactionRequest")
	case req.TransactionId == "" && req.MpesaReceiptId == "":
		return nil, errs.MissingField("transaction/mpesa id")
	default:
		if req.TransactionId != "" {
			ID, err = strconv.Atoi(req.TransactionId)
			if err != nil {
				return nil, errs.WrapMessage(codes.InvalidArgument, "transaction id")
			}
		}
	}

	db := &STKTransaction{}

	if req.TransactionId != "" {
		err = stkAPI.SQLDB.First(db, "id=?", ID).Error
	} else if req.MpesaReceiptId != "" {
		err = stkAPI.SQLDB.First(db, "mpesa_receipt_id=?", req.MpesaReceiptId).Error
	}
	switch {
	case err == nil:
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, errs.DoesNotExist("stk transaction", req.TransactionId)
	default:
		stkAPI.Logger.Errorln(err)
		return nil, errs.WrapMessage(codes.Internal, "failed to get stk transaction")
	}

	return STKTransactionPB(db)
}

const defaultPageSize = 100

func userAllowedPhonesSet(userID string) string {
	return fmt.Sprintf("stk:user:%s:allowedphones", userID)
}

func (stkAPI *stkAPIServer) ListStkTransactions(
	ctx context.Context, req *stk.ListStkTransactionsRequest,
) (*stk.ListStkTransactionsResponse, error) {
	// Authorization
	actor, err := stkAPI.AuthAPI.AuthenticateRequestV2(ctx)
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

	// Read from redis list of phone numbers
	var allowedPhones []string
	allowedPhones, err = stkAPI.RedisDB.SMembers(
		ctx, userAllowedPhonesSet(actor.ID),
	).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, errs.RedisCmdFailed(err, "smembers")
	}

	pageSize := req.GetPageSize()
	switch {
	case pageSize <= 0:
		pageSize = defaultPageSize
	case pageSize > defaultPageSize:
		if !stkAPI.AuthAPI.IsAdmin(actor.Group) {
			pageSize = defaultPageSize
		}
	}

	var ID uint

	pageToken := req.GetPageToken()
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

	dbs := make([]*STKTransaction, 0, pageSize+1)

	db := stkAPI.SQLDB.Limit(int(pageSize) + 1).Order("id DESC")
	if ID != 0 {
		db = db.Where("id<?", ID)
	}

	if len(allowedPhones) > 0 {
		db = db.Where("phone_number IN(?)", allowedPhones)
	}

	// Apply filters
	if req.Filter != nil {
		startTimestamp := req.Filter.GetStartTimestamp()
		endTimestamp := req.Filter.GetEndTimestamp()

		// Timestamp filter
		if endTimestamp > startTimestamp {
			db = db.Where("create_time BETWEEN ? AND ?", time.Unix(startTimestamp, 0), time.Unix(endTimestamp, 0))
		} else if req.Filter.TxDate != "" {
			// Date filter
			t, err := getTime(req.Filter.TxDate)
			if err != nil {
				return nil, err
			}
			db = db.Where("create_time BETWEEN ? AND ?", t, t.Add(time.Hour*24))
		}

		if len(req.Filter.Msisdns) > 0 {
			db = db.Where("phone_number IN(?)", req.Filter.Msisdns)
		}

		if req.Filter.ProcessState != c2b.ProcessedState_PROCESS_STATE_UNSPECIFIED {
			switch req.Filter.ProcessState {
			case c2b.ProcessedState_PROCESS_STATE_UNSPECIFIED:
			case c2b.ProcessedState_NOT_PROCESSED:
				db = db.Where("processed=?", "NO")
			case c2b.ProcessedState_PROCESSED:
				db = db.Where("processed=?", "NO")
			}
		}
	}

	err = db.Find(&dbs).Error
	switch {
	case err == nil:
	default:
		return nil, errs.FailedToFind("mpesa payloads", err)
	}

	pbs := make([]*stk.StkTransaction, 0, len(dbs))

	for i, db := range dbs {
		pb, err := STKTransactionPB(db)
		if err != nil {
			return nil, err
		}

		if i == int(pageSize) {
			break
		}

		pbs = append(pbs, pb)

		ID = db.ID
	}

	var token string
	if len(dbs) > int(pageSize) {
		// Next page token
		token = base64.StdEncoding.EncodeToString([]byte(fmt.Sprint(ID)))
	}

	return &stk.ListStkTransactionsResponse{
		NextPageToken:   token,
		StkTransactions: pbs,
	}, nil
}

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

func (stkAPI *stkAPIServer) ProcessStkTransaction(
	ctx context.Context, req *stk.ProcessStkTransactionRequest,
) (*emptypb.Empty, error) {
	// Authorization
	_, err := stkAPI.AuthAPI.AuthorizeAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	var ID int
	switch {
	case req == nil:
		return nil, errs.NilObject("process request")
	case req.TransactionId == "" && req.MpesaReceiptId == "":
		return nil, errs.MissingField("transaction/mpesa id")
	default:
		if req.TransactionId != "" {
			ID, err = strconv.Atoi(req.TransactionId)
			if err != nil {
				return nil, errs.WrapMessage(codes.InvalidArgument, "transaction id")
			}
		}
	}

	processed := "NO"
	if req.Processed {
		processed = "YES"
	}

	if req.TransactionId != "" {
		err = stkAPI.SQLDB.Model(&STKTransaction{}).Unscoped().Where("id=?", ID).
			Update("processed", processed).Error
	} else {
		err = stkAPI.SQLDB.Model(&STKTransaction{}).Unscoped().Where("mpesa_receipt_id=?", req.MpesaReceiptId).
			Update("processed", processed).Error
	}
	switch {
	case err == nil:
	default:
		stkAPI.Logger.Errorln(err)
		return nil, errs.WrapMessage(codes.Internal, "failed to process stk transaction")
	}

	return &emptypb.Empty{}, nil
}

func (stkAPI *stkAPIServer) PublishStkTransaction(
	ctx context.Context, req *stk.PublishStkTransactionRequest,
) (*emptypb.Empty, error) {
	// Authorization
	_, err := stkAPI.AuthAPI.AuthorizeAdmin(ctx)
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

	stkBody := req.PublishMessage.GetTransactionInfo()

	if req.PublishMessage.TransactionId != "" {
		// Get mpesa payload
		v, err := stkAPI.GetStkTransaction(ctx, &stk.GetStkTransactionRequest{
			TransactionId: req.PublishMessage.TransactionId,
		})
		if err == nil {
			stkBody = v
		} else {
			stkAPI.Logger.Errorln(err)
		}
	}

	// Update the value
	req.PublishMessage.TransactionInfo = stkBody

	// Marshal data
	bs, err := proto.Marshal(req.PublishMessage)
	if err != nil {
		return nil, errs.FromProtoMarshal(err, "publish message")
	}

	channel := stkAPI.PublishChannel

	// Publish based on state
	switch req.ProcessedState {
	case c2b.ProcessedState_PROCESS_STATE_UNSPECIFIED:
		err = stkAPI.RedisDB.Publish(ctx, channel, bs).Err()
		if err != nil {
			return nil, errs.RedisCmdFailed(err, "PUBSUB")
		}
	case c2b.ProcessedState_PROCESSED:
		// Publish only if the processed state is true
		if stkBody.Processed {
			err = stkAPI.RedisDB.Publish(ctx, channel, bs).Err()
			if err != nil {
				return nil, errs.RedisCmdFailed(err, "PUBSUB")
			}
		}
	case c2b.ProcessedState_NOT_PROCESSED:
		// Publish only if the processed state is false
		if !stkBody.Processed {
			err = stkAPI.RedisDB.Publish(ctx, channel, bs).Err()
			if err != nil {
				return nil, errs.RedisCmdFailed(err, "PUBSUB")
			}
		}
	}

	return &emptypb.Empty{}, nil
}
