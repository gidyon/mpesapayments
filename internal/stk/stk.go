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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gidyon/micro/pkg/grpc/auth"
	"github.com/gidyon/micro/utils/errs"
	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"github.com/gidyon/mpesapayments/pkg/api/stk"
	redis "github.com/go-redis/redis/v8"
	"github.com/speps/go-hashids"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
)

const (
	// FailedTxList is redis list for failed mpesa transactions
	FailedTxList            = "mpesa:stk:failedtx:list"
	failedTxListv2          = "mpesa:stk:failedtx:list"
	unprocessedTxList       = "mpesa:stk:failedtx:list:failed"
	pendingConfirmationSet  = "mpesa:stk:pendingtx:set"
	pendingConfirmationList = "mpesa:stk:pendingtx:list"
	publishChannel          = "mpesa:stk:pubsub"
	bulkInsertSize          = 1000
)

// HTTPClient makes mocking test easier
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type stkAPIServer struct {
	stk.UnimplementedStkPushAPIServer
	lastProcessedTxTime time.Time
	mpesaAPI            mpesapayment.LipaNaMPESAServer
	insertChan          chan *PayloadStk
	insertTimeOut       time.Duration
	ctxAdmin            context.Context
	*Options
}

// Options contain parameters passed for creating stk service
type Options struct {
	SQLDB                     *gorm.DB
	RedisDB                   *redis.Client
	Logger                    grpclog.LoggerV2
	AuthAPI                   auth.API
	PaginationHasher          *hashids.HashID
	OptionsSTK                *OptionsSTK
	HTTPClient                HTTPClient
	UpdateAccessTokenDuration time.Duration
	WorkerDuration            time.Duration
	InitiatorExpireDuration   time.Duration
	RedisKeyPrefix            string
	PublishChannel            string
	DisableMpesaService       bool
	DisablePublishing         bool
}

// ValidateOptions validates options required by stk service
func ValidateOptions(opt *Options) error {
	var err error
	switch {
	case opt == nil:
		err = errs.NilObject("options")
	case opt.SQLDB == nil:
		err = errs.NilObject("sql db")
	case opt.RedisDB == nil && !opt.DisablePublishing:
		err = errs.NilObject("redis db")
	case opt.Logger == nil:
		err = errs.NilObject("logger")
	case opt.AuthAPI == nil:
		err = errs.NilObject("auth API")
	case opt.PaginationHasher == nil:
		err = errs.NilObject("pagination PaginationHasher")
	case opt.HTTPClient == nil:
		err = errs.NilObject("http client")
	case opt.RedisKeyPrefix == "":
		err = errs.MissingField("keys prefix")
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
	ctx context.Context, opt *Options, mpesaAPI mpesapayment.LipaNaMPESAServer,
) (stk.StkPushAPIServer, error) {
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
		ctx, &auth.Payload{Group: auth.AdminGroup()}, time.Now().Add(10*365*24*time.Hour))
	if err != nil {
		return nil, fmt.Errorf("failed to generate auth token: %v", err)
	}

	md := metadata.Pairs(auth.Header(), fmt.Sprintf("%s %s", auth.Scheme(), token))

	ctxAdmin := metadata.NewIncomingContext(ctx, md)

	// Authenticate the jwt
	ctxAdmin, err = opt.AuthAPI.AuthFunc(ctxAdmin)
	if err != nil {
		return nil, err
	}

	// API server
	stkAPI := &stkAPIServer{
		mpesaAPI:      mpesaAPI,
		Options:       opt,
		insertChan:    make(chan *PayloadStk, bulkInsertSize),
		insertTimeOut: time.Duration(5 * time.Second),
		ctxAdmin:      ctxAdmin,
	}

	stkAPI.Logger.Infof("Publishing to stk consumers on channel: %v", stkAPI.addPrefix(stkAPI.PublishChannel))

	// Auto migration
	if !stkAPI.SQLDB.Migrator().HasTable(StkTable) {
		err = stkAPI.SQLDB.Migrator().AutoMigrate(&PayloadStk{})
		if err != nil {
			return nil, err
		}
	}

	// workerDur := time.Minute * 30
	// if opt.WorkerDuration > 0 {
	// 	workerDur = opt.WorkerDuration
	// }

	// // Start worker for failed stk transactions
	// go stkAPI.worker(ctx, workerDur)

	dur := time.Minute * 45
	if opt.UpdateAccessTokenDuration > 0 {
		dur = opt.UpdateAccessTokenDuration
	}

	// Worker for updating access token
	go stkAPI.updateAccessTokenWorker(ctx, dur)

	// worker for handling inserts
	go stkAPI.insertWorker(ctx)

	return stkAPI, nil
}

// ValidateStkPayload validates MPESA transaction
func ValidateStkPayload(payload *stk.StkPayload) error {
	var err error
	switch {
	case payload == nil:
		err = errs.NilObject("stk payload")
	case payload.CheckoutRequestId == "":
		// err = errs.MissingField("checkout request id")
	case payload.MerchantRequestId == "":
		// err = errs.MissingField("merchant request id")
	case payload.PhoneNumber == "" || payload.PhoneNumber == "0":
		err = errs.MissingField("phone number")
	case payload.Amount == "":
		err = errs.MissingField("transaction amount")
	case payload.ResultCode == "":
		err = errs.MissingField("result code")
	case payload.ResultDesc == "":
		err = errs.MissingField("result description")
	case payload.TransactionDate == "":
		err = errs.MissingField("transaction time")
	}
	return err
}

// GetMpesaSTKPushKey retrives key storing initiator key
func GetMpesaSTKPushKey(msisdn, keyPrefix string) string {
	return fmt.Sprintf("mpesa:stkpush:%s", msisdn)
}

// GetMpesaSTKPayloadKey retrives key storing payload of stk initiator
func GetMpesaSTKPayloadKey(initiatorID, keyPrefix string) string {
	return fmt.Sprintf("mpesa:stkpayload:%s", initiatorID)
}

func (stkAPI *stkAPIServer) addPrefix(key string) string {
	return fmt.Sprintf("%s:%s", stkAPI.RedisKeyPrefix, key)
}

func (stkAPI *stkAPIServer) InitiateSTKPush(
	ctx context.Context, initReq *stk.InitiateSTKPushRequest,
) (*stk.InitiateSTKPushResponse, error) {
	if stkAPI.DisablePublishing {
		return nil, errs.WrapMessage(codes.PermissionDenied, "initiating stk push disabled")
	}

	// Validation
	switch {
	case initReq == nil:
		return nil, errs.NilObject("initiate request")
	case initReq.InitiatorId == "":
		return nil, errs.MissingField("initiator id")
	case initReq.Phone == "":
		return nil, errs.MissingField("phone")
	case initReq.Amount <= 0:
		return nil, errs.MissingField("amount")
	case initReq.Payload == nil:
		return nil, errs.MissingField("stk payload")
	}

	txKey := GetMpesaSTKPushKey(initReq.Phone, stkAPI.RedisKeyPrefix)

	// Marshal initReq
	bs, err := proto.Marshal(initReq)
	if err != nil {
		stkAPI.RedisDB.Del(ctx, txKey)
		return nil, errs.FromProtoMarshal(err, "init request")
	}

	// Check whether another transaction is under way
	exists, err := stkAPI.RedisDB.Exists(ctx, txKey).Result()
	if err != nil {
		return nil, errs.RedisCmdFailed(err, "exists")
	}

	if exists == 1 {
		return &stk.InitiateSTKPushResponse{
			Progress: false,
			Message:  "Another MPESA STK is currently underway. Please wait.",
		}, nil
	}

	pipeliner := stkAPI.RedisDB.TxPipeline()

	// Key to payload
	txKey2 := GetMpesaSTKPayloadKey(initReq.GetInitiatorId(), stkAPI.RedisKeyPrefix)

	// Save initiator key in cache for 100 seconds
	err = pipeliner.Set(ctx, txKey, txKey2, 100*time.Second).Err()
	if err != nil {
		stkAPI.RedisDB.Del(ctx, txKey)
		return nil, errs.RedisCmdFailed(err, "set")
	}

	// Save initiator payload in cache for ne week
	err = pipeliner.Set(ctx, txKey2, bs, stkAPI.InitiatorExpireDuration).Err()
	if err != nil {
		stkAPI.RedisDB.Del(ctx, txKey)
		return nil, errs.RedisCmdFailed(err, "set")
	}

	// Execute transaction
	_, err = pipeliner.Exec(ctx)
	if err != nil {
		stkAPI.RedisDB.Del(ctx, txKey)
		return nil, errs.RedisCmdFailed(err, "exec")
	}

	// Correct phone number
	phoneNumber := strings.TrimSpace(initReq.Phone)
	phoneNumber = strings.TrimPrefix(initReq.Phone, "+")
	phoneNumber = strings.TrimPrefix(initReq.Phone, "0")
	if strings.HasPrefix(phoneNumber, "7") {
		phoneNumber = "254" + phoneNumber
	}

	// STK Payload
	stkPayload := &PayloadStkRequest{
		BusinessShortCode: stkAPI.OptionsSTK.BusinessShortCode,
		Password:          stkAPI.OptionsSTK.password,
		Timestamp:         stkAPI.OptionsSTK.Timestamp,
		TransactionType:   "CustomerPayBillOnline",
		Amount:            fmt.Sprint(initReq.Amount),
		PartyA:            phoneNumber,
		PartyB:            stkAPI.OptionsSTK.BusinessShortCode,
		PhoneNumber:       phoneNumber,
		CallBackURL:       stkAPI.OptionsSTK.CallBackURL,
		AccountReference:  stkAPI.OptionsSTK.AccountReference,
		TransactionDesc:   "payload",
	}

	// Json Marshal
	bs, err = json.Marshal(stkPayload)
	if err != nil {
		stkAPI.RedisDB.Del(ctx, txKey)
		return nil, errs.FromJSONMarshal(err, "stkPayload")
	}

	// Create request
	req, err := http.NewRequest(http.MethodPost, stkAPI.OptionsSTK.PostURL, bytes.NewReader(bs))
	if err != nil {
		stkAPI.RedisDB.Del(ctx, txKey)
		return nil, errs.WrapMessage(codes.Internal, "failed to create new request")
	}

	// Update headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", stkAPI.OptionsSTK.accessToken))
	req.Header.Set("Content-Type", "application/json")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Post to MPESA API
		res, err := stkAPI.HTTPClient.Do(req)
		if err != nil {
			stkAPI.RedisDB.Del(ctx, txKey)
			stkAPI.Logger.Errorf("failed to post stk payload to mpesa API: %v", err)
			return
		}

		resData := make(map[string]interface{}, 0)

		err = json.NewDecoder(res.Body).Decode(&resData)
		if err != nil && err != io.EOF {
			stkAPI.RedisDB.Del(ctx, txKey)
			stkAPI.Logger.Errorf("failed to decode mpesa response: %v", err)
			return
		}

		// Check for error
		errMsg, ok := resData["errorMessage"]
		if ok {
			stkAPI.RedisDB.Del(ctx, txKey)
			stkAPI.Logger.Errorf("error happened while sending stk push: %v", errMsg)
			return
		}
	}()

	return &stk.InitiateSTKPushResponse{
		Progress: true,
		Message:  "Processing. Pay popup will come shortly",
	}, nil
}

func (stkAPI *stkAPIServer) CreateStkPayload(
	ctx context.Context, createReq *stk.CreateStkPayloadRequest,
) (*stk.StkPayload, error) {
	// Authorization
	_, err := stkAPI.AuthAPI.AuthorizeGroups(ctx, auth.Admins()...)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case createReq == nil:
		return nil, errs.NilObject("CreateStkPayloadRequest")
	default:
		err = ValidateStkPayload(createReq.Payload)
		if err != nil {
			return nil, err
		}
	}

	stkPayloadDB, err := GetStkPayloadDB(createReq.Payload)
	if err != nil {
		return nil, err
	}

	// Save payload via channel
	go func() {
		stkAPI.insertChan <- stkPayloadDB
	}()

	if !stkAPI.DisableMpesaService {
		// Update mpesa payment
		// stkAPI.mpesaAPI.ProcessMpesaPayment(ctx, &mpesapayment.ProcessMpesaPaymentRequest{
		// 	PaymentId: stkPayloadDB.MpesaReceiptNumber,
		// 	State:     true,
		// })
	}

	return &stk.StkPayload{
		PayloadId:          fmt.Sprint(stkPayloadDB.PayloadID),
		MpesaReceiptNumber: stkPayloadDB.MpesaReceiptNumber,
		CheckoutRequestId:  stkPayloadDB.CheckoutRequestID,
	}, nil
}

func (stkAPI *stkAPIServer) GetStkPayload(
	ctx context.Context, getReq *stk.GetStkPayloadRequest,
) (*stk.StkPayload, error) {
	// Authorization
	err := stkAPI.AuthAPI.AuthenticateRequest(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	var payloadID int
	switch {
	case getReq == nil:
		return nil, errs.NilObject("GetStkPayloadRequest")
	case getReq.PayloadId == "":
		return nil, errs.MissingField("payload id")
	default:
		payloadID, _ = strconv.Atoi(getReq.PayloadId)
	}

	stkPayloadDB := &PayloadStk{}

	if payloadID != 0 {
		err = stkAPI.SQLDB.First(stkPayloadDB, "payload_id=?", payloadID).Error
	} else {
		err = stkAPI.SQLDB.First(stkPayloadDB, "mpesa_receipt_number=?", getReq.PayloadId).Error
	}
	switch {
	case err == nil:
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, errs.DoesNotExist("mpesa payload", getReq.PayloadId)
	}

	return GetStkPayloadPB(stkPayloadDB)
}

const defaultPageSize = 20

func userAllowedPhonesSet(userID string) string {
	return fmt.Sprintf("user:%s:allowedphones", userID)
}
func inGroup(group string, groups []string) bool {
	for _, grp := range groups {
		if grp == group {
			return true
		}
	}
	return false
}
func (stkAPI *stkAPIServer) ListStkPayloads(
	ctx context.Context, listReq *stk.ListStkPayloadsRequest,
) (*stk.ListStkPayloadsResponse, error) {
	// Authorization
	payload, err := stkAPI.AuthAPI.AuthenticateRequestV2(ctx)
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

	// Read from redis list of phone numbers
	var allowedPhones []string
	if stkAPI.DisablePublishing {
		allowedPhones, err = stkAPI.RedisDB.SMembers(
			ctx, stkAPI.addPrefix(userAllowedPhonesSet(payload.ID)),
		).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, errs.RedisCmdFailed(err, "smembers")
		}
	}

	pageSize := listReq.GetPageSize()
	if pageSize <= 0 || pageSize > defaultPageSize {
		if !inGroup(payload.Group, auth.Admins()) {
			pageSize = defaultPageSize
		}
	}

	var payloadID uint

	pageToken := listReq.GetPageToken()
	if pageToken != "" {
		ids, err := stkAPI.PaginationHasher.DecodeInt64WithError(listReq.GetPageToken())
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(
				codes.InvalidArgument, err, "failed to parse page token",
			)
		}
		payloadID = uint(ids[0])
	}

	mpesapayloads := make([]*PayloadStk, 0, pageSize)

	db := stkAPI.SQLDB.Limit(int(pageSize) + 1).Order("payload_id DESC")

	if len(allowedPhones) > 0 {
		db = db.Where("phone_number IN(?)", allowedPhones)
	}

	// Apply payload id filter
	if payloadID != 0 {
		db = db.Where("payload_id<=?", payloadID)
	}

	// Apply filters
	if listReq.Filter != nil {
		startTimestamp := listReq.Filter.GetStartTimestamp()
		endTimestamp := listReq.Filter.GetEndTimestamp()

		// Timestamp filter
		if endTimestamp > startTimestamp {
			db = db.Where("created_at BETWEEN ? AND ?", startTimestamp, endTimestamp)
		} else {
			// Date filter
			if listReq.Filter.TxDate != "" {
				t, err := getTime(listReq.Filter.TxDate)
				if err != nil {
					return nil, err
				}
				db = db.Where("created_at BETWEEN ? AND ?", t, t.Add(time.Hour*24).Unix())
			}
		}

		if len(listReq.Filter.Msisdns) > 0 {
			if payload.Group == auth.AdminGroup() {
				db = db.Where("phone_number IN(?)", listReq.Filter.Msisdns)
			}
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

	err = db.Find(&mpesapayloads).Error
	switch {
	case err == nil:
	default:
		return nil, errs.FailedToFind("mpesa payloads", err)
	}

	payloadPayloadsPB := make([]*stk.StkPayload, 0, len(mpesapayloads))

	for i, payloadPayloadDB := range mpesapayloads {
		payloadPaymenPB, err := GetStkPayloadPB(payloadPayloadDB)
		if err != nil {
			return nil, err
		}

		if i == int(pageSize) {
			break
		}

		payloadPayloadsPB = append(payloadPayloadsPB, payloadPaymenPB)
		payloadID = payloadPayloadDB.PayloadID
	}

	var token string
	if len(mpesapayloads) > int(pageSize) {
		// Next page token
		token, err = stkAPI.PaginationHasher.EncodeInt64([]int64{int64(payloadID)})
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "failed to generate next page token")
		}
	}

	return &stk.ListStkPayloadsResponse{
		NextPageToken: token,
		StkPayloads:   payloadPayloadsPB,
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

func (stkAPI *stkAPIServer) ProcessStkPayload(
	ctx context.Context, processReq *stk.ProcessStkPayloadRequest,
) (*emptypb.Empty, error) {
	// Authorization
	_, err := stkAPI.AuthAPI.AuthorizeGroups(ctx, auth.Admins()...)
	if err != nil {
		return nil, err
	}

	// Validation
	var payloadID int
	switch {
	case processReq == nil:
		return nil, errs.NilObject("process request")
	case processReq.PayloadId == "":
		return nil, errs.MissingField("payload id")
	default:
		payloadID, _ = strconv.Atoi(processReq.PayloadId)
	}

	if payloadID != 0 {
		err = stkAPI.SQLDB.Model(&PayloadStk{}).Unscoped().Where("payload_id=?", payloadID).
			Update("processed", processReq.Processed).Error
	} else {
		err = stkAPI.SQLDB.Model(&PayloadStk{}).Unscoped().Where("mpesa_receipt_number=?", processReq.PayloadId).
			Update("processed", processReq.Processed).Error
	}
	switch {
	case err == nil:
	default:
		return nil, errs.FailedToUpdate("stk payload", err)
	}

	return &emptypb.Empty{}, nil
}

func (stkAPI *stkAPIServer) PublishStkPayload(
	ctx context.Context, pubReq *stk.PublishStkPayloadRequest,
) (*emptypb.Empty, error) {
	if stkAPI.DisablePublishing {
		return nil, errs.WrapMessage(codes.PermissionDenied, "publishing stk payload disabled")
	}

	// Authorization
	_, err := stkAPI.AuthAPI.AuthorizeGroups(ctx, auth.Admins()...)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case pubReq == nil:
		return nil, errs.NilObject("publish request")
	case pubReq.PayloadId == "":
		return nil, errs.MissingField("payload id")
	case pubReq.Payload == nil && !pubReq.FromCache:
		return nil, errs.MissingField("stk payload")
	}

	// Get mpesa payload
	mpesaPayload, err := stkAPI.GetStkPayload(ctx, &stk.GetStkPayloadRequest{
		PayloadId: pubReq.PayloadId,
	})
	if err != nil {
		return nil, err
	}

	// Payload to be published
	publishPayload := &stk.PublishMessage{
		PayloadId: pubReq.PayloadId,
		Payload:   pubReq.Payload,
	}

	// Get payload from cache
	if pubReq.FromCache {
		// Get key
		txKey := GetMpesaSTKPayloadKey(mpesaPayload.PhoneNumber, stkAPI.RedisKeyPrefix)
		val, err := stkAPI.RedisDB.Get(ctx, txKey).Result()
		switch {
		case err == nil:
		case errors.Is(err, redis.Nil):
			return nil, errs.WrapMessage(codes.NotFound, "publish payload not found in cache")
		default:
			return nil, errs.RedisCmdFailed(err, "get")
		}

		// Unmarshal
		if val != "" {
			payload := &stk.InitiateSTKPushRequest{}
			err = proto.Unmarshal([]byte(val), payload)
			if err != nil {
				return nil, errs.FromProtoUnMarshal(err, "initiate stk payload")
			}
			publishPayload.Payload = payload.Payload
		}
	}

	// Marshal data
	bs, err := proto.Marshal(publishPayload)
	if err != nil {
		return nil, errs.FromProtoMarshal(err, "publish message")
	}

	channel := stkAPI.addPrefix(stkAPI.PublishChannel)

	// Publish based on state
	switch pubReq.ProcessedState {
	case mpesapayment.ProcessedState_PROCESS_STATE_UNSPECIFIED:
		err = stkAPI.RedisDB.Publish(ctx, channel, bs).Err()
		if err != nil {
			return nil, errs.RedisCmdFailed(err, "PUBSUB")
		}
	case mpesapayment.ProcessedState_PROCESSED:
		// Publish only if the processed state is true
		if mpesaPayload.Processed {
			err = stkAPI.RedisDB.Publish(ctx, channel, bs).Err()
			if err != nil {
				return nil, errs.RedisCmdFailed(err, "PUBSUB")
			}
		}
	case mpesapayment.ProcessedState_NOT_PROCESSED:
		// Publish only if the processed state is false
		if !mpesaPayload.Processed {
			err = stkAPI.RedisDB.Publish(ctx, channel, bs).Err()
			if err != nil {
				return nil, errs.RedisCmdFailed(err, "PUBSUB")
			}
		}
	}

	return &emptypb.Empty{}, nil
}

func (stkAPI *stkAPIServer) PublishAllStkPayload(
	ctx context.Context, pubReq *stk.PublishAllStkPayloadRequest,
) (*emptypb.Empty, error) {
	if stkAPI.DisablePublishing {
		return nil, errs.WrapMessage(codes.PermissionDenied, "publishing stk payload disabled")
	}

	// Authorization
	_, err := stkAPI.AuthAPI.AuthorizeGroups(ctx, auth.Admins()...)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case pubReq == nil:
		return nil, errs.NilObject("publish all request")
	case pubReq.StartTimestamp > time.Now().Unix() || pubReq.EndTimestamp > time.Now().Unix():
		return nil, errs.WrapMessage(codes.InvalidArgument, "cannot work with future times")
	default:
		if pubReq.EndTimestamp != 0 || pubReq.StartTimestamp != 0 {
			if pubReq.EndTimestamp < pubReq.StartTimestamp {
				return nil, errs.WrapMessage(
					codes.InvalidArgument, "start timestamp cannot be greater than end timestamp",
				)
			}
		}
	}

	if pubReq.StartTimestamp == 0 {
		if pubReq.EndTimestamp == 0 {
			pubReq.EndTimestamp = time.Now().Unix()
		}
		pubReq.StartTimestamp = pubReq.EndTimestamp - int64(7*24*60*60)
		if pubReq.StartTimestamp < 0 {
			pubReq.StartTimestamp = 0
		}
	}

	var (
		pageToken string
		pageSize  int32 = 1000
		next            = true
	)

	for next {
		// List transactions
		listRes, err := stkAPI.ListStkPayloads(ctx, &stk.ListStkPayloadsRequest{
			PageToken: pageToken,
			PageSize:  pageSize,
			Filter: &stk.ListStkPayloadFilter{
				ProcessState:   pubReq.ProcessedState,
				StartTimestamp: pubReq.StartTimestamp,
				EndTimestamp:   pubReq.StartTimestamp,
			},
		})
		if err != nil {
			return nil, err
		}
		if listRes.NextPageToken == "" {
			next = false
		}
		pageToken = listRes.NextPageToken

		wg := &sync.WaitGroup{}

		// Publish the mpesa transactions to listeners
		for _, mpesaPB := range listRes.StkPayloads {
			wg.Add(1)

			go func(mpesaPB *stk.StkPayload) {
				defer wg.Done()

				// Publish to consumers
				stkAPI.PublishStkPayload(stkAPI.ctxAdmin, &stk.PublishStkPayloadRequest{
					PayloadId: mpesaPB.PayloadId,
					FromCache: pubReq.FromCache,
				})

			}(mpesaPB)
		}

		wg.Wait()
	}

	return &emptypb.Empty{}, nil
}
