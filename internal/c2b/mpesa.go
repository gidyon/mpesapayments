package c2b

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/gidyon/micro/v2/pkg/middleware/grpc/auth"
	"github.com/gidyon/micro/v2/utils/errs"
	"github.com/gidyon/mpesapayments/pkg/api/c2b"
	redis "github.com/go-redis/redis/v8"
	"github.com/speps/go-hashids"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
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

type c2bAPIServer struct {
	c2b.UnimplementedLipaNaMPESAServer
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
		err = errs.MissingField("redis keys prefix")
	}
	return err
}

// NewAPIServerMPESA creates a singleton instance of mpesa API server
func NewAPIServerMPESA(ctx context.Context, opt *Options) (c2b.LipaNaMPESAServer, error) {
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

	c2bAPI := &c2bAPIServer{
		Options:       opt,
		insertChan:    make(chan *incomingPayment, bulkInsertSize),
		insertTimeOut: 5 * time.Second,
		ctxAdmin:      ctxAdmin,
	}

	c2bAPI.Logger.Infof("Publishing to mpesa consumers on channel: %v", c2bAPI.AddPrefix(opt.PublishChannel))

	// Auto migrations
	if !c2bAPI.SQLDB.Migrator().HasTable(&PaymentMpesa{}) {
		err = c2bAPI.SQLDB.Migrator().AutoMigrate(&PaymentMpesa{})
		if err != nil {
			err = c2bAPI.SQLDB.Migrator().AutoMigrate(&PaymentMpesa{})
		}
	}

	if !c2bAPI.SQLDB.Migrator().HasTable(&Stat{}) {
		err = c2bAPI.SQLDB.Migrator().AutoMigrate(&Stat{})
		if err != nil {
			return nil, fmt.Errorf("failed to automigrate %s table: %v", (&Stat{}).TableName(), err)
		}
	}

	if !c2bAPI.SQLDB.Migrator().HasTable(&Scopes{}) {
		err = c2bAPI.SQLDB.Migrator().AutoMigrate(&Scopes{})
		if err != nil {
			return nil, fmt.Errorf("failed to automigrate %s table: %v", (&Scopes{}).TableName(), err)
		}
	}

	if !c2bAPI.SQLDB.Migrator().HasTable(&QueueBulk{}) {
		err = c2bAPI.SQLDB.Migrator().AutoMigrate(&QueueBulk{})
		if err != nil {
			return nil, fmt.Errorf("failed to automigrate %s table: %v", (&QueueBulk{}).TableName(), err)
		}
	}

	if !c2bAPI.SQLDB.Migrator().HasTable(&BlastReport{}) {
		err = c2bAPI.SQLDB.Migrator().AutoMigrate(&BlastReport{})
		if err != nil {
			return nil, fmt.Errorf("failed to automigrate %s table: %v", (&BlastReport{}).TableName(), err)
		}
	}

	if !c2bAPI.SQLDB.Migrator().HasTable(&BlastFile{}) {
		err = c2bAPI.SQLDB.Migrator().AutoMigrate(&BlastFile{})
		if err != nil {
			return nil, fmt.Errorf("failed to automigrate %s table: %v", (&BlastFile{}).TableName(), err)
		}
	}

	if !c2bAPI.SQLDB.Migrator().HasTable(&UploadedFileData{}) {
		err = c2bAPI.SQLDB.Migrator().AutoMigrate(&UploadedFileData{})
		if err != nil {
			return nil, fmt.Errorf("failed to automigrate %s table: %v", (&UploadedFileData{}).TableName(), err)
		}
	}

	// Disable strict group by
	err = c2bAPI.SQLDB.Exec("SET GLOBAL sql_mode=(SELECT REPLACE(@@sql_mode,'ONLY_FULL_GROUP_BY',''));").Error
	if err != nil {
		return nil, fmt.Errorf("failed to set group by mode: %v", err)
	}

	// Insert worker
	go c2bAPI.insertWorker(ctx)

	// Update stats worker
	go c2bAPI.dailyStatWorker(ctx)

	return c2bAPI, nil
}

// ValidateC2BPayment validates MPESA transaction
func ValidateC2BPayment(payment *c2b.C2BPayment) error {
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
	case payment.TransactionTimeSeconds == 0:
		err = errs.MissingField("transaction time")
	}
	return err
}

// AddPrefix adds a prefix to redis key
func AddPrefix(key, prefix string) string {
	return fmt.Sprintf("%s:%s", prefix, key)
}

// AddPrefix adds a prefix to redis key
func (c2bAPI *c2bAPIServer) AddPrefix(key string) string {
	return AddPrefix(key, c2bAPI.RedisKeyPrefix)
}

func (c2bAPI *c2bAPIServer) CreateC2BPayment(
	ctx context.Context, createReq *c2b.CreateC2BPaymentRequest,
) (*c2b.CreateC2BPaymentResponse, error) {
	// Authentication
	_, err := c2bAPI.AuthAPI.AuthorizeAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case createReq == nil:
		return nil, errs.NilObject("CreateC2BRequest")
	default:
		err = ValidateC2BPayment(createReq.MpesaPayment)
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
		c2bAPI.insertChan <- &incomingPayment{
			payment: mpesaDB,
			publish: createReq.Publish,
		}
	}()

	return &c2b.CreateC2BPaymentResponse{
		PaymentId: fmt.Sprint(createReq.MpesaPayment.TransactionId),
	}, nil
}

func (c2bAPI *c2bAPIServer) GetC2BPayment(
	ctx context.Context, getReq *c2b.GetC2BPaymentRequest,
) (*c2b.C2BPayment, error) {
	var err error

	// Validation
	switch {
	case getReq == nil:
		return nil, errs.NilObject("GetC2BRequest")
	case getReq.PaymentId == "":
		return nil, errs.MissingField("payment id")
	default:
	}

	mpesaDB := &PaymentMpesa{}

	if paymentID, err1 := strconv.Atoi(getReq.PaymentId); err1 == nil && paymentID != 0 {
		err = c2bAPI.SQLDB.First(mpesaDB, paymentID).Error
	} else {
		err = c2bAPI.SQLDB.First(mpesaDB, "transaction_id=?", getReq.PaymentId).Error
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

func (c2bAPI *c2bAPIServer) ExistC2BPayment(
	ctx context.Context, existReq *c2b.ExistC2BPaymentRequest,
) (*c2b.ExistC2BPaymentResponse, error) {
	var err error

	// Validation
	switch {
	case existReq == nil:
		return nil, errs.NilObject("exist request")
	case existReq.PaymentId == "":
		return nil, errs.MissingField("payment id")
	default:
	}

	mpesaDB := &PaymentMpesa{}

	if paymentID, err1 := strconv.Atoi(existReq.PaymentId); err1 == nil && paymentID != 0 {
		err = c2bAPI.SQLDB.First(mpesaDB, paymentID).Error
	} else {
		err = c2bAPI.SQLDB.First(mpesaDB, "transaction_id=?", existReq.PaymentId).Error
	}

	switch {
	case err == nil:
		return &c2b.ExistC2BPaymentResponse{
			Exists: true,
			Amount: mpesaDB.Amount,
		}, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return &c2b.ExistC2BPaymentResponse{
			Exists: false,
			Amount: mpesaDB.Amount,
		}, nil
	default:
		return nil, errs.FailedToFind("mpesa payment", err)
	}
}

const defaultPageSize = 50

func userScopesKey(userID string) string {
	return fmt.Sprintf("user:%s:scopes", userID)
}
func userAllowedShortCodes(userID string) string {
	return fmt.Sprintf("user:%s:allowedshortcodes", userID)
}

func userAllowedAccSet(userID string) string {
	return fmt.Sprintf("user:%s:allowedaccounts", userID)
}

func userAllowedPhonesSet(userID string) string {
	return fmt.Sprintf("user:%s:allowedphones", userID)
}

func userAllowedAmounts(userID string) string {
	return fmt.Sprintf("user:%s:allowedamounts", userID)
}

func userAllowedPercent(userID string) string {
	return fmt.Sprintf("user:%s:percent", userID)
}

func (c2bAPI *c2bAPIServer) ListC2BPayments(
	ctx context.Context, listReq *c2b.ListC2BPaymentsRequest,
) (*c2b.ListC2BPaymentsResponse, error) {
	// Authentication
	actor, err := c2bAPI.AuthAPI.AuthenticateRequestV2(ctx)
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

	// Get scopes
	scopesPB, err := c2bAPI.GetScopes(ctx, &c2b.GetScopesRequest{
		UserId: actor.ID,
	})
	if err != nil {
		return nil, err
	}

	// Union request filter with scopes ACL
	var (
		msisdns        = getStringUnion(scopesPB.GetScopes().GetAllowedPhones(), listReq.GetFilter().GetMsisdns())
		accountNumbers = getStringUnion(scopesPB.GetScopes().GetAllowedAccNumber(), listReq.GetFilter().GetAccountsNumber())
		amounts        = getFloat32Union(scopesPB.GetScopes().GetAllowedAmounts(), listReq.GetFilter().GetAmounts())
		shortCodes     = getStringUnion(scopesPB.GetScopes().GetAllowedShortCodes(), listReq.GetFilter().GetShortCodes())
		pageSize       = listReq.GetPageSize()
		pageToken      = listReq.GetPageToken()
		percent        = scopesPB.GetScopes().GetPercentage()
		paymentID      uint
	)

	if pageSize <= 0 || pageSize > defaultPageSize {
		if c2bAPI.AuthAPI.IsAdmin(actor.Group) {
			pageSize = defaultPageSize
		}
	}

	if pageToken != "" {
		ids, err := c2bAPI.PaginationHasher.DecodeInt64WithError(listReq.GetPageToken())
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "failed to parse page token")
		}
		paymentID = uint(ids[0])
	}

	C2Bs := make([]*PaymentMpesa, 0, pageSize+1)

	db := c2bAPI.SQLDB.Limit(int(pageSize + 1)).Order("payment_id DESC").Debug()

	// Apply payment id filter
	if paymentID != 0 {
		db = db.Where("payment_id<?", paymentID)
	}

	// Apply filters
	if len(accountNumbers) > 0 {
		db = db.Where("reference_number IN(?)", accountNumbers)
	}
	if len(msisdns) > 0 {
		db = db.Where("msisdn IN(?)", msisdns)
	}
	if len(amounts) > 0 {
		db = db.Where("amount IN(?)", amounts)
	}
	if len(shortCodes) > 0 {
		db = db.Where("business_short_code IN(?)", shortCodes)
	}
	if listReq.GetFilter().GetStartTimeSeconds() < listReq.GetFilter().GetEndTimeSeconds() {
		db = db.Where(
			"transaction_time BETWEEN ? AND ?",
			time.Unix(listReq.GetFilter().GetStartTimeSeconds(), 0),
			time.Unix(listReq.GetFilter().GetEndTimeSeconds(), 0),
		)
	} else if listReq.GetFilter().GetTxDate() != "" {
		t, err := getTime(listReq.Filter.TxDate)
		if err != nil {
			return nil, err
		}
		db = db.Where("transaction_time BETWEEN ? AND ?", t, t.Add(time.Hour*24))
	}
	if listReq.GetFilter().GetProcessState() != c2b.ProcessedState_PROCESS_STATE_UNSPECIFIED {
		switch listReq.Filter.ProcessState {
		case c2b.ProcessedState_PROCESS_STATE_UNSPECIFIED:
		case c2b.ProcessedState_NOT_PROCESSED:
			db = db.Where("processed=false")
		case c2b.ProcessedState_PROCESSED:
			db = db.Where("processed=true")
		}
	}
	if listReq.GetFilter().GetOnlyUnique() {
		// db = db.Group("msisdn")
	}

	err = db.Find(&C2Bs).Error
	switch {
	case err == nil:
	default:
		return nil, errs.FailedToFind("c2b payments", err)
	}

	paymentsPB := make([]*c2b.C2BPayment, 0, len(C2Bs))

	for i, paymentDB := range C2Bs {
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
	if len(C2Bs) > int(pageSize) {
		// Next page token
		token, err = c2bAPI.PaginationHasher.EncodeInt64([]int64{int64(paymentID)})
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "failed to generate next page token")
		}
	}

	// Update percentage
	switch {
	case percent > 100:
		percent = 100
	case percent < 0:
		percent = 1
	case percent == 0:
		percent = 100
	}

	// Jan 27 2021 and wasted 30 minutes
	paymentsPB = paymentsPB[:int((float32(len(paymentsPB))*percent)/100)]

	return &c2b.ListC2BPaymentsResponse{
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

func stringArrayToFloat(arr []float32) []string {
	arr1 := make([]string, 0, len(arr))
	for _, v := range arr {
		arr1 = append(arr1, fmt.Sprint(v))
	}
	return arr1
}

func float32ArrToString(arr []float32) []string {
	arr1 := make([]string, 0, len(arr))
	for _, v := range arr {
		arr1 = append(arr1, fmt.Sprint(v))
	}
	return arr1
}

func stringArrToFloat32(arr []string) []float32 {
	arr1 := make([]float32, 0, len(arr))
	for _, v := range arr {
		v2, err := strconv.ParseFloat(v, 32)
		if err == nil {
			arr1 = append(arr1, float32(v2))
		}
	}
	return arr1
}

func (c2bAPI *c2bAPIServer) SaveScopes(
	ctx context.Context, addReq *c2b.SaveScopesRequest,
) (*emptypb.Empty, error) {
	// Validation
	switch {
	case addReq == nil:
		return nil, errs.NilObject("SaveScopesRequest")
	case addReq.UserId == "":
		return nil, errs.MissingField("user id")
	case addReq.Scopes == nil:
		return nil, errs.NilObject("scopes")
	}

	// Scopes json
	bs, err := json.Marshal(addReq.Scopes)
	if err != nil {
		return nil, errs.FromJSONMarshal(err, "scopes")
	}

	// Save in database
	scopesDB := &Scopes{
		UserID: addReq.UserId,
		Scopes: bs,
	}

	// Check if record exists
	err = c2bAPI.SQLDB.First(&Scopes{}, "user_id = ?", addReq.UserId).Error
	switch {
	case err == nil:
		// Update
		err = c2bAPI.SQLDB.Where("user_id = ?", addReq.UserId).Updates(scopesDB).Error
		if err != nil {
			return nil, errs.FailedToUpdate("scopes", err)
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		// Create
		err = c2bAPI.SQLDB.Create(scopesDB).Error
		if err != nil {
			return nil, errs.FailedToSave("scopes", err)
		}
	}

	// Proto marshal scopes
	bs, err = proto.Marshal(addReq.Scopes)
	if err != nil {
		return nil, errs.FromProtoMarshal(err, "scopes")
	}

	// Set in cache
	err = c2bAPI.RedisDB.Set(ctx, userScopesKey(addReq.UserId), bs, 0).Err()
	if err != nil {
		return nil, errs.RedisCmdFailed(err, "set")
	}

	return &emptypb.Empty{}, nil
}

func (c2bAPI *c2bAPIServer) GetScopes(
	ctx context.Context, getReq *c2b.GetScopesRequest,
) (*c2b.GetScopesResponse, error) {
	var err error

	// Validation
	switch {
	case getReq == nil:
		return nil, errs.NilObject("list request")
	case getReq.UserId == "":
		return nil, errs.MissingField("user id")
	}

	// Get scopes from redis
	key := userScopesKey(getReq.UserId)
	val, err := c2bAPI.RedisDB.Get(ctx, key).Result()
	switch {
	case err == nil:
	case errors.Is(err, redis.Nil):
		// Save default scopes
		_, err = c2bAPI.SaveScopes(ctx, &c2b.SaveScopesRequest{
			Scopes: &c2b.Scopes{},
			UserId: getReq.UserId,
		})
		if err != nil {
			return nil, err
		}
	default:
		return nil, errs.RedisCmdFailed(err, "get")
	}

	// Proto unmarshal scopes
	scopesPB := &c2b.Scopes{}
	if val != "" {
		err = proto.Unmarshal([]byte(val), scopesPB)
		if err != nil {
			return nil, errs.FromJSONMarshal(err, "scopes")
		}
	}

	return &c2b.GetScopesResponse{
		Scopes: scopesPB,
	}, nil
}

func (c2bAPI *c2bAPIServer) ProcessC2BPayment(
	ctx context.Context, processReq *c2b.ProcessC2BPaymentRequest,
) (*emptypb.Empty, error) {
	// Authentication
	_, err := c2bAPI.AuthAPI.AuthorizeAdmin(ctx)
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
		db := c2bAPI.SQLDB.Model(&PaymentMpesa{}).Unscoped().Where("payment_id=?", processReq.PaymentId).Update("processed", processReq.State)
		err = db.Error
		count = db.RowsAffected
	} else {
		db := c2bAPI.SQLDB.Model(&PaymentMpesa{}).Unscoped().Where("transaction_id=?", processReq.PaymentId).Update("processed", processReq.State)
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
				c2bAPI.Logger.Infoln("started goroutine to update mpesa payment once its received")

				updateFn := func() int64 {
					return c2bAPI.SQLDB.Model(&PaymentMpesa{}).Where("transaction_id=?", paymentID).Update("processed", processReq.State).RowsAffected
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

func (c2bAPI *c2bAPIServer) PublishC2BPayment(
	ctx context.Context, pubReq *c2b.PublishC2BPaymentRequest,
) (*emptypb.Empty, error) {
	// Authentication
	_, err := c2bAPI.AuthAPI.AuthorizeAdmin(ctx)
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
	ctbPayment, err := c2bAPI.GetC2BPayment(ctx, &c2b.GetC2BPaymentRequest{
		PaymentId: pubReq.PaymentId,
	})
	if err != nil {
		return nil, err
	}

	publishPayload := fmt.Sprintf("PAYMENT:%s:%s", pubReq.PaymentId, pubReq.InitiatorId)

	// Publish based on state
	switch pubReq.ProcessedState {
	case c2b.ProcessedState_PROCESS_STATE_UNSPECIFIED:
		err = c2bAPI.RedisDB.Publish(
			ctx, c2bAPI.AddPrefix(c2bAPI.PublishChannel), publishPayload,
		).Err()
		if err != nil {
			return nil, errs.RedisCmdFailed(err, "PUBSUB")
		}
	case c2b.ProcessedState_NOT_PROCESSED:
		// Publish only if the processed state is false
		if !ctbPayment.Processed {
			err = c2bAPI.RedisDB.Publish(
				ctx, c2bAPI.AddPrefix(c2bAPI.PublishChannel), publishPayload,
			).Err()
			if err != nil {
				return nil, errs.RedisCmdFailed(err, "PUBSUB")
			}
		}
	case c2b.ProcessedState_PROCESSED:
		// Publish only if the processed state is true
		if ctbPayment.Processed {
			err = c2bAPI.RedisDB.Publish(
				ctx, c2bAPI.AddPrefix(c2bAPI.PublishChannel), publishPayload,
			).Err()
			if err != nil {
				return nil, errs.RedisCmdFailed(err, "PUBSUB")
			}
		}
	}

	return &emptypb.Empty{}, nil
}

func (c2bAPI *c2bAPIServer) PublishAllC2BPayments(
	ctx context.Context, pubReq *c2b.PublishAllC2BPaymentsRequest,
) (*emptypb.Empty, error) {
	// Authentication
	_, err := c2bAPI.AuthAPI.AuthorizeAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case pubReq == nil:
		return nil, errs.NilObject("publish all request")
	case pubReq.StartTimeSeconds > time.Now().Unix() || pubReq.EndTimeSeconds > time.Now().Unix():
		return nil, errs.WrapMessage(codes.InvalidArgument, "cannot work with future times")
	default:
		if pubReq.EndTimeSeconds != 0 || pubReq.StartTimeSeconds != 0 {
			if pubReq.EndTimeSeconds < pubReq.StartTimeSeconds {
				return nil, errs.WrapMessage(
					codes.InvalidArgument, "start timestamp cannot be greater than end timestamp",
				)
			}
		}
	}

	if pubReq.StartTimeSeconds == 0 {
		if pubReq.EndTimeSeconds == 0 {
			pubReq.EndTimeSeconds = time.Now().Unix()
		}
		pubReq.StartTimeSeconds = pubReq.EndTimeSeconds - int64(7*24*60*60)
		if pubReq.StartTimeSeconds < 0 {
			pubReq.StartTimeSeconds = 0
		}
	}

	var (
		nextPageToken string
		pageSize      int32 = 1000
		next          bool  = true
	)

	for next {
		// List transactions
		listRes, err := c2bAPI.ListC2BPayments(ctx, &c2b.ListC2BPaymentsRequest{
			PageToken: nextPageToken,
			PageSize:  pageSize,
			Filter: &c2b.ListC2BPaymentsFilter{
				ProcessState:     pubReq.ProcessedState,
				StartTimeSeconds: pubReq.StartTimeSeconds,
				EndTimeSeconds:   pubReq.StartTimeSeconds,
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
		pipeliner := c2bAPI.RedisDB.Pipeline()

		// Publish the mpesa transactions to listeners
		for _, mpesaPB := range listRes.MpesaPayments {
			publishPayload := fmt.Sprintf("PAYMENT:%s:%s", mpesaPB.PaymentId, "")
			err := pipeliner.Publish(
				ctx, c2bAPI.AddPrefix(c2bAPI.PublishChannel), publishPayload,
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

func inStringArray(element string, array []string) bool {
	for _, v := range array {
		if v == element {
			return true
		}
	}
	return false
}

func inFloat32Array(element float32, array []float32) bool {
	for _, v := range array {
		if v == element {
			return true
		}
	}
	return false
}

func getStringUnion(arr1, arr2 []string) []string {
	if len(arr1) == 0 {
		return arr2
	}
	if len(arr2) == 0 {
		return arr1
	}
	arr3 := make([]string, 0, len(arr1))
	for _, v := range arr2 {
		if inStringArray(v, arr1) {
			arr3 = append(arr3, v)
		}
	}
	if len(arr3) == 0 {
		return arr1
	}
	return arr3
}

func getFloat32Union(arr1, arr2 []float32) []float32 {
	if len(arr1) == 0 {
		return arr2
	}
	if len(arr2) == 0 {
		return arr1
	}
	arr3 := make([]float32, 0, len(arr1))
	for _, v := range arr2 {
		if inFloat32Array(v, arr1) {
			arr3 = append(arr3, v)
		}
	}
	if len(arr3) == 0 {
		return arr1
	}
	return arr3
}

func (c2bAPI *c2bAPIServer) GetTransactionsCount(
	ctx context.Context, getReq *c2b.GetTransactionsCountRequest,
) (*c2b.TransactionsSummary, error) {
	// Authentication
	actor, err := c2bAPI.AuthAPI.GetJwtPayload(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case getReq == nil:
		return nil, errs.NilObject("get transactions count")
	default:
		if getReq.StartTimeSeconds > 0 || getReq.EndTimeSeconds > 0 {
			if getReq.StartTimeSeconds > getReq.EndTimeSeconds {
				return nil, errs.WrapMessage(codes.InvalidArgument, "start time cannot be greater than end time")
			}
		}
	}

	// Get scopes
	scopesPB, err := c2bAPI.GetScopes(ctx, &c2b.GetScopesRequest{
		UserId: actor.ID,
	})
	if err != nil {
		return nil, err
	}

	// Union request filter with scopes ACL
	var (
		msisdns        = getStringUnion(scopesPB.GetScopes().GetAllowedPhones(), getReq.GetMsisdns())
		accountNumbers = getStringUnion(scopesPB.GetScopes().GetAllowedAccNumber(), getReq.GetAccountsNumber())
		shortCodes     = getStringUnion(scopesPB.GetScopes().GetAllowedShortCodes(), getReq.GetShortCodes())
		amounts        = getFloat32Union(scopesPB.GetScopes().GetAllowedAmounts(), getReq.GetAmounts())
	)

	db := c2bAPI.SQLDB.Model(&PaymentMpesa{})

	// Apply filters
	if len(amounts) > 0 {
		db = db.Where("amount IN (?)", amounts)
	}
	if len(shortCodes) > 0 {
		db = db.Where("business_short_code IN (?)", shortCodes)
	}
	if len(accountNumbers) > 0 {
		db = db.Where("reference_number IN (?)", accountNumbers)
	}
	if len(msisdns) > 0 {
		db = db.Where("msisdn IN (?)", msisdns)
	}
	if getReq.StartTimeSeconds > 0 || getReq.EndTimeSeconds > 0 {
		db = db.Where("transaction_time BETWEEN ? AND ?", time.Unix(getReq.StartTimeSeconds, 0), time.Unix(getReq.EndTimeSeconds, 0))
	}

	var transactions int64

	// Count of transactions
	err = db.Count(&transactions).Error
	if err != nil {
		return nil, errs.FailedToFind("transactions", err)
	}

	var totalAmount sql.NullFloat64

	// Get total amount
	err = db.Model(&PaymentMpesa{}).Select("sum(amount) as total").Row().Scan(&totalAmount)
	if err != nil {
		return nil, errs.FailedToFind("total", err)
	}

	var totalAmountF float32

	if totalAmount.Valid {
		totalAmountF = float32(totalAmount.Float64)
	}

	percent := scopesPB.GetScopes().GetPercentage()
	switch {
	case percent > 100:
		percent = 100
	case percent < 0:
		percent = 1
	case percent == 0:
		percent = 100
	}

	totalTransactions := int32((float32(transactions) * percent) / 100)
	totalAmount2 := (totalAmountF * percent) / 100
	if totalTransactions == 0 {
		totalAmount2 = 0
	}

	return &c2b.TransactionsSummary{
		TotalAmount:       totalAmount2,
		TransactionsCount: totalTransactions,
	}, nil
}

type total struct {
	total float32
}

func (c2bAPI *c2bAPIServer) GetRandomTransaction(
	ctx context.Context, getReq *c2b.GetRandomTransactionRequest,
) (*c2b.C2BPayment, error) {
	// Authentication
	_, err := c2bAPI.AuthAPI.AuthorizeAdmin(ctx)
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
		amounts := []float32{}
		if getReq.Amount > 0 {
			amounts = []float32{getReq.Amount}
		}
		listRes, err := c2bAPI.ListC2BPayments(ctx, &c2b.ListC2BPaymentsRequest{
			PageToken: pageToken,
			PageSize:  defaultPageSize,
			Filter: &c2b.ListC2BPaymentsFilter{
				AccountsNumber:   getReq.GetAccountsNumber(),
				Amounts:          amounts,
				StartTimeSeconds: getReq.GetStartTimeSeconds(),
				EndTimeSeconds:   getReq.GetEndTimeSeconds(),
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

	return c2bAPI.GetC2BPayment(ctx, &c2b.GetC2BPaymentRequest{
		PaymentId: fmt.Sprint(winnerID),
	})
}

func (c2bAPI *c2bAPIServer) ArchiveTransactions(
	ctx context.Context, archiveReq *c2b.ArchiveTransactionsRequest,
) (*emptypb.Empty, error) {
	// Authentication
	_, err := c2bAPI.AuthAPI.AuthorizeAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case archiveReq == nil:
		return nil, errs.NilObject("get request")
	case archiveReq.ArchiveName == "":
		return nil, errs.MissingField("archive name")
	}

	liveTable := (&PaymentMpesa{}).TableName()

	archiveTable := fmt.Sprintf("%s_%s", liveTable, archiveReq.ArchiveName)

	// Check table does not exist
	if c2bAPI.SQLDB.Migrator().HasTable(archiveTable) {
		return nil, errs.WrapMessage(codes.AlreadyExists, "archive name exists, use a different one")
	}

	// Create table and copy data
	err = c2bAPI.SQLDB.Exec(fmt.Sprintf("CREATE TABLE %s AS SELECT * FROM %s", archiveTable, liveTable)).Error
	if err != nil {
		return nil, errs.WrapErrorWithCodeAndMsg(codes.Internal, err, "failed to archive table")
	}

	// Truncate the old table
	err = c2bAPI.SQLDB.Exec(fmt.Sprintf("TRUNCATE TABLE %s", liveTable)).Error
	if err != nil {
		return nil, errs.WrapErrorWithCodeAndMsg(codes.Internal, err, "failed to truncate table")
	}

	return &emptypb.Empty{}, nil
}

func (c2bAPI *c2bAPIServer) GetStats(
	ctx context.Context, getStat *c2b.GetStatsRequest,
) (*c2b.StatsResponse, error) {
	// Authentication
	_, err := c2bAPI.AuthAPI.AuthorizeAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case getStat == nil:
		return nil, errs.NilObject("get stats request")
	case getStat.ShortCode == "":
		return nil, errs.NilObject("short code")
	case getStat.AccountName == "":
		return nil, errs.NilObject("account name")
	case len(getStat.Dates) == 0:
		return nil, errs.MissingField("dates")
	}

	var (
		mu      = &sync.Mutex{} // guards statsPB
		errChan = make(chan error, len(getStat.Dates))
		statsPB = make([]*c2b.Stat, 0, len(getStat.Dates))
	)

	for _, date := range getStat.Dates {

		go func(date string) {

			// Get stats for each day listed
			statDB := &Stat{}
			err = c2bAPI.SQLDB.First(statDB, "date = ? AND short_code = ? AND account_name = ?", date, getStat.ShortCode, getStat.AccountName).Error
			switch {
			case err == nil:
			case errors.Is(err, gorm.ErrRecordNotFound):
				startTime, err := getTime(date)
				if err != nil {
					errChan <- err
					return
				}
				// generate stat
				c2bAPI.generateStatistics(ctx, startTime.Unix(), startTime.Unix()+(24*3600))
				err = c2bAPI.SQLDB.First(statDB, "date = ? AND short_code = ? AND account_name = ?", date, getStat.ShortCode, getStat.AccountName).Error
				if err != nil && errors.Is(err, gorm.ErrRecordNotFound) == false {
					errChan <- err
					return
				}
			default:
				errChan <- errs.FailedToFind("stat", err)
				return
			}

			statPB, err := GetStatPB(statDB)
			if err != nil {
				errChan <- err
				return
			}

			mu.Lock()
			statsPB = append(statsPB, statPB)
			mu.Unlock()

			errChan <- nil
		}(date)
	}

	for range getStat.Dates {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err := <-errChan:
			if err != nil {
				return nil, err
			}
		}
	}

	return &c2b.StatsResponse{
		Stats: statsPB,
	}, nil
}

func (c2bAPI *c2bAPIServer) ListStats(
	ctx context.Context, listReq *c2b.ListStatsRequest,
) (*c2b.StatsResponse, error) {
	// Authorize the request
	actor, err := c2bAPI.AuthAPI.GetJwtPayload(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case listReq == nil:
		return nil, errs.NilObject("list request")
	}

	// Get scopes for actor
	scopesPB, err := c2bAPI.GetScopes(ctx, &c2b.GetScopesRequest{
		UserId: actor.ID,
	})
	if err != nil {
		return nil, err
	}

	// Union request filter with scopes ACL
	var (
		pageSize       = listReq.GetPageSize()
		pageToken      = listReq.GetPageToken()
		accountNumbers = getStringUnion(scopesPB.GetScopes().GetAllowedAccNumber(), listReq.GetFilter().GetAccountsNumber())
		shortCodes     = getStringUnion(scopesPB.GetScopes().GetAllowedShortCodes(), listReq.GetFilter().GetShortCodes())

		statID uint
	)

	if pageSize <= 0 || pageSize > defaultPageSize {
		pageSize = defaultPageSize
	}

	if pageToken != "" {
		ids, err := c2bAPI.PaginationHasher.DecodeInt64WithError(listReq.GetPageToken())
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "failed to parse page token")
		}
		statID = uint(ids[0])
	}

	stats := make([]*Stat, 0, pageSize+1)

	db := c2bAPI.SQLDB.Limit(int(pageSize + 1)).Order("stat_id DESC")

	// Apply payment id filter
	if statID != 0 {
		db = db.Where("stat_id<?", statID)
	}

	// Apply filters
	if len(accountNumbers) > 0 {
		db = db.Where("account_name IN(?)", accountNumbers)
	}
	if len(shortCodes) > 0 {
		db = db.Where("short_code IN(?)", shortCodes)
	}
	if listReq.GetFilter().GetStartTimeSeconds() < listReq.GetFilter().GetEndTimeSeconds() {
		db = db.Where("created_at BETWEEN ? AND ?", listReq.GetFilter().GetStartTimeSeconds(), listReq.GetFilter().GetEndTimeSeconds())
	} else if listReq.GetFilter().GetTxDate() != "" {
		db = db.Where("date = ?", listReq.GetFilter().GetTxDate())
	}

	err = db.Find(&stats).Error
	switch {
	case err == nil:
	default:
		return nil, errs.FailedToFind("c2b stat", err)
	}

	percent := scopesPB.GetScopes().GetPercentage()
	switch {
	case percent > 100:
		percent = 100
	case percent < 0:
		percent = 1
	case percent == 0:
		percent = 100
	}

	statsPB := make([]*c2b.Stat, 0, len(stats))

	for i, stat := range stats {
		statPB, err := GetStatPB(stat)
		if err != nil {
			return nil, err
		}

		// Ignore the last element
		if i == int(pageSize) {
			break
		}

		// Update total transactions and cost using scopes
		totalTransactions := int32((float32(statPB.TotalTransactions) * percent) / 100)
		totalAmount := (statPB.TotalAmount * percent) / 100
		if totalTransactions == 0 {
			totalAmount = 0
		}

		statPB.TotalAmount = totalAmount
		statPB.TotalTransactions = totalTransactions

		statsPB = append(statsPB, statPB)
		statID = stat.StatID
	}

	var token string
	if len(stats) > int(pageSize) {
		// Next page token
		token, err = c2bAPI.PaginationHasher.EncodeInt64([]int64{int64(statID)})
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "failed to generate next page token")
		}
	}

	return &c2b.StatsResponse{
		Stats:         statsPB,
		NextPageToken: token,
	}, nil
}
