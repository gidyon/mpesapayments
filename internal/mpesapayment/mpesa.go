package mpesapayment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gidyon/micro/pkg/grpc/auth"
	"github.com/gidyon/micro/utils/errs"
	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	redis "github.com/go-redis/redis/v8"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/speps/go-hashids"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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

type mpesaAPIServer struct {
	mpesapayment.UnimplementedLipaNaMPESAServer
	lastProcessedTxTime time.Time
	*Options
	insertChan    chan *PaymentMpesa
	insertTimeOut time.Duration
	ctxAdmin      context.Context
}

// Options contains options for starting mpesa service
type Options struct {
	PublishChannel    string
	SQLDB             *gorm.DB
	RedisDB           *redis.Client
	Logger            grpclog.LoggerV2
	AuthAPI           auth.API
	PaginationHasher  *hashids.HashID
	DisablePublishing bool
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

	mpesaAPI := &mpesaAPIServer{
		Options:       opt,
		insertChan:    make(chan *PaymentMpesa, bulkInsertSize),
		insertTimeOut: 5 * time.Second,
		ctxAdmin:      ctxAdmin,
	}

	mpesaAPI.Logger.Infof("Publishing to mpesa consumers on channel: %v", opt.PublishChannel)

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
	case payment.TxId == "":
		err = errs.MissingField("transaction id")
	case payment.TxTimestamp == 0:
		err = errs.MissingField("transaction time")
	}
	return err
}

func (mpesaAPI *mpesaAPIServer) CreateMPESAPayment(
	ctx context.Context, createReq *mpesapayment.CreateMPESAPaymentRequest,
) (*mpesapayment.CreateMPESAPaymentResponse, error) {
	// Authentication
	_, err := mpesaAPI.AuthAPI.AuthorizeGroups(ctx, auth.Admins()...)
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
		mpesaAPI.insertChan <- mpesaDB
	}()

	return &mpesapayment.CreateMPESAPaymentResponse{
		PaymentId: fmt.Sprint(createReq.MpesaPayment.TxId),
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
	var paymentID int
	switch {
	case getReq == nil:
		return nil, errs.NilObject("GetMPESAPaymentRequest")
	case getReq.PaymentId == "":
		return nil, errs.MissingField("payment id")
	default:
	}

	mpesaDB := &PaymentMpesa{}

	err = mpesaAPI.SQLDB.First(mpesaDB, "payment_id=? OR tx_id=?", paymentID, paymentID).Error
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
	payload, err := mpesaAPI.AuthAPI.AuthenticateRequestV2(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case listReq == nil:
		return nil, errs.NilObject("list request")
	}

	var allowedAccNo, allowedPhones []string

	if !mpesaAPI.DisablePublishing {
		// Read from redis list of acc numbers
		allowedAccNo, err = mpesaAPI.RedisDB.SMembers(ctx, userAllowedAccSet(payload.ID)).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, errs.RedisCmdFailed(err, "smembers")
		}
		// Read from redis list of phone numbers
		allowedPhones, err = mpesaAPI.RedisDB.SMembers(ctx, userAllowedPhonesSet(payload.ID)).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, errs.RedisCmdFailed(err, "smembers")
		}
	}

	pageSize := listReq.GetPageSize()
	if pageSize <= 0 || pageSize > defaultPageSize {
		pageSize = defaultPageSize
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
			db = db.Where("msisdn IN(?)", listReq.Filter.Msisdns)
		}
		if len(listReq.Filter.AccountsNumber) > 0 {
			db = db.Where("tx_ref_number IN(?)", listReq.Filter.AccountsNumber)
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
		token, err = mpesaAPI.PaginationHasher.EncodeInt64([]int64{int64(paymentID)})
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
	if mpesaAPI.DisablePublishing {
		return nil, errs.WrapMessage(codes.PermissionDenied, "not allowed to add scopes")
	}

	// Authentication
	_, err := mpesaAPI.AuthAPI.AuthorizeGroups(ctx, auth.Admins()...)
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
	mpesaAPI.RedisDB.Del(ctx, userAllowedAccSet(addReq.UserId))

	if len(addReq.Scopes.AllowedAccNumber) > 0 {
		// Add the scopes
		err = mpesaAPI.RedisDB.SAdd(ctx, userAllowedAccSet(addReq.UserId), addReq.Scopes.AllowedAccNumber).Err()
		if err != nil {
			return nil, errs.RedisCmdFailed(err, "sadd")
		}
	}

	// Delete the scopes
	mpesaAPI.RedisDB.Del(ctx, userAllowedPhonesSet(addReq.UserId))

	if len(addReq.Scopes.AllowedPhones) > 0 {
		// Add the scopes
		err = mpesaAPI.RedisDB.SAdd(ctx, userAllowedPhonesSet(addReq.UserId), addReq.Scopes.AllowedPhones).Err()
		if err != nil {
			return nil, errs.RedisCmdFailed(err, "sadd")
		}
	}

	return &empty.Empty{}, nil
}

func (mpesaAPI *mpesaAPIServer) GetScopes(
	ctx context.Context, getReq *mpesapayment.GetScopesRequest,
) (*mpesapayment.GetScopesResponse, error) {
	if mpesaAPI.DisablePublishing {
		return nil, errs.WrapMessage(codes.PermissionDenied, "not allowed to get scopes")
	}

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
	allowedAccNo, err := mpesaAPI.RedisDB.SMembers(ctx, userAllowedAccSet(getReq.UserId)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, errs.RedisCmdFailed(err, "smembers")
	}

	// Read from redis set of phone numbers
	allowedPhones, err := mpesaAPI.RedisDB.SMembers(ctx, userAllowedPhonesSet(getReq.UserId)).Result()
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
	_, err := mpesaAPI.AuthAPI.AuthorizeGroups(ctx, auth.Admins()...)
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
	switch {
	case err == nil:
	case status.Code(err) == codes.NotFound:
		// Update mpesa processed state when it comes
		go func(paymentID string) {
			mpesaAPI.Logger.Infoln("started goroutine to update mpesa payment once its received")

			updateFn := func() int64 {
				return mpesaAPI.SQLDB.Model(&PaymentMpesa{}).Where("tx_id=?", paymentID).Update("processed", true).RowsAffected
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
					mpesaAPI.Logger.Infoln("updated processed state of mpesa payment to true")
					if !timer.Stop() {
						<-timer.C
					}
					break loop
				}
				count++
			}
		}(processReq.PaymentId)

		return &empty.Empty{}, nil
	default:
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
	if mpesaAPI.DisablePublishing {
		return nil, errs.WrapMessage(codes.PermissionDenied, "not allowed to publish payment")
	}

	// Authentication
	_, err := mpesaAPI.AuthAPI.AuthorizeGroups(ctx, auth.Admins()...)
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

	err = mpesaAPI.RedisDB.Publish(ctx, mpesaAPI.PublishChannel, publishPayload).Err()
	if err != nil {
		return nil, errs.RedisCmdFailed(err, "PUBSUB")
	}

	// Publish based on state
	switch pubReq.ProcessedState {
	case mpesapayment.ProcessedState_PROCESS_STATE_UNSPECIFIED:
		err = mpesaAPI.RedisDB.Publish(ctx, publishChannel, publishPayload).Err()
		if err != nil {
			return nil, errs.RedisCmdFailed(err, "PUBSUB")
		}
	case mpesapayment.ProcessedState_NOT_PROCESSED:
		// Publish only if the processed state is false
		if !mpesaPayment.Processed {
			err = mpesaAPI.RedisDB.Publish(ctx, publishChannel, publishPayload).Err()
			if err != nil {
				return nil, errs.RedisCmdFailed(err, "PUBSUB")
			}
		}
	case mpesapayment.ProcessedState_PROCESSED:
		// Publish only if the processed state is true
		if mpesaPayment.Processed {
			err = mpesaAPI.RedisDB.Publish(ctx, publishChannel, publishPayload).Err()
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
	if mpesaAPI.DisablePublishing {
		return nil, errs.WrapMessage(codes.PermissionDenied, "not allowed to publish payment")
	}

	// Authentication
	_, err := mpesaAPI.AuthAPI.AuthorizeGroups(ctx, auth.Admins()...)
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
				err := mpesaAPI.RedisDB.Publish(ctx, mpesaAPI.PublishChannel, mpesaPB.PaymentId).Err()
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return &empty.Empty{}, nil
}

type transactionsSummary struct {
}

func (mpesaAPI *mpesaAPIServer) GetTransactionsCount(
	ctx context.Context, getReq *mpesapayment.GetTransactionsCountRequest,
) (*mpesapayment.TransactionsSummary, error) {
	// Authentication
	_, err := mpesaAPI.AuthAPI.AuthenticateRequestV2(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case getReq == nil:
		return nil, errs.NilObject("get transactions count")
	case getReq.Amount == 0:
		return nil, errs.NilObject("amount")
	default:
		if getReq.StartTimeSeconds > 0 || getReq.EndTimeSeconds > 0 {
			if getReq.StartTimeSeconds > getReq.EndTimeSeconds {
				return nil, errs.WrapMessage(codes.InvalidArgument, "start time cannot be greater than end time")
			}
		}
	}

	db := mpesaAPI.SQLDB.Table(MpesaPayments).Where("tx_amount=?", getReq.Amount)

	// Apply filters
	if len(getReq.AccountsNumber) > 0 {
		db = db.Where("tx_ref_number IN ?", getReq.AccountsNumber)
	}
	if len(getReq.Msisdns) > 0 {
		db = db.Where("msisdn IN ?", getReq.Msisdns)
	}

	// Apply date filters
	if getReq.StartTimeSeconds > 0 || getReq.EndTimeSeconds > 0 {
		db = db.Where("tx_timestamp BETWEEN ? AND ?", getReq.StartTimeSeconds, getReq.EndTimeSeconds)
	}

	var transactions int64

	// Count results
	err = db.Count(&transactions).Error
	if err != nil {
		return nil, errs.FailedToFind("transactions", err)
	}

	return &mpesapayment.TransactionsSummary{
		TotalAmount:       float32(transactions) * getReq.Amount,
		TransactionsCount: int32(transactions),
	}, nil
}
