package b2c

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gidyon/micro/v2/pkg/middleware/grpc/auth"
	"github.com/gidyon/micro/v2/utils/errs"
	b2c_model "github.com/gidyon/mpesapayments/internal/b2c"
	b2c "github.com/gidyon/mpesapayments/pkg/api/b2c/v2"
	"github.com/gidyon/mpesapayments/pkg/payload"
	"github.com/gidyon/mpesapayments/pkg/utils/formatutil"
	"github.com/gidyon/mpesapayments/pkg/utils/httputils"
	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
)

const SourceKey = "source"

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

type b2cAPIServer struct {
	b2c.UnimplementedB2CV2Server
	*Options
}

// Options contains options for starting b2c service
type Options struct {
	// PublishChannel     string
	QueryBalanceURL    string
	B2CURL             string
	ReversalURL        string
	SQLDB              *gorm.DB
	RedisDB            *redis.Client
	Logger             grpclog.LoggerV2
	AuthAPI            auth.API
	HTTPClient         httpClient
	OptionB2C          *OptionB2C
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
	case opt.OptionB2C == nil:
		err = errs.NilObject("b2c options")
	case opt.QueryBalanceURL == "":
		err = errs.MissingField("query balance url")
	case opt.B2CURL == "":
		err = errs.MissingField("b2c url")
	case opt.ReversalURL == "":
		err = errs.MissingField("reversal url")
	}

	return err
}

// OptionB2C contains options for doing b2c with mpesa
type OptionB2C struct {
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

// ValidateOptionB2C validates b2c options
func ValidateOptionB2C(opt *OptionB2C) error {
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
func NewB2CAPI(ctx context.Context, opt *Options) (b2c.B2CV2Server, error) {
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
		err = ValidateOptionB2C(opt.OptionB2C)
		if err != nil {
			return nil, err
		}
	}

	// Update Basic Token
	opt.OptionB2C.basicToken = base64.StdEncoding.EncodeToString([]byte(
		opt.OptionB2C.ConsumerKey + ":" + opt.OptionB2C.ConsumerSecret,
	))

	b2cAPI := &b2cAPIServer{
		Options: opt,
	}

	// Auto migration
	// if !b2cAPI.SQLDB.Migrator().HasTable(&b2c_model.Payment{}) {
	err = b2cAPI.SQLDB.Migrator().AutoMigrate(&b2c_model.Payment{})
	if err != nil {
		return nil, err
	}
	// }

	// if !b2cAPI.SQLDB.Migrator().HasTable(&b2c_model.DailyStat{}) {
	err = b2cAPI.SQLDB.Migrator().AutoMigrate(&b2c_model.DailyStat{})
	if err != nil {
		return nil, err
	}
	// }

	// Worker for updating access token
	go b2cAPI.updateAccessTokenWorker(ctx, 30*time.Minute)

	// Worker to generate daily statistics
	go b2cAPI.dailyDailyStatWorker(ctx)

	return b2cAPI, nil
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

	var phone int

	// Validate request
	switch {
	case req == nil:
		return nil, errs.NilObject("transfer request")
	case req.InitiatorId == "":
		return nil, errs.MissingField("inititator id")
	case req.Amount == 0:
		return nil, errs.MissingField("amount")
	case req.CommandId == b2c.CommandId_COMMANDID_UNSPECIFIED:
		return nil, errs.MissingField("command id")
	case req.Msisdn == "":
		return nil, errs.MissingField("msisdn")
	case req.ShortCode == "":
		return nil, errs.MissingField("short code")
	case req.Remarks == "":
		return nil, errs.MissingField("remarks")
	case req.Publish && req.GetPublishMessage().GetChannelName() == "":
		return nil, errs.MissingField("publisch channel")
	default:
		phone, err = strconv.Atoi(formatutil.FormatPhoneKE(req.Msisdn))
		if err != nil {
			return nil, errs.WrapMessage(codes.InvalidArgument, "incorrect org short code")
		}
	}

	var commandID string
	switch req.CommandId {
	case b2c.CommandId_BUSINESS_PAYMENT:
		commandID = "BusinessPayment"
	case b2c.CommandId_PROMOTION_PAYMENT:
		commandID = "PromotionPayment"
	case b2c.CommandId_SALARY_PAYMENT:
		commandID = "SalaryPayment"
	}

	// Transfer funds payload
	queryBalPayload := &payload.B2CRequest{
		InitiatorName:      b2cAPI.Options.OptionB2C.InitiatorUsername,
		SecurityCredential: b2cAPI.OptionB2C.InitiatorEncryptedPassword,
		CommandID:          commandID,
		Amount:             fmt.Sprint(req.Amount),
		PartyA:             fmt.Sprint(req.ShortCode),
		PartyB:             int64(phone),
		Remarks:            req.Remarks,
		QueueTimeOutURL:    fmt.Sprintf("%s?%s=DARAJA", b2cAPI.OptionB2C.QueueTimeOutURL, SourceKey),
		ResultURL:          fmt.Sprintf("%s?%s=DARAJA", b2cAPI.OptionB2C.ResultURL, SourceKey),
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

	reqHttp, err := http.NewRequest(http.MethodPost, b2cAPI.B2CURL, bytes.NewReader(bs))
	if err != nil {
		return nil, errs.WrapMessage(codes.Internal, "failed to create transfer funds request")
	}

	reqHttp.Header.Set("Authorization", fmt.Sprintf("Bearer %s", b2cAPI.OptionB2C.accessToken))
	reqHttp.Header.Set("Content-Type", "application/json")

	httputils.DumpRequest(reqHttp, "TransferFunds Request")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		res, err := b2cAPI.HTTPClient.Do(reqHttp)
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
			var (
				convId     = fmt.Sprint(apiRes.Response["ConversationID"])
				origConvId = fmt.Sprint(apiRes.Response["OriginatorConversationID"])
			)

			// Save the request to database
			err = b2cAPI.SQLDB.Create(&b2c_model.Payment{
				ID:                            0,
				InitiatorID:                   req.InitiatorId,
				InitiatorTransactionReference: req.InitiatorTransactionReference,
				InitiatorCustomerReference:    req.InitiatorCustomerReference,
				InitiatorCustomerNames:        req.InitiatorCustomerNames,
				Msisdn:                        fmt.Sprint(phone),
				OrgShortCode:                  req.ShortCode,
				CommandId:                     req.CommandId.String(),
				Amount:                        float32(req.Amount),
				ConversationID:                convId,
				OriginatorConversationID:      origConvId,
				ResponseDescription:           fmt.Sprint(apiRes.Response["ResponseDescription"]),
				ResponseCode:                  fmt.Sprint(apiRes.Response["ResponseCode"]),
				ResultCode:                    "",
				ResultDescription:             "",
				WorkingAccountFunds:           0,
				UtilityAccountFunds:           0,
				MpesaCharges:                  0,
				OnfonCharges:                  0,
				RecipientRegistered:           false,
				MpesaReceiptId:                "",
				ReceiverPublicName:            "",
				Status:                        b2c.B2CStatus_B2C_REQUEST_SUBMITED.String(),
				Source:                        "",
				Tag:                           "",
				Succeeded:                     "",
				Processed:                     "",
				TransactionTime:               sql.NullTime{Valid: true, Time: time.Now().UTC()},
				CreatedAt:                     time.Time{},
			}).Error
			if err != nil {
				b2cAPI.Logger.Errorln("Failed to create b2c payload: ", err)
			}

			// Marshal req
			bs, err := proto.Marshal(req)
			if err != nil {
				b2cAPI.Logger.Errorln(err)
				return
			}

			requestId := GetMpesaRequestKey(convId)

			// Save in cache
			err = b2cAPI.RedisDB.Set(ctx, requestId, bs, time.Minute*30).Err()
			if err != nil {
				b2cAPI.Logger.Errorln("Failed to set TransferFunds Request to cache: ", err)
				return
			}
		default:
		}
	}()

	return &b2c.TransferFundsResponse{
		Progress: true,
		Message:  "Processing. Disbursement will come",
	}, nil
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

	db := &b2c_model.Payment{}

	if !req.IsMpesaId {
		err = b2cAPI.SQLDB.First(db, "id=?", key).Error
	} else {
		err = b2cAPI.SQLDB.First(db, "mpesa_receipt_id=?", req.PaymentId).Error
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
		Initiator:          b2cAPI.OptionB2C.InitiatorUsername,
		SecurityCredential: b2cAPI.OptionB2C.InitiatorEncryptedPassword,
		QueueTimeOutURL:    b2cAPI.OptionB2C.QueueTimeOutURL,
		ResultURL:          b2cAPI.OptionB2C.ResultURL,
	}

	// Json Marshal
	bs, err := json.Marshal(queryBalPayload)
	if err != nil {
		return nil, errs.FromJSONMarshal(err, "b2cPayload")
	}

	// Create request
	reqHttp, err := http.NewRequest(http.MethodPost, b2cAPI.QueryBalanceURL, bytes.NewReader(bs))
	if err != nil {
		return nil, errs.WrapMessage(codes.Internal, "failed to create request to query balance")
	}

	// Update headers
	reqHttp.Header.Set("Authorization", fmt.Sprintf("Bearer %s", b2cAPI.OptionB2C.accessToken))
	reqHttp.Header.Set("Content-Type", "application/json")

	httputils.DumpRequest(reqHttp, "QueryAccountBalance Request")

	// Post to MPESA API
	res, err := b2cAPI.HTTPClient.Do(reqHttp)
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
		Initiator:              b2cAPI.Options.OptionB2C.InitiatorUsername,
		SecurityCredential:     b2cAPI.OptionB2C.InitiatorEncryptedPassword,
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
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", b2cAPI.OptionB2C.accessToken))
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

	db := b2cAPI.SQLDB.Model(&b2c_model.Payment{}).Limit(int(pageSize + 1))
	if key != "" {
		switch req.GetFilter().GetOrderField() {
		case b2c.B2COrderField_B2C_PAYMENT_ID:
			db = db.Where("id<?", key).Order("id DESC")
		case b2c.B2COrderField_B2C_TRANSACTION_TIMESTAMP:
			db = db.Where("transaction_time<?", key).Order("transaction_time DESC")
		}
	} else {
		switch req.GetFilter().GetOrderField() {
		case b2c.B2COrderField_B2C_PAYMENT_ID:
			db = db.Order("id DESC")
		case b2c.B2COrderField_B2C_TRANSACTION_TIMESTAMP:
			db = db.Order("transaction_time DESC")
		}
	}

	// Apply filters
	if req.Filter != nil {
		startTimestamp := req.Filter.GetStartTimestamp()
		endTimestamp := req.Filter.GetEndTimestamp()

		if endTimestamp > startTimestamp {
			switch req.GetFilter().GetOrderField() {
			case b2c.B2COrderField_B2C_PAYMENT_ID:
				db = db.Where("created_at BETWEEN ? AND ?", time.Unix(startTimestamp, 0), time.Unix(endTimestamp, 0))
			case b2c.B2COrderField_B2C_TRANSACTION_TIMESTAMP:
				db = db.Where("transaction_time BETWEEN ? AND ?", time.Unix(startTimestamp, 0), time.Unix(endTimestamp, 0))
			}
		} else if req.Filter.TxDate != "" {
			t, err := getTime(req.Filter.TxDate)
			if err != nil {
				return nil, err
			}
			switch req.GetFilter().GetOrderField() {
			case b2c.B2COrderField_B2C_PAYMENT_ID:
				db = db.Where("created_at BETWEEN ? AND ?", t, t.Add(time.Hour*24))
			case b2c.B2COrderField_B2C_TRANSACTION_TIMESTAMP:
				db = db.Where("transaction_time BETWEEN ? AND ?", t, t.Add(time.Hour*24))
			}
		}

		if len(req.Filter.Msisdns) > 0 {
			db = db.Where("phone_number IN(?)", req.Filter.Msisdns)
		}

		if len(req.Filter.MpesaReceipts) > 0 {
			db = db.Where("mpesa_receipt_id IN(?)", req.Filter.MpesaReceipts)
		}

		if len(req.Filter.InitiatorCustomerReferences) > 0 {
			db = db.Where("initiator_customer_reference IN(?)", req.Filter.InitiatorCustomerReferences)
		}

		if len(req.Filter.InitiatorTransactionReferences) > 0 {
			db = db.Where("initiator_transaction_reference IN(?)", req.Filter.InitiatorTransactionReferences)
		}

		if len(req.Filter.ShortCodes) > 0 {
			db = db.Where("org_short_code IN(?)", req.Filter.ShortCodes)
		}

		if len(req.Filter.B2CStatuses) > 0 {
			ss := make([]string, 0, len(req.Filter.B2CStatuses))
			for _, s := range req.Filter.B2CStatuses {
				ss = append(ss, s.String())
			}
			db = db.Where("b2c_status IN(?)", ss)
		}

		switch req.Filter.ProcessState {
		case b2c.B2CProcessedState_B2C_PROCESS_STATE_UNSPECIFIED:
		case b2c.B2CProcessedState_B2C_NOT_PROCESSED:
			db = db.Where("processed=false")
		case b2c.B2CProcessedState_B2C_PROCESSED:
			db = db.Where("processed=true")
		}
	}

	var collectionCount int64

	if pageToken == "" {
		err = db.Count(&collectionCount).Error
		if err != nil {
			b2cAPI.Logger.Errorln(err)
			return nil, errs.WrapMessage(codes.Internal, "failed to get count of b2c transactions")
		}
	}

	txs := make([]*b2c_model.Payment, 0, pageSize+1)

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
		case b2c.B2COrderField_B2C_PAYMENT_ID:
			key = fmt.Sprint(db.ID)
		case b2c.B2COrderField_B2C_TRANSACTION_TIMESTAMP:
			key = db.TransactionTime.Time.UTC().Format(time.RFC3339)
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
		err = b2cAPI.SQLDB.Model(&b2c_model.Payment{}).Unscoped().Where("id=?", key).
			Update("processed", req.Processed).Error
	} else {
		err = b2cAPI.SQLDB.Model(&b2c_model.Payment{}).Unscoped().Where("mpesa_receipt_id=?", req.PaymentId).
			Update("processed", req.Processed).Error
	}
	switch {
	case err == nil:
	default:
		b2cAPI.Logger.Errorln(err)
		return nil, errs.WrapMessage(codes.Internal, "failed to process b2c transaction")
	}

	return &emptypb.Empty{}, nil
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
	if req.GetPublishMessage().GetPaymentId() != "" {
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
	channel := req.GetPublishMessage().GetPublishInfo().GetChannelName()
	if channel == "" {
		return &emptypb.Empty{}, nil
	}

	// Publish based on state
	switch req.ProcessedState {
	case b2c.B2CProcessedState_B2C_PROCESS_STATE_UNSPECIFIED:
		err = b2cAPI.RedisDB.Publish(ctx, channel, bs).Err()
		if err != nil {
			return nil, errs.RedisCmdFailed(err, "PUBSUB")
		}
	case b2c.B2CProcessedState_B2C_NOT_PROCESSED:
		// Publish only if the processed state is false
		if !pb.Processed {
			err = b2cAPI.RedisDB.Publish(ctx, channel, bs).Err()
			if err != nil {
				return nil, errs.RedisCmdFailed(err, "PUBSUB")
			}
		}
	case b2c.B2CProcessedState_B2C_PROCESSED:
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

	stats := make([]*b2c_model.DailyStat, 0, pageSize+1)

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
