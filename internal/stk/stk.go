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
	"time"

	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"github.com/gidyon/mpesapayments/pkg/api/stk"
	"github.com/gidyon/services/pkg/auth"
	"github.com/gidyon/services/pkg/utils/encryption"
	"github.com/gidyon/services/pkg/utils/errs"
	"github.com/go-redis/redis"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/speps/go-hashids"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/protobuf/proto"
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
)

// HTTPClient makes mocking test easier
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type stkAPIServer struct {
	stk.UnimplementedStkPushAPIServer
	authAPI             auth.Interface
	hasher              *hashids.HashID
	lastProcessedTxTime time.Time
	*Options
}

// Options contain parameters passed for creating stk service
type Options struct {
	SQLDB                     *gorm.DB
	RedisDB                   *redis.Client
	Logger                    grpclog.LoggerV2
	JWTSigningKey             []byte
	OptionsSTK                *OptionsSTK
	HTTPClient                HTTPClient
	UpdateAccessTokenDuration time.Duration
	WorkerDuration            time.Duration
	PublishChannel            string
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
	case len(opt.JWTSigningKey) == 0:
		err = errs.NilObject("jwt signing key")
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
	ConsumerKey       string
	ConsumerSecret    string
	BusinessShortCode string
	AccountReference  string
	Timestamp         string
	Password          string
	CallBackURL       string
	PostURL           string
	QueryURL          string
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
	case opt.Password == "":
		err = errs.MissingField("password")
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
func NewStkAPI(ctx context.Context, opt *Options) (stk.StkPushAPIServer, error) {
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
		err = ValidateOptionsSTK(opt.OptionsSTK)
		if err != nil {
			return nil, err
		}
	}

	opt.OptionsSTK.basicToken = base64.StdEncoding.EncodeToString([]byte(
		opt.OptionsSTK.ConsumerKey + ":" + opt.OptionsSTK.ConsumerSecret,
	))

	// Authentication API
	authAPI, err := auth.NewAPI(opt.JWTSigningKey, "STK API", "trusted accounts")
	if err != nil {
		return nil, err
	}

	// Pagination hasher
	hasher, err := encryption.NewHasher(string(opt.JWTSigningKey))
	if err != nil {
		return nil, fmt.Errorf("failed to generate hash id: %v", err)
	}

	stkAPI := &stkAPIServer{
		Options: opt,
		authAPI: authAPI,
		hasher:  hasher,
	}

	// Auto migration
	if !stkAPI.SQLDB.Migrator().HasTable(StkTable) {
		err = stkAPI.SQLDB.Migrator().AutoMigrate(&PayloadStk{})
		if err != nil {
			return nil, err
		}
	}

	workerDur := time.Minute * 30
	if opt.WorkerDuration > 0 {
		workerDur = opt.WorkerDuration
	}

	// Start worker for failed stk transactions
	go stkAPI.worker(ctx, workerDur)

	dur := time.Minute * 30
	if opt.UpdateAccessTokenDuration > 0 {
		dur = opt.UpdateAccessTokenDuration
	}

	// Worker for updating access token
	go stkAPI.updateAccessTokenWorker(ctx, dur)

	return stkAPI, nil
}

// ValidateStkPayload validates MPESA transaction
func ValidateStkPayload(payload *stk.StkPayload) error {
	var err error
	switch {
	case payload == nil:
		err = errs.NilObject("stk payload")
	case payload.CheckoutRequestId == "":
		err = errs.MissingField("checkout request id")
	case payload.MerchantRequestId == "":
		err = errs.MissingField("merchant request id")
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

// GetMpesaSTKPushKey retrives hash storing details of an mpesa transaction
func GetMpesaSTKPushKey(msisdn string) string {
	return fmt.Sprintf("mpesa:stkpush:%s", msisdn)
}

func (stkAPI *stkAPIServer) InitiateSTKPush(
	ctx context.Context, initReq *stk.InitiateSTKPushRequest,
) (*stk.InitiateSTKPushResponse, error) {
	// Authentication
	err := stkAPI.authAPI.AuthenticateRequest(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case initReq == nil:
		return nil, errs.NilObject("initiate request")
	case initReq.Phone == "":
		return nil, errs.MissingField("phone")
	case initReq.Amount <= 0:
		return nil, errs.MissingField("amount")
	case initReq.Payload == nil:
		return nil, errs.MissingField("stk payload")
	}

	txKey := GetMpesaSTKPushKey(initReq.Phone)

	// Check whether another transaction is under way
	exists, err := stkAPI.RedisDB.Exists(txKey).Result()
	if err != nil {
		return nil, errs.RedisCmdFailed(err, "exists")
	}

	if exists == 1 {
		return &stk.InitiateSTKPushResponse{
			Progress: false,
			Message:  "Another MPESA STK is currently underway. Please wait.",
		}, nil
	}

	// Marshal initReq
	bs, err := proto.Marshal(initReq)
	if err != nil {
		return nil, errs.FromProtoMarshal(err, "init request")
	}

	// Save transaction information in cache for 30 seconds
	err = stkAPI.RedisDB.Set(txKey, bs, 100*time.Second).Err()
	if err != nil {
		return nil, errs.RedisCmdFailed(err, "set")
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
		Password:          stkAPI.OptionsSTK.Password,
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
		stkAPI.RedisDB.Del(txKey)
		return nil, errs.FromJSONMarshal(err, "stkPayload")
	}

	// Create request
	req, err := http.NewRequest(http.MethodPost, stkAPI.OptionsSTK.PostURL, bytes.NewReader(bs))
	if err != nil {
		stkAPI.RedisDB.Del(txKey)
		return nil, errs.WrapMessage(codes.Internal, "failed to create new request")
	}

	// Update headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", stkAPI.OptionsSTK.accessToken))
	req.Header.Set("Content-Type", "application/json")

	go func() {
		// Post to MPESA API
		res, err := stkAPI.HTTPClient.Do(req)
		if err != nil {
			stkAPI.RedisDB.Del(txKey)
			stkAPI.Logger.Errorf("failed to post stk payload to mpesa API: %v", err)
			return
		}

		resData := make(map[string]interface{}, 0)

		err = json.NewDecoder(res.Body).Decode(&resData)
		if err != nil && err != io.EOF {
			stkAPI.RedisDB.Del(txKey)
			stkAPI.Logger.Errorf("failed to decode mpesa response: %v", err)
			return
		}

		// Check for error
		errMsg, ok := resData["errorMessage"]
		if ok {
			stkAPI.RedisDB.Del(txKey)
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
	// Authentication
	_, err := stkAPI.authAPI.AuthorizeGroups(ctx, auth.Admins()...)
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

	// Save model
	err = stkAPI.SQLDB.Create(stkPayloadDB).Error
	switch {
	case err == nil:
	case strings.Contains(strings.ToLower(err.Error()), "duplicate"):
		stkAPI.Logger.Infoln("duplicate request skipped")
		return &stk.StkPayload{
			PayloadId:          fmt.Sprint(stkPayloadDB.PayloadID),
			MpesaReceiptNumber: stkPayloadDB.MpesaReceiptNumber,
			CheckoutRequestId:  stkPayloadDB.CheckoutRequestID,
		}, nil
	default:
		return nil, errs.FailedToSave("mpesa payload", err)
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
	// Authentication
	err := stkAPI.authAPI.AuthenticateRequest(ctx)
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
		payloadID, err = strconv.Atoi(getReq.PayloadId)
		if err != nil {
			return nil, errs.IncorrectVal("payload id")
		}
	}

	stkPayloadDB := &PayloadStk{}

	err = stkAPI.SQLDB.First(stkPayloadDB, "payload_id=?", payloadID).Error
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

func (stkAPI *stkAPIServer) ListStkPayloads(
	ctx context.Context, listReq *stk.ListStkPayloadsRequest,
) (*stk.ListStkPayloadsResponse, error) {
	// Authentication
	payload, err := stkAPI.authAPI.AuthenticateRequestV2(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case listReq == nil:
		return nil, errs.NilObject("list request")
	}

	// Read from redis list of phone numbers
	allowedPhones, err := stkAPI.RedisDB.SMembers(userAllowedPhonesSet(payload.ID)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, errs.RedisCmdFailed(err, "smembers")
	}

	pageSize := listReq.GetPageSize()
	if pageSize <= 0 || pageSize > defaultPageSize {
		pageSize = defaultPageSize
	}

	var payloadID uint

	pageToken := listReq.GetPageToken()
	if pageToken != "" {
		ids, err := stkAPI.hasher.DecodeInt64WithError(listReq.GetPageToken())
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "failed to parse page token")
		}
		payloadID = uint(ids[0])
	}

	mpesapayloads := make([]*PayloadStk, 0, pageSize)

	db := stkAPI.SQLDB.Limit(int(pageSize)).Order("payload_id DESC")

	if len(allowedPhones) > 0 {
		db = db.Where("phone_number IN(?)", allowedPhones)
	}

	// Apply payload id filter
	if payloadID != 0 {
		db = db.Where("payload_id<=?", payloadID)
	}

	// Apply filters
	if listReq.Filter != nil {
		if listReq.Filter.TxDate != "" {
			t, err := getTime(listReq.Filter.TxDate)
			if err != nil {
				return nil, err
			}
			db = db.Where("created_at BETWEEN ? AND ?", t, t.Add(time.Hour*24).Unix())
		}
		if len(listReq.Filter.Msisdns) > 0 {
			if payload.Group == auth.AdminGroup() {
				db = db.Where("phone_number IN(?)", listReq.Filter.Msisdns)
			}
		}
		if listReq.Filter.ProcessState != mpesapayment.ProcessedState_PROCESS_STATE_UNSPECIFIED {
			switch listReq.Filter.ProcessState {
			case mpesapayment.ProcessedState_ANY:
			case mpesapayment.ProcessedState_UNPROCESSED_ONLY:
				db = db.Where("processed=false")
			case mpesapayment.ProcessedState_PROCESSED_ONLY:
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

	for _, payloadPayloadDB := range mpesapayloads {
		payloadPaymenPB, err := GetStkPayloadPB(payloadPayloadDB)
		if err != nil {
			return nil, err
		}
		payloadPayloadsPB = append(payloadPayloadsPB, payloadPaymenPB)
		payloadID = payloadPayloadDB.PayloadID
	}

	var token string
	if len(mpesapayloads) >= int(pageSize) {
		// Next page token
		token, err = stkAPI.hasher.EncodeInt64([]int64{int64(payloadID)})
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
) (*empty.Empty, error) {
	// Authentication
	_, err := stkAPI.authAPI.AuthorizeGroups(ctx, auth.Admins()...)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case processReq == nil:
		return nil, errs.NilObject("process request")
	case processReq.PayloadId == "":
		return nil, errs.MissingField("payload id")
	}

	// Get mpesa payload
	mpesaPayload, err := stkAPI.GetStkPayload(ctx, &stk.GetStkPayloadRequest{
		PayloadId: processReq.PayloadId,
	})
	if err != nil {
		return nil, err
	}

	// Check if not already processed process
	if mpesaPayload.Processed == false {
		err = stkAPI.SQLDB.Model(&PayloadStk{}).Where("payload_id=?", processReq.PayloadId).Update("processed", true).Error
		switch {
		case err == nil:
		default:
			return nil, errs.FailedToUpdate("mpesa payload", err)
		}
	}

	return &empty.Empty{}, nil
}

func (stkAPI *stkAPIServer) PublishStkPayload(
	ctx context.Context, pubReq *stk.PublishStkPayloadRequest,
) (*empty.Empty, error) {
	// Authentication
	_, err := stkAPI.authAPI.AuthorizeGroups(ctx, auth.Admins()...)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case pubReq == nil:
		return nil, errs.NilObject("publish request")
	case pubReq.PayloadId == "":
		return nil, errs.MissingField("payload id")
	case pubReq.Payload == nil:
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

	// Marshal data
	bs, err := proto.Marshal(publishPayload)
	if err != nil {
		return nil, errs.FromProtoMarshal(err, "publish message")
	}

	// Publish based on state
	switch pubReq.ProcessedState {
	case mpesapayment.ProcessedState_PROCESS_STATE_UNSPECIFIED:
		// Publish only if the processed state is false
		if !mpesaPayload.Processed {
			err = stkAPI.RedisDB.Publish(publishChannel, bs).Err()
			if err != nil {
				return nil, errs.RedisCmdFailed(err, "PUBSUB")
			}
		}
	case mpesapayment.ProcessedState_ANY:
		err = stkAPI.RedisDB.Publish(publishChannel, bs).Err()
		if err != nil {
			return nil, errs.RedisCmdFailed(err, "PUBSUB")
		}
	case mpesapayment.ProcessedState_UNPROCESSED_ONLY:
		// Publish only if the processed state is false
		if !mpesaPayload.Processed {
			err = stkAPI.RedisDB.Publish(publishChannel, bs).Err()
			if err != nil {
				return nil, errs.RedisCmdFailed(err, "PUBSUB")
			}
		}
	case mpesapayment.ProcessedState_PROCESSED_ONLY:
		// Publish only if the processed state is true
		if mpesaPayload.Processed {
			err = stkAPI.RedisDB.Publish(publishChannel, bs).Err()
			if err != nil {
				return nil, errs.RedisCmdFailed(err, "PUBSUB")
			}
		}
	}

	return &empty.Empty{}, nil
}

func (stkAPI *stkAPIServer) PublishAllStkPayload(
	ctx context.Context, pubReq *stk.PublishAllStkPayloadRequest,
) (*empty.Empty, error) {
	// Authentication
	_, err := stkAPI.authAPI.AuthorizeGroups(ctx, auth.Admins()...)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case pubReq == nil:
		return nil, errs.NilObject("publish all request")
	}

	var startDate time.Time
	if pubReq.SinceTimeSeconds != 0 {
		startDate = time.Unix(pubReq.SinceTimeSeconds, 0)
	} else {
		startDate = stkAPI.lastProcessedTxTime
	}

	for currDate := startDate; currDate.Unix() >= time.Now().Unix(); currDate = currDate.Add(24 * time.Hour) {
		var (
			nextPageToken  string
			pageSize       int32 = 1000
			shouldContinue bool
		)

		for shouldContinue {
			// List transactions
			listRes, err := stkAPI.ListStkPayloads(ctx, &stk.ListStkPayloadsRequest{
				PageToken: nextPageToken,
				PageSize:  pageSize,
				Filter: &stk.ListStkPayloadFilter{
					TxDate:       currDate.String()[:10],
					ProcessState: pubReq.ProcessedState,
				},
			})
			if err != nil {
				return nil, err
			}
			if listRes.NextPageToken == "" {
				shouldContinue = false
			}
			nextPageToken = listRes.NextPageToken

			// Publish the mpesa transactions to listeners
			for _, mpesaPB := range listRes.StkPayloads {
				err := stkAPI.RedisDB.Publish(publishChannel, mpesaPB.PayloadId).Err()
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return &empty.Empty{}, nil
}
