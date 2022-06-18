package stk

import (
	"fmt"

	"github.com/gidyon/micro/v2/utils/errs"
	stk_model "github.com/gidyon/mpesapayments/internal/stk"
	stk "github.com/gidyon/mpesapayments/pkg/api/stk/v2"
)

// STKTransactionPB returns the protobuf message of mpesa payment model
func STKTransactionPB(db *stk_model.STKTransaction) (*stk.StkTransaction, error) {
	if db == nil {
		return nil, errs.NilObject("stk payment")
	}

	pb := &stk.StkTransaction{
		InitiatorId:                   db.InitiatorID,
		TransactionId:                 fmt.Sprint(db.ID),
		InitiatorTransactionReference: db.InitiatorTransactionReference,
		InitiatorCustomerReference:    db.InitiatorCustomerReference,
		InitiatorCustomerNames:        db.InitiatorCustomerNames,
		MerchantRequestId:             db.MerchantRequestID,
		CheckoutRequestId:             db.CheckoutRequestID,
		ShortCode:                     db.ShortCode,
		AccountReference:              db.AccountReference,
		TransactionDesc:               db.TransactionDesc,
		ResultCode:                    db.StkResultCode,
		ResultDesc:                    db.StkResultDesc,
		Amount:                        db.Amount,
		MpesaReceiptId:                db.MpesaReceiptId,
		Balance:                       "",
		PhoneNumber:                   db.PhoneNumber,
		Status:                        stk.StkStatus(stk.StkStatus_value[db.StkStatus]),
		Source:                        db.Source,
		Tag:                           db.Tag,
		Succeeded:                     db.Succeeded == "YES",
		Processed:                     db.Processed == "YES",
		TransactionTimestamp:          db.TransactionTime.Time.UTC().Unix(),
		CreateTimestamp:               db.CreatedAt.UTC().Unix(),
	}

	return pb, nil
}
