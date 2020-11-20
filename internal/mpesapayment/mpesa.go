package mpesapayment

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gidyon/mpesapayments/internal/stk"
	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"github.com/gidyon/services/pkg/auth"
	"github.com/gidyon/services/pkg/utils/encryption"
	"github.com/gidyon/services/pkg/utils/errs"
	"github.com/go-redis/redis"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/speps/go-hashids"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
)

// FailedTxList is redis list for failed mpesa transactions
const (
	FailedTxList            = "mpesa:payments:failedtx:list"
	failedTxListv2          = "mpesa:payments:failedtx:list"
	unprocessedTxList       = "mpesa:payments:failedtx:list:failed"
	pendingConfirmationSet  = "mpesa:payments:pendingtx:set"
	pendingConfirmationList = "mpesa:payments:pendingtx:list"
	publishChannel          = "mpesa:payments:pubsub"
)

type mpesaAPIServer struct {
	mpesapayment.UnimplementedLipaNaMPESAServer
	authAPI             auth.Interface
	hasher              *hashids.HashID
	lastProcessedTxTime time.Time
	*stk.Options
}

// NewAPIServerMPESA creates a singleton instance of mpesa API server
func NewAPIServerMPESA(ctx context.Context, opt *stk.Options) (mpesapayment.LipaNaMPESAServer, error) {
	// Validation
	var err error
	switch {
	case ctx == nil:
		return nil, errs.NilObject("context")
	default:
		err = stk.ValidateOptions(opt)
		if err != nil {
			return nil, err
		}
	}

	// Authentication API
	authAPI, err := auth.NewAPI(opt.JWTSigningKey, "Mpesa Payments", "trusted accounts")
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
	if !mpesaAPI.SQLDB.Migrator().HasTable(MpesaPayments) {
		err = mpesaAPI.SQLDB.Migrator().AutoMigrate(&PaymentMpesa{})
		if err != nil {
			return nil, err
		}
	}

	workerDur := time.Minute * 30
	if opt.WorkerDuration > 0 {
		workerDur = opt.WorkerDuration
	}

	// Start worker for failed transactions
	go mpesaAPI.worker(ctx, workerDur)

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
	_, err := mpesaAPI.authAPI.AuthorizeGroups(ctx, auth.Admins()...)
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
		mpesaAPI.Logger.Warning("duplicate request skipped")
		return &mpesapayment.CreateMPESAPaymentResponse{}, nil
	default:
		return nil, errs.FailedToSave("mpesa payment", err)
	}

	return &mpesapayment.CreateMPESAPaymentResponse{
		PaymentId: fmt.Sprint(mpesaDB.PaymentID),
	}, nil
}

func (mpesaAPI *mpesaAPIServer) GetMPESAPayment(
	ctx context.Context, getReq *mpesapayment.GetMPESAPaymentRequest,
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
		return nil, errs.NilObject("GetMPESAPaymentRequest")
	case getReq.PaymentId == "":
		return nil, errs.MissingField("payment id")
	default:
		paymentID, err = strconv.Atoi(getReq.PaymentId)
		if err != nil {
			return nil, errs.IncorrectVal("payment id")
		}
	}

	mpesaDB := &PaymentMpesa{}

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

	mpesapayments := make([]*PaymentMpesa, 0, pageSize)

	db := mpesaAPI.SQLDB.Limit(int(pageSize)).Order("payment_id DESC")

	if len(allowedAccNo) > 0 {
		db = db.Where("tx_ref_number IN(?)", allowedAccNo)
	}

	if len(allowedPhones) > 0 {
		db = db.Where("msisdn IN(?)", allowedPhones)
	}

	// Apply payment id filter
	if paymentID != 0 {
		db = db.Where("payment_id<=?", paymentID)
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

	err = db.Find(&mpesapayments).Error
	switch {
	case err == nil:
	default:
		return nil, errs.FailedToFind("ussd channels", err)
	}

	paymentPaymentsPB := make([]*mpesapayment.MPESAPayment, 0, len(mpesapayments))

	for _, paymentPaymentDB := range mpesapayments {
		paymentPaymenPB, err := GetMpesaPB(paymentPaymentDB)
		if err != nil {
			return nil, err
		}
		paymentPaymentsPB = append(paymentPaymentsPB, paymentPaymenPB)
		paymentID = paymentPaymentDB.PaymentID
	}

	var token string
	if len(mpesapayments) >= int(pageSize) {
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

func (mpesaAPI *mpesaAPIServer) ProcessMpesaPayment(
	ctx context.Context, processReq *mpesapayment.ProcessMpesaPaymentRequest,
) (*empty.Empty, error) {
	// Authentication
	_, err := mpesaAPI.authAPI.AuthorizeGroups(ctx, auth.Admins()...)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case processReq == nil:
		return nil, errs.NilObject("process request")
	case processReq.PaymentId == "":
		return nil, errs.MissingField("payment id")
	}

	// Get mpesa payment
	mpesaPayment, err := mpesaAPI.GetMPESAPayment(ctx, &mpesapayment.GetMPESAPaymentRequest{
		PaymentId: processReq.PaymentId,
	})
	if err != nil {
		return nil, err
	}

	// Check if not already processed process
	if mpesaPayment.Processed == false {
		err = mpesaAPI.SQLDB.Model(&PaymentMpesa{}).Where("payment_id=?", processReq.PaymentId).Update("processed", true).Error
		switch {
		case err == nil:
		default:
			return nil, errs.FailedToUpdate("mpesa payment", err)
		}
	}

	return &empty.Empty{}, nil
}

func (mpesaAPI *mpesaAPIServer) PublishMpesaPayment(
	ctx context.Context, pubReq *mpesapayment.PublishMpesaPaymentRequest,
) (*empty.Empty, error) {
	// Authentication
	_, err := mpesaAPI.authAPI.AuthorizeGroups(ctx, auth.Admins()...)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case pubReq == nil:
		return nil, errs.NilObject("publish request")
	case pubReq.PaymentId == "":
		return nil, errs.MissingField("payment id")
	case pubReq.InitiatorId == "":
		return nil, errs.MissingField("initiator id")
	}

	// Get mpesa payment
	mpesaPayment, err := mpesaAPI.GetMPESAPayment(ctx, &mpesapayment.GetMPESAPaymentRequest{
		PaymentId: pubReq.PaymentId,
	})
	if err != nil {
		return nil, err
	}

	publishPayload := fmt.Sprintf("PAYMENT:%s:%s", pubReq.PaymentId, pubReq.InitiatorId)

	// Publish based on state
	switch pubReq.ProcessedState {
	case mpesapayment.ProcessedState_PROCESS_STATE_UNSPECIFIED:
		// Publish only if the processed state is false
		if !mpesaPayment.Processed {
			err = mpesaAPI.RedisDB.Publish(publishChannel, publishPayload).Err()
			if err != nil {
				return nil, errs.RedisCmdFailed(err, "PUBSUB")
			}
		}
	case mpesapayment.ProcessedState_ANY:
		err = mpesaAPI.RedisDB.Publish(publishChannel, publishPayload).Err()
		if err != nil {
			return nil, errs.RedisCmdFailed(err, "PUBSUB")
		}
	case mpesapayment.ProcessedState_UNPROCESSED_ONLY:
		// Publish only if the processed state is false
		if !mpesaPayment.Processed {
			err = mpesaAPI.RedisDB.Publish(publishChannel, publishPayload).Err()
			if err != nil {
				return nil, errs.RedisCmdFailed(err, "PUBSUB")
			}
		}
	case mpesapayment.ProcessedState_PROCESSED_ONLY:
		// Publish only if the processed state is true
		if mpesaPayment.Processed {
			err = mpesaAPI.RedisDB.Publish(publishChannel, publishPayload).Err()
			if err != nil {
				return nil, errs.RedisCmdFailed(err, "PUBSUB")
			}
		}
	}

	return &empty.Empty{}, nil
}

func (mpesaAPI *mpesaAPIServer) PublishAllMpesaPayment(
	ctx context.Context, pubReq *mpesapayment.PublishAllMpesaPaymentRequest,
) (*empty.Empty, error) {
	// Authentication
	_, err := mpesaAPI.authAPI.AuthorizeGroups(ctx, auth.Admins()...)
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
		startDate = mpesaAPI.lastProcessedTxTime
	}

	for currDate := startDate; currDate.Unix() >= time.Now().Unix(); currDate = currDate.Add(24 * time.Hour) {
		var (
			nextPageToken  string
			pageSize       int32 = 1000
			shouldContinue bool
		)

		for shouldContinue {
			// List transactions
			listRes, err := mpesaAPI.ListMPESAPayments(ctx, &mpesapayment.ListMPESAPaymentsRequest{
				PageToken: nextPageToken,
				PageSize:  pageSize,
				Filter: &mpesapayment.ListMPESAPaymentsFilter{
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
			for _, mpesaPB := range listRes.MpesaPayments {
				err := mpesaAPI.RedisDB.Publish(publishChannel, mpesaPB.PaymentId).Err()
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return &empty.Empty{}, nil
}
