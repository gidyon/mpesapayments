package mpesapayment

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/gidyon/micro/v2/pkg/middleware/grpc/auth"
	"github.com/gidyon/micro/v2/utils/errs"
	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	redis "github.com/go-redis/redis/v8"
	"github.com/speps/go-hashids"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
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
	bulkInsertSize          = 1000
)

type incomingPayment struct {
	payment *PaymentMpesa
	publish bool
}

type mpesaAPIServer struct {
	mpesapayment.UnimplementedLipaNaMPESAServer
	lastProcessedTxTime time.Time
	*Options
	insertChan    chan *incomingPayment
	insertTimeOut time.Duration
	ctxAdmin      context.Context
}

// Options contains options for starting mpesa service
type Options struct {
	PublishChannel   string
	RedisKeyPrefix   string
	SQLDB            *gorm.DB
	RedisDB          *redis.Client
	Logger           grpclog.LoggerV2
	AuthAPI          auth.API
	PaginationHasher *hashids.HashID
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
		err = errs.NilObject("pagination PaginationHasher")
	case opt.RedisKeyPrefix == "":
		err = errs.MissingField("keys prefix")
	}
	return err
}

// NewAPIServerMPESA creates a singleton instance of mpesa API server
func NewAPIServerMPESA(ctx context.Context, opt *Options) (mpesapayment.LipaNaMPESAServer, error) {
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
	}

	// Update publish channel
	if opt.PublishChannel == "" {
		opt.PublishChannel = publishChannel
	}

	// Generate jwt for API
	token, err := opt.AuthAPI.GenToken(
		ctx, &auth.Payload{Group: auth.DefaultAdminGroup()}, time.Now().Add(10*365*24*time.Hour))
	if err != nil {
		return nil, fmt.Errorf("failed to generate auth token: %v", err)
	}

	md := metadata.Pairs(auth.Header(), fmt.Sprintf("%s %s", auth.Scheme(), token))

	ctxAdmin := metadata.NewIncomingContext(ctx, md)

	// Authenticate the jwt context
	ctxAdmin, err = opt.AuthAPI.AuthorizeFunc(ctxAdmin)
	if err != nil {
		return nil, err
	}

	mpesaAPI := &mpesaAPIServer{
		Options:       opt,
		insertChan:    make(chan *incomingPayment, bulkInsertSize),
		insertTimeOut: 5 * time.Second,
		ctxAdmin:      ctxAdmin,
	}

	mpesaAPI.Logger.Infof("Publishing to mpesa consumers on channel: %v", mpesaAPI.AddPrefix(opt.PublishChannel))

	// Auto migration
	if !mpesaAPI.SQLDB.Migrator().HasTable(MpesaPayments) {
		err = mpesaAPI.SQLDB.Migrator().AutoMigrate(&PaymentMpesa{})
		if err != nil {
			return nil, err
		}
	}

	// workerDur := time.Minute * 30
	// if opt.WorkerDuration > 0 {
	// 	workerDur = opt.WorkerDuration
	// }

	// // Start worker for failed transactions
	// go mpesaAPI.worker(ctx, workerDur)

	// Insert worker
	go mpesaAPI.insertWorker(ctx)

	return mpesaAPI, nil
}

// ValidateMPESAPayment validates MPESA transaction
func ValidateMPESAPayment(payment *mpesapayment.MPESAPayment) error {
	var err error
	switch {
	case payment == nil:
		err = errs.NilObject("mpesa payment")
	case payment.BusinessShortCode == 0 && payment.TransactionType == "PAY_BILL":
		err = errs.MissingField("business short code")
	case payment.RefNumber == "" && payment.TransactionType == "PAY_BILL":
		err = errs.MissingField("account number")
	case payment.Msisdn == "" || payment.Msisdn == "0":
		err = errs.MissingField("msisdn")
	case payment.TransactionType == "":
		err = errs.MissingField("transaction type")
	case int(payment.Amount) == 0:
		err = errs.MissingField("transaction amount")
	case payment.TransactionId == "":
		err = errs.MissingField("transaction id")
	case payment.TransactionTimestamp == 0:
		err = errs.MissingField("transaction time")
	}
	return err
}

// AddPrefix adds a prefix to redis key
func AddPrefix(key, prefix string) string {
	return fmt.Sprintf("%s:%s", prefix, key)
}

// AddPrefix adds a prefix to redis key
func (mpesaAPI *mpesaAPIServer) AddPrefix(key string) string {
	return AddPrefix(key, mpesaAPI.RedisKeyPrefix)
}

func (mpesaAPI *mpesaAPIServer) CreateMPESAPayment(
	ctx context.Context, createReq *mpesapayment.CreateMPESAPaymentRequest,
) (*mpesapayment.CreateMPESAPaymentResponse, error) {
	// Authentication
	_, err := mpesaAPI.AuthAPI.AuthorizeAdmin(ctx)
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

	// Save payload via channel
	go func() {
		mpesaAPI.insertChan <- &incomingPayment{
			payment: mpesaDB,
			publish: createReq.Publish,
		}
	}()

	return &mpesapayment.CreateMPESAPaymentResponse{
		PaymentId: fmt.Sprint(createReq.MpesaPayment.TransactionId),
	}, nil
}

func (mpesaAPI *mpesaAPIServer) GetMPESAPayment(
	ctx context.Context, getReq *mpesapayment.GetMPESAPaymentRequest,
) (*mpesapayment.MPESAPayment, error) {
	// Authentication
	err := mpesaAPI.AuthAPI.AuthenticateRequest(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case getReq == nil:
		return nil, errs.NilObject("GetMPESAPaymentRequest")
	case getReq.PaymentId == "":
		return nil, errs.MissingField("payment id")
	default:
	}

	mpesaDB := &PaymentMpesa{}

	if paymentID, err1 := strconv.Atoi(getReq.PaymentId); err1 == nil && paymentID != 0 {
		err = mpesaAPI.SQLDB.First(mpesaDB, paymentID).Error
	} else {
		err = mpesaAPI.SQLDB.First(mpesaDB, "transaction_id=?", getReq.PaymentId).Error
	}

	switch {
	case err == nil:
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, errs.DoesNotExist("mpesa payment", getReq.PaymentId)
	default:
		return nil, errs.FailedToFind("mpesa payment", err)
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
	payload, err := mpesaAPI.AuthAPI.AuthenticateRequestV2(ctx)
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

	var allowedAccNo, allowedPhones []string

	// Read from redis list of acc numbers
	allowedAccNo, err = mpesaAPI.RedisDB.SMembers(
		ctx, mpesaAPI.AddPrefix(userAllowedAccSet(payload.ID)),
	).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, errs.RedisCmdFailed(err, "smembers")
	}
	// Read from redis list of phone numbers
	allowedPhones, err = mpesaAPI.RedisDB.SMembers(
		ctx, mpesaAPI.AddPrefix(userAllowedPhonesSet(payload.ID)),
	).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, errs.RedisCmdFailed(err, "smembers")
	}

	pageSize := listReq.GetPageSize()
	if pageSize <= 0 || pageSize > defaultPageSize {
		if mpesaAPI.AuthAPI.IsAdmin(payload.Group) {
			pageSize = defaultPageSize
		}
	}

	var paymentID uint

	pageToken := listReq.GetPageToken()
	if pageToken != "" {
		ids, err := mpesaAPI.PaginationHasher.DecodeInt64WithError(listReq.GetPageToken())
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "failed to parse page token")
		}
		paymentID = uint(ids[0])
	}

	mpesapayments := make([]*PaymentMpesa, 0, pageSize+1)

	// Add filter from request filters
	allowedAccNo = append(allowedAccNo, listReq.GetFilter().GetAccountsNumber()...)
	allowedPhones = append(allowedPhones, listReq.GetFilter().GetMsisdns()...)

	db := mpesaAPI.SQLDB.Limit(int(pageSize + 1)).Order("payment_id DESC")

	// Apply filters
	if len(allowedAccNo) > 0 {
		db = db.Where("reference_number IN(?)", allowedAccNo)
	}

	if len(allowedPhones) > 0 {
		db = db.Where("msisdn IN(?)", allowedPhones)
	}

	if int(listReq.GetFilter().GetAmount()) > 0 {
		db = db.Where("amount BETWEEN ? AND ?", listReq.Filter.Amount-0.5, listReq.Filter.Amount)
	}

	// Apply payment id filter
	if paymentID != 0 {
		db = db.Where("payment_id<?", paymentID)
	}

	// Apply filters
	if listReq.Filter != nil {
		startTimestamp := listReq.Filter.GetStartTimestamp()
		endTimestamp := listReq.Filter.GetEndTimestamp()

		// Timestamp filter
		if endTimestamp > startTimestamp {
			db = db.Where("transaction_timestamp BETWEEN ? AND ?", startTimestamp, endTimestamp)
		} else {
			// Date filter
			if listReq.Filter.TxDate != "" {
				t, err := getTime(listReq.Filter.TxDate)
				if err != nil {
					return nil, err
				}
				db = db.Where("transaction_timestamp BETWEEN ? AND ?", t.Unix(), t.Add(time.Hour*24).Unix())
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

	err = db.Find(&mpesapayments).Error
	switch {
	case err == nil:
	default:
		return nil, errs.FailedToFind("ussd channels", err)
	}

	paymentsPB := make([]*mpesapayment.MPESAPayment, 0, len(mpesapayments))

	for i, paymentDB := range mpesapayments {
		paymentPaymenPB, err := GetMpesaPB(paymentDB)
		if err != nil {
			return nil, err
		}

		// Ignore the last element
		if i == int(pageSize) {
			break
		}

		paymentsPB = append(paymentsPB, paymentPaymenPB)
		paymentID = paymentDB.PaymentID
	}

	var token string
	if len(mpesapayments) > int(pageSize) {
		// Next page token
		token, err = mpesaAPI.PaginationHasher.EncodeInt64([]int64{int64(paymentID)})
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "failed to generate next page token")
		}
	}

	return &mpesapayment.ListMPESAPaymentsResponse{
		NextPageToken: token,
		MpesaPayments: paymentsPB,
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
) (*emptypb.Empty, error) {
	// Authentication
	_, err := mpesaAPI.AuthAPI.AuthorizeAdmin(ctx)
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
	mpesaAPI.RedisDB.Del(ctx, mpesaAPI.AddPrefix(userAllowedAccSet(addReq.UserId)))

	if len(addReq.Scopes.AllowedAccNumber) > 0 {
		// Add the scopes
		err = mpesaAPI.RedisDB.SAdd(
			ctx, mpesaAPI.AddPrefix(userAllowedAccSet(addReq.UserId)), addReq.Scopes.AllowedAccNumber,
		).Err()
		if err != nil {
			return nil, errs.RedisCmdFailed(err, "sadd")
		}
	}

	// Delete the scopes
	mpesaAPI.RedisDB.Del(ctx, mpesaAPI.AddPrefix(userAllowedPhonesSet(addReq.UserId)))

	if len(addReq.Scopes.AllowedPhones) > 0 {
		// Add the scopes
		err = mpesaAPI.RedisDB.SAdd(
			ctx, mpesaAPI.AddPrefix(userAllowedPhonesSet(addReq.UserId)), addReq.Scopes.AllowedPhones,
		).Err()
		if err != nil {
			return nil, errs.RedisCmdFailed(err, "sadd")
		}
	}

	return &emptypb.Empty{}, nil
}

func (mpesaAPI *mpesaAPIServer) GetScopes(
	ctx context.Context, getReq *mpesapayment.GetScopesRequest,
) (*mpesapayment.GetScopesResponse, error) {
	// Authentication
	_, err := mpesaAPI.AuthAPI.AuthenticateRequestV2(ctx)
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
	allowedAccNo, err := mpesaAPI.RedisDB.SMembers(
		ctx, mpesaAPI.AddPrefix(userAllowedAccSet(getReq.UserId)),
	).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, errs.RedisCmdFailed(err, "smembers")
	}

	// Read from redis set of phone numbers
	allowedPhones, err := mpesaAPI.RedisDB.SMembers(
		ctx, mpesaAPI.AddPrefix(userAllowedPhonesSet(getReq.UserId)),
	).Result()
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
) (*emptypb.Empty, error) {
	// Authentication
	_, err := mpesaAPI.AuthAPI.AuthorizeAdmin(ctx)
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

	var count int64

	// Check if not already processed process
	if _, err := strconv.Atoi(processReq.PaymentId); err == nil {
		db := mpesaAPI.SQLDB.Model(&PaymentMpesa{}).Unscoped().Where("payment_id=?", processReq.PaymentId).Update("processed", processReq.State)
		err = db.Error
		count = db.RowsAffected
	} else {
		db := mpesaAPI.SQLDB.Model(&PaymentMpesa{}).Unscoped().Where("transaction_id=?", processReq.PaymentId).Update("processed", processReq.State)
		err = db.Error
		count = db.RowsAffected
	}
	if err != nil {
		return nil, errs.FailedToUpdate("mpesa payment", err)
	}

	// Update if not processed
	if count == 0 {
		if processReq.Retry {
			// Update mpesa processed state after some interval
			go func(paymentID string) {
				mpesaAPI.Logger.Infoln("started goroutine to update mpesa payment once its received")

				updateFn := func() int64 {
					return mpesaAPI.SQLDB.Model(&PaymentMpesa{}).Where("transaction_id=?", paymentID).Update("processed", processReq.State).RowsAffected
				}

				timer := time.NewTimer(5 * time.Second)
				count := 0
			loop:
				for range timer.C {
					if count == 5 {
						break loop
					}
					res := updateFn()
					if res > 0 {
						if !timer.Stop() {
							<-timer.C
						}
						break loop
					}
					count++
				}
			}(processReq.PaymentId)
		}
	}

	return &emptypb.Empty{}, nil
}

func (mpesaAPI *mpesaAPIServer) PublishMpesaPayment(
	ctx context.Context, pubReq *mpesapayment.PublishMpesaPaymentRequest,
) (*emptypb.Empty, error) {
	// Authentication
	_, err := mpesaAPI.AuthAPI.AuthorizeAdmin(ctx)
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
		err = mpesaAPI.RedisDB.Publish(
			ctx, mpesaAPI.AddPrefix(mpesaAPI.PublishChannel), publishPayload,
		).Err()
		if err != nil {
			return nil, errs.RedisCmdFailed(err, "PUBSUB")
		}
	case mpesapayment.ProcessedState_NOT_PROCESSED:
		// Publish only if the processed state is false
		if !mpesaPayment.Processed {
			err = mpesaAPI.RedisDB.Publish(
				ctx, mpesaAPI.AddPrefix(mpesaAPI.PublishChannel), publishPayload,
			).Err()
			if err != nil {
				return nil, errs.RedisCmdFailed(err, "PUBSUB")
			}
		}
	case mpesapayment.ProcessedState_PROCESSED:
		// Publish only if the processed state is true
		if mpesaPayment.Processed {
			err = mpesaAPI.RedisDB.Publish(
				ctx, mpesaAPI.AddPrefix(mpesaAPI.PublishChannel), publishPayload,
			).Err()
			if err != nil {
				return nil, errs.RedisCmdFailed(err, "PUBSUB")
			}
		}
	}

	return &emptypb.Empty{}, nil
}

func (mpesaAPI *mpesaAPIServer) PublishAllMpesaPayment(
	ctx context.Context, pubReq *mpesapayment.PublishAllMpesaPaymentRequest,
) (*emptypb.Empty, error) {
	// Authentication
	_, err := mpesaAPI.AuthAPI.AuthorizeAdmin(ctx)
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
		nextPageToken string
		pageSize      int32 = 1000
		next          bool  = true
	)

	for next {
		// List transactions
		listRes, err := mpesaAPI.ListMPESAPayments(ctx, &mpesapayment.ListMPESAPaymentsRequest{
			PageToken: nextPageToken,
			PageSize:  pageSize,
			Filter: &mpesapayment.ListMPESAPaymentsFilter{
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
		nextPageToken = listRes.NextPageToken

		// Pipeline
		pipeliner := mpesaAPI.RedisDB.Pipeline()

		// Publish the mpesa transactions to listeners
		for _, mpesaPB := range listRes.MpesaPayments {
			err := pipeliner.Publish(
				ctx, mpesaAPI.AddPrefix(mpesaAPI.PublishChannel), mpesaPB.PaymentId,
			).Err()
			if err != nil {
				return nil, err
			}
		}

		// Execute
		_, err = pipeliner.Exec(ctx)
		if err != nil {
			return nil, errs.RedisCmdFailed(err, "exec")
		}
	}

	return &emptypb.Empty{}, nil
}

type transactionsSummary struct {
}

func inGroup(group string, groups []string) bool {
	for _, grp := range groups {
		if grp == group {
			return true
		}
	}
	return false
}

func (mpesaAPI *mpesaAPIServer) GetTransactionsCount(
	ctx context.Context, getReq *mpesapayment.GetTransactionsCountRequest,
) (*mpesapayment.TransactionsSummary, error) {
	// Authentication
	payload, err := mpesaAPI.AuthAPI.AuthenticateRequestV2(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case getReq == nil:
		return nil, errs.NilObject("get transactions count")
	case getReq.Amount == 0 && mpesaAPI.AuthAPI.IsAdmin(payload.Group):
		return nil, errs.NilObject("amount")
	default:
		if getReq.StartTimeSeconds > 0 || getReq.EndTimeSeconds > 0 {
			if getReq.StartTimeSeconds > getReq.EndTimeSeconds {
				return nil, errs.WrapMessage(codes.InvalidArgument, "start time cannot be greater than end time")
			}
		}
	}

	db := mpesaAPI.SQLDB.Model(&PaymentMpesa{})

	// Apply filters
	if getReq.Amount > 0 {
		db = db.Where("amount BETWEEN ? AND ?", getReq.Amount-0.5, getReq.Amount)
	}
	if len(getReq.AccountsNumber) > 0 {
		db = db.Where("reference_number IN (?)", getReq.AccountsNumber)
	}
	if len(getReq.Msisdns) > 0 {
		db = db.Where("msisdn IN (?)", getReq.Msisdns)
	}

	// Apply date filters
	if getReq.StartTimeSeconds > 0 || getReq.EndTimeSeconds > 0 {
		db = db.Where("transaction_timestamp BETWEEN ? AND ?", getReq.StartTimeSeconds, getReq.EndTimeSeconds)
	}

	var transactions int64

	// Count of transactions
	err = db.Count(&transactions).Error
	if err != nil {
		return nil, errs.FailedToFind("transactions", err)
	}

	var totalAmount float32

	// Get total amount
	err = db.Model(&PaymentMpesa{}).Select("sum(amount) as total").Row().Scan(&totalAmount)
	if err != nil {
		return nil, errs.FailedToFind("total", err)
	}

	return &mpesapayment.TransactionsSummary{
		TotalAmount:       totalAmount,
		TransactionsCount: int32(transactions),
	}, nil
}

type total struct {
	total float32
}

func (mpesaAPI *mpesaAPIServer) GetRandomTransaction(
	ctx context.Context, getReq *mpesapayment.GetRandomTransactionRequest,
) (*mpesapayment.MPESAPayment, error) {
	// Authentication
	_, err := mpesaAPI.AuthAPI.AuthorizeAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case getReq == nil:
		return nil, errs.NilObject("get request")
	}

	// Get Max and Min Ids
	var (
		pageToken  string
		next       = true
		lastResult = true
		min, max   int
	)

	for next {
		listRes, err := mpesaAPI.ListMPESAPayments(ctx, &mpesapayment.ListMPESAPaymentsRequest{
			PageToken: pageToken,
			PageSize:  defaultPageSize,
			Filter: &mpesapayment.ListMPESAPaymentsFilter{
				AccountsNumber: getReq.GetAccountsNumber(),
				Amount:         getReq.GetAmount(),
				StartTimestamp: getReq.GetStartTimeSeconds(),
				EndTimestamp:   getReq.GetEndTimeSeconds(),
			},
		})
		if err != nil {
			return nil, err
		}

		itemsPresent := len(listRes.MpesaPayments) > 0
		pageToken = listRes.NextPageToken

		if listRes.NextPageToken == "" {
			next = false
		}

		// First result
		if listRes.NextPageToken == "" && itemsPresent {
			min, err = strconv.Atoi(listRes.MpesaPayments[len(listRes.MpesaPayments)-1].PaymentId)
			if err != nil {
				return nil, errs.ConvertingType(err, "string", "int")
			}
		}

		// Last result
		if lastResult && itemsPresent {
			max, err = strconv.Atoi(listRes.MpesaPayments[0].PaymentId)
			if err != nil {
				return nil, errs.ConvertingType(err, "string", "int")
			}
			lastResult = false
		}
	}

	rand.Seed(time.Now().UnixNano())

	if max <= min {
		return nil, errs.WrapMessage(codes.FailedPrecondition, "no mpesa payments found")
	}

	// Random winner using RNG
	winnerID := rand.Intn(max-min) + min

	if winnerID == 0 {
		winnerID = max
	}

	return mpesaAPI.GetMPESAPayment(ctx, &mpesapayment.GetMPESAPaymentRequest{
		PaymentId: fmt.Sprint(winnerID),
	})
}
