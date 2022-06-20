package b2c

import (
	"database/sql"
	"fmt"
	"time"

	b2c_model "github.com/gidyon/mpesapayments/internal/b2c"
	b2c "github.com/gidyon/mpesapayments/pkg/api/b2c/v1"
)

// B2CPaymentDB is wrapper to converts database model to protobuf b2c payment
func B2CPaymentDB(pb *b2c.B2CPayment) (*b2c_model.Payment, error) {
	success := "NO"
	if pb.Succeeded {
		success = "YES"
	}
	process := "NO"
	if pb.Processed {
		process = "YES"
	}
	db := &b2c_model.Payment{
		ID:                            0,
		InitiatorID:                   pb.InitiatorId,
		Msisdn:                        pb.Msisdn,
		InitiatorTransactionReference: pb.TransactionReference,
		InitiatorCustomerReference:    pb.CustomerReference,
		InitiatorCustomerNames:        pb.CustomerNames,
		OrgShortCode:                  pb.OrgShortCode,
		ReceiverPublicName:            pb.ReceiverPartyPublicName,
		CommandId:                     pb.TransactionType,
		MpesaReceiptId:                pb.TransactionId,
		ConversationID:                pb.ConversationId,
		OriginatorConversationID:      pb.OriginatorConversationId,
		ResultCode:                    pb.ResultCode,
		ResultDescription:             pb.ResultDescription,
		TransactionTime: sql.NullTime{
			Valid: true,
			Time:  time.Unix(pb.TransactionTimestamp, 0),
		},
		Amount:              pb.Amount,
		WorkingAccountFunds: pb.WorkingAccountFunds,
		UtilityAccountFunds: pb.UtilityAccountFunds,
		MpesaCharges:        pb.MpesaCharges,
		OnfonCharges:        pb.OnfonCharges,
		RecipientRegistered: pb.RecipientRegistered,
		Succeeded:           success,
		Processed:           process,
		CreatedAt:           time.Unix(pb.TransactionTimestamp, 0),
	}
	return db, nil
}

// B2CPaymentPB is wrapper to converts protobuf b2c payment to database model
func B2CPaymentPB(db *b2c_model.Payment) (*b2c.B2CPayment, error) {
	pb := &b2c.B2CPayment{
		PaymentId:                fmt.Sprint(db.ID),
		InitiatorId:              db.InitiatorID,
		OrgShortCode:             db.OrgShortCode,
		Msisdn:                   db.Msisdn,
		TransactionReference:     db.InitiatorTransactionReference,
		CustomerReference:        db.InitiatorCustomerReference,
		CustomerNames:            db.InitiatorCustomerNames,
		ReceiverPartyPublicName:  db.ReceiverPublicName,
		TransactionType:          db.CommandId,
		TransactionId:            db.MpesaReceiptId,
		ConversationId:           db.ConversationID,
		OriginatorConversationId: db.OriginatorConversationID,
		ResultCode:               db.ResultCode,
		ResultDescription:        db.ResultDescription,
		TransactionTimestamp:     db.TransactionTime.Time.Unix(),
		Amount:                   db.Amount,
		WorkingAccountFunds:      db.WorkingAccountFunds,
		UtilityAccountFunds:      db.UtilityAccountFunds,
		MpesaCharges:             db.MpesaCharges,
		OnfonCharges:             db.OnfonCharges,
		RecipientRegistered:      db.RecipientRegistered,
		Succeeded:                db.Succeeded == "YES",
		Processed:                db.Processed == "YES",
		CreateDate:               db.CreatedAt.UTC().Format(time.RFC3339),
	}
	return pb, nil
}

// GetDailyStatDB gets mpesa statistics model from protobuf message
func GetDailyStatDB(pb *b2c.DailyStat) (*b2c_model.DailyStat, error) {
	return &b2c_model.DailyStat{
		OrgShortCode:           pb.OrgShortCode,
		Date:                   pb.Date,
		TotalTransactions:      pb.TotalTransactions,
		SuccessfulTransactions: int32(pb.SuccessfulTransactions),
		FailedTransactions:     int32(pb.FailedTransactions),
		TotalAmountTransacted:  pb.TotalAmountTransacted,
		TotalCharges:           pb.TotalCharges,
	}, nil
}

// GetDailyStatPB gets mpesa statistics protobuf from model
func GetDailyStatPB(db *b2c_model.DailyStat) (*b2c.DailyStat, error) {
	return &b2c.DailyStat{
		StatId:                 fmt.Sprint(db.ID),
		OrgShortCode:           db.OrgShortCode,
		TotalTransactions:      db.TotalTransactions,
		SuccessfulTransactions: int64(db.SuccessfulTransactions),
		FailedTransactions:     int64(db.FailedTransactions),
		TotalAmountTransacted:  db.TotalAmountTransacted,
		TotalCharges:           db.TotalCharges,
		CreateTimeSeconds:      db.CreatedAt.Unix(),
		UpdateTimeSeconds:      db.UpdatedAt.Unix(),
	}, nil
}
