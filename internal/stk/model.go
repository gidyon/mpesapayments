package stk

import (
	"fmt"
	"time"

	"github.com/gidyon/micro/utils/errs"
	"github.com/gidyon/mpesapayments/pkg/api/stk"
)

// StkTable is table for mpesa payments
const StkTable = "mpesa_stk_results"

// PayloadStk contains mpesa transaction details
type PayloadStk struct {
	PayloadID          uint   `gorm:"primaryKey;autoIncrement"`
	MerchantRequestID  string `gorm:"type:varchar(50);not null"`
	CheckoutRequestID  string `gorm:"type:varchar(50);not null;unique"`
	ResultCode         string `gorm:"type:varchar(5);not null"`
	ResultDesc         string `gorm:"type:varchar(100);not null"`
	Amount             string `gorm:"type:float(10);not null"`
	MpesaReceiptNumber string `gorm:"type:varchar(50);unique"`
	TransactionDate    string `gorm:"type:varchar(50);not null"`
	PhoneNumber        string `gorm:"type:varchar(50);not null"`
	Succeeded          bool   `gorm:"type:tinyint(1)"`
	Processed          bool   `gorm:"type:tinyint(1)"`
	CreatedAt          time.Time
}

// TableName returns the name of the table
func (*PayloadStk) TableName() string {
	return StkTable
}

// GetStkPayloadDB converts protobuf mpesa message to MPESAPayloadStk
func GetStkPayloadDB(stkPayloadPB *stk.StkPayload) (*PayloadStk, error) {
	if stkPayloadPB == nil {
		return nil, errs.NilObject("mpesa payment")
	}

	stkPayloadDB := &PayloadStk{
		MerchantRequestID:  stkPayloadPB.MerchantRequestId,
		CheckoutRequestID:  stkPayloadPB.CheckoutRequestId,
		ResultCode:         stkPayloadPB.ResultCode,
		ResultDesc:         stkPayloadPB.ResultDesc,
		Amount:             stkPayloadPB.Amount,
		MpesaReceiptNumber: stkPayloadPB.MpesaReceiptNumber,
		TransactionDate:    stkPayloadPB.TransactionDate,
		PhoneNumber:        stkPayloadPB.PhoneNumber,
		Succeeded:          stkPayloadPB.Succeeded,
		Processed:          stkPayloadPB.Processed,
	}

	return stkPayloadDB, nil
}

// GetStkPayloadPB returns the protobuf message of mpesa payment model
func GetStkPayloadPB(stkPayloadDB *PayloadStk) (*stk.StkPayload, error) {
	if stkPayloadDB == nil {
		return nil, errs.NilObject("mpesa payment")
	}

	mpesaPB := &stk.StkPayload{
		PayloadId:          fmt.Sprint(stkPayloadDB.PayloadID),
		MerchantRequestId:  stkPayloadDB.MerchantRequestID,
		CheckoutRequestId:  stkPayloadDB.CheckoutRequestID,
		ResultCode:         stkPayloadDB.ResultCode,
		ResultDesc:         stkPayloadDB.ResultDesc,
		Amount:             stkPayloadDB.Amount,
		MpesaReceiptNumber: stkPayloadDB.MpesaReceiptNumber,
		TransactionDate:    stkPayloadDB.TransactionDate,
		PhoneNumber:        stkPayloadDB.PhoneNumber,
		Succeeded:          stkPayloadDB.Succeeded,
		Processed:          stkPayloadDB.Processed,
	}

	return mpesaPB, nil
}
