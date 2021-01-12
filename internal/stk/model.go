package stk

import (
	"fmt"
	"os"

	"github.com/gidyon/micro/v2/utils/errs"
	"github.com/gidyon/mpesapayments/pkg/api/stk"
)

// StkTable is table for mpesa payments
const StkTable = "stks_mpesa"

// PayloadStk contains mpesa transaction details
type PayloadStk struct {
	PayloadID            uint   `gorm:"primaryKey;autoIncrement"`
	InitiatorID          string `gorm:"type:varchar(50);not null"`
	MerchantRequestID    string `gorm:"type:varchar(50);not null"`
	CheckoutRequestID    string `gorm:"type:varchar(50);not null;unique"`
	ResultCode           string `gorm:"type:varchar(5);not null"`
	ResultDesc           string `gorm:"type:varchar(100);not null"`
	Amount               string `gorm:"type:float(10);not null"`
	TransactionID        string `gorm:"type:varchar(50);unique"`
	PhoneNumber          string `gorm:"type:varchar(50);not null"`
	Succeeded            bool   `gorm:"type:tinyint(1)"`
	Processed            bool   `gorm:"type:tinyint(1)"`
	TransactionTimestamp int64  `gorm:"type:int(15);not null"`
	CreateTimestamp      int64  `gorm:"autoCreateTime"`
}

// TableName returns the name of the table
func (*PayloadStk) TableName() string {
	// Get table prefix
	prefix := os.Getenv("TABLE_PREFIX")
	if prefix != "" {
		return fmt.Sprintf("%s-%s", prefix, StkTable)
	}
	return StkTable
}

// GetStkPayloadDB converts protobuf mpesa message to MPESAPayloadStk
func GetStkPayloadDB(stkPayloadPB *stk.StkPayload) (*PayloadStk, error) {
	if stkPayloadPB == nil {
		return nil, errs.NilObject("mpesa payment")
	}

	stkPayloadDB := &PayloadStk{
		InitiatorID:          stkPayloadPB.InitiatorId,
		MerchantRequestID:    stkPayloadPB.MerchantRequestId,
		CheckoutRequestID:    stkPayloadPB.CheckoutRequestId,
		ResultCode:           stkPayloadPB.ResultCode,
		ResultDesc:           stkPayloadPB.ResultDesc,
		Amount:               stkPayloadPB.Amount,
		TransactionID:        stkPayloadPB.TransactionId,
		TransactionTimestamp: stkPayloadPB.TransactionTimestamp,
		PhoneNumber:          stkPayloadPB.PhoneNumber,
		Succeeded:            stkPayloadPB.Succeeded,
		Processed:            stkPayloadPB.Processed,
	}

	return stkPayloadDB, nil
}

// GetStkPayloadPB returns the protobuf message of mpesa payment model
func GetStkPayloadPB(stkPayloadDB *PayloadStk) (*stk.StkPayload, error) {
	if stkPayloadDB == nil {
		return nil, errs.NilObject("mpesa payment")
	}

	mpesaPB := &stk.StkPayload{
		PayloadId:            fmt.Sprint(stkPayloadDB.PayloadID),
		InitiatorId:          stkPayloadDB.InitiatorID,
		MerchantRequestId:    stkPayloadDB.MerchantRequestID,
		CheckoutRequestId:    stkPayloadDB.CheckoutRequestID,
		ResultCode:           stkPayloadDB.ResultCode,
		ResultDesc:           stkPayloadDB.ResultDesc,
		Amount:               stkPayloadDB.Amount,
		TransactionId:        stkPayloadDB.TransactionID,
		TransactionTimestamp: stkPayloadDB.TransactionTimestamp,
		PhoneNumber:          stkPayloadDB.PhoneNumber,
		Succeeded:            stkPayloadDB.Succeeded,
		Processed:            stkPayloadDB.Processed,
		CreateTimestamp:      stkPayloadDB.CreateTimestamp,
	}

	return mpesaPB, nil
}
