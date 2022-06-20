package stk

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/gidyon/micro/v2/utils/errs"
	stk_model "github.com/gidyon/mpesapayments/internal/stk"
	stk "github.com/gidyon/mpesapayments/pkg/api/stk/v1"
)

// STKTransactionModel converts protobuf mpesa message to MPESASTKTransaction
func STKTransactionModel(pb *stk.StkTransaction) (*stk_model.STKTransaction, error) {
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

	db := &stk_model.STKTransaction{
		ID:                            0,
		InitiatorID:                   pb.InitiatorId,
		InitiatorTransactionReference: "",
		InitiatorCustomerReference:    "",
		InitiatorCustomerNames:        "",
		PhoneNumber:                   pb.PhoneNumber,
		Amount:                        pb.Amount,
		ShortCode:                     pb.ShortCode,
		AccountReference:              pb.AccountReference,
		TransactionDesc:               pb.TransactionDesc,
		MerchantRequestID:             pb.MerchantRequestId,
		CheckoutRequestID:             pb.CheckoutRequestId,
		StkResponseDescription:        "",
		StkResponseCustomerMessage:    "",
		StkResponseCode:               "",
		ResultCode:                    pb.ResultCode,
		ResultDescription:             pb.ResultDesc,
		MpesaReceiptId:                pb.MpesaReceiptId,
		StkStatus:                     "",
		Source:                        "",
		Tag:                           "",
		Succeeded:                     success,
		Processed:                     processed,
		TransactionTime:               sql.NullTime{},
		CreatedAt:                     time.Time{},
	}

	if pb.TransactionTimestamp != 0 {
		db.TransactionTime = sql.NullTime{Valid: true, Time: time.Unix(pb.TransactionTimestamp, 0)}
	}

	return db, nil
}

// STKTransactionPB returns the protobuf message of mpesa payment model
func STKTransactionPB(db *stk_model.STKTransaction) (*stk.StkTransaction, error) {
	if db == nil {
		return nil, errs.NilObject("stk payment")
	}

	pb := &stk.StkTransaction{
		InitiatorId:          db.InitiatorID,
		TransactionId:        fmt.Sprint(db.ID),
		MerchantRequestId:    db.MerchantRequestID,
		CheckoutRequestId:    db.CheckoutRequestID,
		ShortCode:            db.ShortCode,
		AccountReference:     db.AccountReference,
		TransactionDesc:      db.TransactionDesc,
		ResultCode:           db.ResultCode,
		ResultDesc:           db.ResultDescription,
		Amount:               db.Amount,
		MpesaReceiptId:       db.MpesaReceiptId,
		Balance:              "",
		PhoneNumber:          db.PhoneNumber,
		Succeeded:            db.Succeeded == "YES",
		Processed:            db.Processed == "YES",
		TransactionTimestamp: db.TransactionTime.Time.UTC().Unix(),
		CreateTimestamp:      db.CreatedAt.UTC().Unix(),
	}

	return pb, nil
}
