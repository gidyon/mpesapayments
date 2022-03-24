package stk

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/gidyon/micro/v2/utils/errs"
	stk "github.com/gidyon/mpesapayments/pkg/api/stk/v1"
)

// StkTable is table for mpesa payments
const StkTable = "stk_transactions"

var tablePrefix = ""

// string short_code = 5;
// string account_reference = 6;
// string transaction_desc = 7;

// STKTransaction contains mpesa stk transaction details
type STKTransaction struct {
	ID                uint         `gorm:"primaryKey;autoIncrement"`
	InitiatorID       string       `gorm:"index;type:varchar(50);not null"`
	MerchantRequestID string       `gorm:"index;type:varchar(50);not null"`
	CheckoutRequestID string       `gorm:"type:varchar(50);not null"`
	ShortCode         string       `gorm:"index;type:varchar(15)"`
	AccountReference  string       `gorm:"index;type:varchar(50)"`
	TransactionDesc   string       `gorm:"type:varchar(100)"`
	ResultCode        string       `gorm:"type:varchar(5);not null"`
	ResultDesc        string       `gorm:"type:varchar(100);not null"`
	Amount            string       `gorm:"type:float(10);not null"`
	MpesaReceiptId    string       `gorm:"index;type:varchar(50)"`
	PhoneNumber       string       `gorm:"index;type:varchar(50);not null"`
	Succeeded         string       `gorm:"index;type:enum('YES','NO');default:NO"`
	Processed         string       `gorm:"index;type:enum('YES','NO');default:NO"`
	TransactionTime   sql.NullTime `gorm:"index:;type:datetime(6)"`
	CreateTime        time.Time    `gorm:"autoCreateTime:nano;not null;type:datetime(6)"`
}

// TableName returns the name of the table
func (*STKTransaction) TableName() string {
	// Get table prefix
	if tablePrefix != "" {
		return fmt.Sprintf("%s_%s", tablePrefix, StkTable)
	}
	return StkTable
}

// STKTransactionModel converts protobuf mpesa message to MPESASTKTransaction
func STKTransactionModel(pb *stk.StkTransaction) (*STKTransaction, error) {
	if pb == nil {
		return nil, errs.NilObject("mpesa payment")
	}

	success := "NO"
	if pb.Succeeded {
		success = "YES"
	}

	processed := "NO"
	if pb.Processed {
		processed = "YES"
	}

	db := &STKTransaction{
		ID:                0,
		InitiatorID:       pb.InitiatorId,
		MerchantRequestID: pb.MerchantRequestId,
		CheckoutRequestID: pb.CheckoutRequestId,
		ShortCode:         pb.ShortCode,
		AccountReference:  pb.AccountReference,
		TransactionDesc:   pb.TransactionDesc,
		ResultCode:        pb.ResultCode,
		ResultDesc:        pb.ResultDesc,
		Amount:            pb.Amount,
		MpesaReceiptId:    pb.MpesaReceiptId,
		PhoneNumber:       pb.PhoneNumber,
		Succeeded:         success,
		Processed:         processed,
		TransactionTime:   sql.NullTime{},
		CreateTime:        time.Time{},
	}

	if pb.TransactionTimestamp != 0 {
		db.TransactionTime = sql.NullTime{Valid: true, Time: time.Unix(pb.TransactionTimestamp, 0)}
	}

	return db, nil
}

// STKTransactionPB returns the protobuf message of mpesa payment model
func STKTransactionPB(db *STKTransaction) (*stk.StkTransaction, error) {
	if db == nil {
		return nil, errs.NilObject("stk payment")
	}

	pb := &stk.StkTransaction{
		InitiatorId:          db.InitiatorID,
		TransactionId:        fmt.Sprint(db.ID),
		MerchantRequestId:    db.MerchantRequestID,
		CheckoutRequestId:    db.CheckoutRequestID,
		ResultCode:           db.ResultCode,
		ResultDesc:           db.ResultDesc,
		Amount:               db.Amount,
		MpesaReceiptId:       db.MpesaReceiptId,
		Balance:              "",
		PhoneNumber:          db.PhoneNumber,
		Succeeded:            db.Succeeded == "YES",
		Processed:            db.Processed == "YES",
		TransactionTimestamp: db.TransactionTime.Time.UTC().Unix(),
		CreateTimestamp:      db.CreateTime.UTC().Unix(),
	}

	return pb, nil
}
