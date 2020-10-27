package mpesapayment

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bitbucket.org/gideonkamau/mpesa-tracking-portal/pkg/api/mpesapayment"
	"github.com/gidyon/services/pkg/auth"
	"github.com/gidyon/services/pkg/utils/encryption"
	"github.com/gidyon/services/pkg/utils/errs"
	"github.com/go-redis/redis"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/speps/go-hashids"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"gorm.io/gorm"
)

const (
	// FailedTxList is redis list for failed mpesa transactions
	FailedTxList            = "mpesa:failedtx:list"
	failedTxListv2          = "mpesa:failedtx:list"
	unprocessedTxList       = "mpesa:failedtx:list:failed"
	pendingConfirmationSet  = "mpesa:pendingtx:set"
	pendingConfirmationList = "mpesa:pendingtx:list"
)

// HTTPClient makes mocking test easier
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type mpesaAPIServer struct {
	authAPI auth.Interface
	hasher  *hashids.HashID
	*Options
}

// Options contain parameters passed to NewMpesaAPIService
type Options struct {
	SQLDB                     *gorm.DB
	RedisDB                   *redis.Client
	Logger                    grpclog.LoggerV2
	JWTSigningKey             []byte
	STKOptions                *STKOptions
	HTTPClient                HTTPClient
	UpdateAccessTokenDuration time.Duration
	WorkerDuration            time.Duration
}

// STKOptions contains options for sending push stk
type STKOptions struct {
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

// NewAPIServerMPESA creates a singleton instance of mpesa API server
func NewAPIServerMPESA(ctx context.Context, opt *Options) (mpesapayment.LipaNaMPESAServer, error) {
	// Validation
	var err error
	switch {
	case ctx == nil:
		err = errs.NilObject("context")
	case opt == nil:
		err = errs.NilObject("options")
	case opt.SQLDB == nil:
		err = errs.NilObject("sql db")
	case opt.RedisDB == nil:
		err = errs.NilObject("redis db")
	case opt.Logger == nil:
		err = errs.NilObject("logger")
	case opt.HTTPClient == nil:
		err = errs.NilObject("http client")
	case opt.JWTSigningKey == nil:
		err = errs.MissingField("jwt signing key")
	case opt.STKOptions == nil:
		err = errs.NilObject("mpesa stk options")
	case opt.STKOptions.AccessTokenURL == "":
		err = errs.MissingField("access token url")
	case opt.STKOptions.ConsumerKey == "":
		err = errs.MissingField("consumer key")
	case opt.STKOptions.ConsumerSecret == "":
		err = errs.MissingField("consumer secret")
	case opt.STKOptions.BusinessShortCode == "":
		err = errs.MissingField("business short code")
	case opt.STKOptions.AccountReference == "":
		err = errs.MissingField("account reference")
	case opt.STKOptions.Timestamp == "":
		err = errs.MissingField("timestamp")
	case opt.STKOptions.Password == "":
		err = errs.MissingField("password")
	case opt.STKOptions.CallBackURL == "":
		err = errs.MissingField("callback url")
	case opt.STKOptions.PostURL == "":
		err = errs.MissingField("mpesa post url")
	}
	if err != nil {
		return nil, err
	}

	opt.STKOptions.basicToken = base64.StdEncoding.EncodeToString([]byte(
		opt.STKOptions.ConsumerKey + ":" + opt.STKOptions.ConsumerSecret,
	))

	// Authentication API
	authAPI, err := auth.NewAPI(opt.JWTSigningKey, "USSD Channel API", "users")
	if err != nil {
		return nil, err
	}

	// Pagination hasher
	hasher, err := encryption.NewHasher(string(opt.JWTSigningKey))
	if err != nil {
		return nil, fmt.Errorf("failed to generate hash id: %v", err)
	}

	mpesaAPI := &mpesaAPIServer{
		Options: opt,
		authAPI: authAPI,
		hasher:  hasher,
	}

	// Auto migration
	if !mpesaAPI.SQLDB.Migrator().HasTable(MpesaTables) {
		err = mpesaAPI.SQLDB.Migrator().AutoMigrate(&Model{})
		if err != nil {
			return nil, err
		}
	}

	return mpesaAPI, nil
}

// ValidateMPESAPayment validates MPESA transaction
func ValidateMPESAPayment(payment *mpesapayment.MPESAPayment) error {
	var err error
	switch {
	case payment == nil:
		err = errs.NilObject("mpesa payment")
	case payment.BusinessShortCode == 0 && payment.TxType == "PAY_BILL":
		err = errs.MissingField("business short code")
	case payment.TxRefNumber == "" && payment.TxType == "PAY_BILL":
		err = errs.MissingField("account number")
	case payment.Msisdn == "" || payment.Msisdn == "0":
		err = errs.MissingField("msisdn")
	case payment.TxType == "":
		err = errs.MissingField("transaction type")
	case payment.TxAmount == 0:
		err = errs.MissingField("transaction amount")
	case payment.TxTimestamp == 0:
		err = errs.MissingField("transaction time")
	}
	return err
}

func (mpesaAPI *mpesaAPIServer) CreateMPESAPayment(
	ctx context.Context, createReq *mpesapayment.CreateMPESAPaymentRequest,
) (*mpesapayment.CreateMPESAPaymentResponse, error) {
	// Authentication
	_, err := mpesaAPI.authAPI.AuthorizeGroups(ctx, auth.AdminGroup())
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case createReq == nil:
		return nil, errs.NilObject("CreateMPESAPaymentRequest")
	default:
		err = ValidateMPESAPayment(createReq.MpesaPayment)
		if err != nil {
			return nil, err
		}
	}

	mpesaDB, err := GetMpesaDB(createReq.MpesaPayment)
	if err != nil {
		return nil, err
	}

	// Save model
	err = mpesaAPI.SQLDB.Create(mpesaDB).Error
	switch {
	case err == nil:
	case strings.Contains(strings.ToLower(err.Error()), "duplicate"):
		mpesaAPI.Logger.Infoln("duplicate request skipped")
		return &mpesapayment.CreateMPESAPaymentResponse{}, nil
	default:
		return nil, errs.FailedToSave("mpesa payment", err)
	}

	return &mpesapayment.CreateMPESAPaymentResponse{
		PaymentId: fmt.Sprint(mpesaDB.PaymentID),
	}, nil
}

func (mpesaAPI *mpesaAPIServer) GetMPESAPayment(
	ctx context.Context, getReq *mpesapayment.GetMPESAPayloadRequest,
) (*mpesapayment.MPESAPayment, error) {
	// Authentication
	err := mpesaAPI.authAPI.AuthenticateRequest(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	var paymentID int
	switch {
	case getReq == nil:
		return nil, errs.NilObject("GetMPESAPayloadRequest")
	case getReq.PaymentId == "":
		return nil, errs.MissingField("payment id")
	default:
		paymentID, err = strconv.Atoi(getReq.PaymentId)
		if err != nil {
			return nil, errs.IncorrectVal("payment id")
		}
	}

	mpesaDB := &Model{}

	err = mpesaAPI.SQLDB.First(mpesaDB, "payment_id=?", paymentID).Error
	switch {
	case err == nil:
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, errs.DoesNotExist("mpesa payment", getReq.PaymentId)
	}

	return GetMpesaPB(mpesaDB)
}

const defaultPageSize = 20

func userAllowedAccSet(userID string) string {
	return fmt.Sprintf("user:%s:allowedaccount", userID)
}

func userAllowedPhonesSet(userID string) string {
	return fmt.Sprintf("user:%s:allowedphones", userID)
}

func (mpesaAPI *mpesaAPIServer) ListMPESAPayments(
	ctx context.Context, listReq *mpesapayment.ListMPESAPaymentsRequest,
) (*mpesapayment.ListMPESAPaymentsResponse, error) {
	// Authentication
	payload, err := mpesaAPI.authAPI.AuthenticateRequestV2(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case listReq == nil:
		return nil, errs.NilObject("list request")
	}

	// Read from redis list of acc numbers
	allowedAccNo, err := mpesaAPI.RedisDB.SMembers(userAllowedAccSet(payload.ID)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, errs.RedisCmdFailed(err, "smembers")
	}
	// Read from redis list of phone numbers
	allowedPhones, err := mpesaAPI.RedisDB.SMembers(userAllowedPhonesSet(payload.ID)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, errs.RedisCmdFailed(err, "smembers")
	}

	pageSize := listReq.GetPageSize()
	if pageSize <= 0 || pageSize > defaultPageSize {
		pageSize = defaultPageSize
	}

	var paymentID uint

	pageToken := listReq.GetPageToken()
	if pageToken != "" {
		ids, err := mpesaAPI.hasher.DecodeInt64WithError(listReq.GetPageToken())
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "failed to parse page token")
		}
		paymentID = uint(ids[0])
	}

	mpesaMPESAPayments := make([]*Model, 0, pageSize)

	db := mpesaAPI.SQLDB.Limit(int(pageSize)).Order("payment_id DESC")

	if len(allowedAccNo) > 0 {
		db = db.Where("tx_ref_number IN(?)", allowedAccNo)
	}

	if len(allowedPhones) > 0 {
		db = db.Where("msisdn IN(?)", allowedPhones)
	}

	// Apply payment id filter
	if paymentID != 0 {
		db = db.Where("payment_id<?", paymentID)
	}

	// Apply filters
	if listReq.Filter != nil {
		if listReq.Filter.TxDate != "" {
			t, err := getTime(listReq.Filter.TxDate)
			if err != nil {
				return nil, err
			}
			db = db.Where("tx_timestamp BETWEEN ? AND ?", t.Unix(), t.Add(time.Hour*24).Unix())
		}
		if len(listReq.Filter.Msisdns) > 0 {
			if payload.Group == auth.AdminGroup() {
				db = db.Where("msisdn IN(?)", listReq.Filter.Msisdns)
			}
		}
		if len(listReq.Filter.AccountsNumber) > 0 {
			if payload.Group == auth.AdminGroup() {
				db = db.Where("tx_ref_number IN(?)", listReq.Filter.AccountsNumber)
			}
		}
	}

	err = db.Find(&mpesaMPESAPayments).Error
	switch {
	case err == nil:
	default:
		return nil, errs.FailedToFind("ussd channels", err)
	}

	paymentPaymentsPB := make([]*mpesapayment.MPESAPayment, 0, len(mpesaMPESAPayments))

	for _, paymentPaymentDB := range mpesaMPESAPayments {
		paymentPaymenPB, err := GetMpesaPB(paymentPaymentDB)
		if err != nil {
			return nil, err
		}
		paymentPaymentsPB = append(paymentPaymentsPB, paymentPaymenPB)
		paymentID = paymentPaymentDB.PaymentID
	}

	var token string
	if int(pageSize) == len(mpesaMPESAPayments) {
		// Next page token
		token, err = mpesaAPI.hasher.EncodeInt64([]int64{int64(paymentID)})
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "failed to generate next page token")
		}
	}

	return &mpesapayment.ListMPESAPaymentsResponse{
		NextPageToken: token,
		MpesaPayments: paymentPaymentsPB,
	}, nil
}

func getTime(dateStr string) (*time.Time, error) {
	// 2020y 08m 16d 20h 41m 16s
	// "2006-01-02T15:04:05Z07:00"

	timeRFC3339Str := fmt.Sprintf("%sT00:00:00Z", dateStr)

	t, err := time.Parse(time.RFC3339, timeRFC3339Str)
	if err != nil {
		return nil, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "failed to parse date to time")
	}

	return &t, nil
}

func (mpesaAPI *mpesaAPIServer) AddScopes(
	ctx context.Context, addReq *mpesapayment.AddScopesRequest,
) (*empty.Empty, error) {
	// Authentication
	_, err := mpesaAPI.authAPI.AuthorizeGroups(ctx, auth.AdminGroup())
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case addReq == nil:
		return nil, errs.NilObject("AddScopesRequest")
	case addReq.UserId == "":
		return nil, errs.MissingField("user id")
	case addReq.Scopes == nil:
		return nil, errs.NilObject("scopes")
	}

	// Delete the scopes
	mpesaAPI.RedisDB.Del(userAllowedAccSet(addReq.UserId))

	if len(addReq.Scopes.AllowedAccNumber) > 0 {
		// Add the scopes
		err = mpesaAPI.RedisDB.SAdd(userAllowedAccSet(addReq.UserId), addReq.Scopes.AllowedAccNumber).Err()
		if err != nil {
			return nil, errs.RedisCmdFailed(err, "sadd")
		}
	}

	// Delete the scopes
	mpesaAPI.RedisDB.Del(userAllowedPhonesSet(addReq.UserId))

	if len(addReq.Scopes.AllowedPhones) > 0 {
		// Add the scopes
		err = mpesaAPI.RedisDB.SAdd(userAllowedPhonesSet(addReq.UserId), addReq.Scopes.AllowedPhones).Err()
		if err != nil {
			return nil, errs.RedisCmdFailed(err, "sadd")
		}
	}

	return &empty.Empty{}, nil
}

func (mpesaAPI *mpesaAPIServer) GetScopes(
	ctx context.Context, getReq *mpesapayment.GetScopesRequest,
) (*mpesapayment.GetScopesResponse, error) {
	// Authentication
	_, err := mpesaAPI.authAPI.AuthenticateRequestV2(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case getReq == nil:
		return nil, errs.NilObject("list request")
	case getReq.UserId == "":
		return nil, errs.MissingField("user id")
	}

	// Read from redis set of acc numbers
	allowedAccNo, err := mpesaAPI.RedisDB.SMembers(userAllowedAccSet(getReq.UserId)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, errs.RedisCmdFailed(err, "smembers")
	}

	// Read from redis set of phone numbers
	allowedPhones, err := mpesaAPI.RedisDB.SMembers(userAllowedPhonesSet(getReq.UserId)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, errs.RedisCmdFailed(err, "smembers")
	}

	return &mpesapayment.GetScopesResponse{
		Scopes: &mpesapayment.Scopes{
			AllowedAccNumber: allowedAccNo,
			AllowedPhones:    allowedPhones,
		},
	}, nil
}
