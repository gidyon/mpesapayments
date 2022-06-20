package b2c

import (
	"fmt"
	"time"

	b2c_model "github.com/gidyon/mpesapayments/internal/b2c"
	b2c "github.com/gidyon/mpesapayments/pkg/api/b2c/v2"
	"gorm.io/gorm"
)

func B2CPaymentPB(db *b2c_model.Payment) (*b2c.B2CPayment, error) {
	pb := &b2c.B2CPayment{
		InitiatorId:                   db.InitiatorID,
		TransactionId:                 fmt.Sprint(db.ID),
		InitiatorTransactionReference: db.InitiatorTransactionReference,
		InitiatorCustomerReference:    db.InitiatorCustomerReference,
		InitiatorCustomerNames:        db.InitiatorCustomerNames,
		OrgShortCode:                  db.OrgShortCode,
		CommandId:                     b2c.CommandId(b2c.CommandId_value[db.CommandId]),
		Msisdn:                        db.Msisdn,
		Amount:                        db.Amount,
		ConversationId:                db.ConversationID,
		OriginalConversationId:        db.OriginatorConversationID,
		B2CResponseDescription:        db.ResponseDescription,
		B2CResponseCode:               db.ResponseCode,
		B2CResultDescription:          db.ResultDescription,
		B2CResultCode:                 db.ResultCode,
		ReceiverPartyPublicName:       db.ReceiverPublicName,
		MpesaReceiptId:                db.MpesaReceiptId,
		WorkingAccountFunds:           db.WorkingAccountFunds,
		UtilityAccountFunds:           db.UtilityAccountFunds,
		MpesaCharges:                  db.MpesaCharges,
		OnfonCharges:                  db.OnfonCharges,
		RecipientRegistered:           db.RecipientRegistered,
		B2CStatus:                     b2c.B2CStatus(b2c.B2CStatus_value[db.Status]),
		Source:                        db.Source,
		Tag:                           db.Tag,
		Succeeded:                     db.Succeeded == "YES",
		Processed:                     db.Processed == "YES",
		TransactionTimestamp:          db.TransactionTime.Time.UTC().Unix(),
		CreateDate:                    db.CreatedAt.UTC().Format(time.RFC3339),
	}
	return pb, nil
}

// GetDailyStatDB gets mpesa statistics model from protobuf message
func GetDailyStatDB(pb *b2c.DailyStat) (*b2c_model.DailyStat, error) {
	return &b2c_model.DailyStat{
		ID:                     0,
		OrgShortCode:           pb.OrgShortCode,
		Date:                   pb.Date,
		TotalTransactions:      pb.TotalTransactions,
		SuccessfulTransactions: int32(pb.SuccessfulTransactions),
		FailedTransactions:     int32(pb.FailedTransactions),
		TotalAmountTransacted:  pb.TotalAmountTransacted,
		TotalCharges:           pb.TotalCharges,
		CreatedAt:              time.Time{},
		UpdatedAt:              time.Time{},
		DeletedAt:              gorm.DeletedAt{},
	}, nil
}

// GetDailyStatPB gets mpesa statistics protobuf from model
func GetDailyStatPB(db *b2c_model.DailyStat) (*b2c.DailyStat, error) {
	return &b2c.DailyStat{
		StatId:                 fmt.Sprint(db.ID),
		Date:                   "",
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
