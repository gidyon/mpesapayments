package b2c

import (
	"fmt"
	"os"

	"github.com/gidyon/mpesapayments/pkg/api/b2c"
)

// B2CTable is table name for b2c transactions
const B2CTable = "b2c_transactions"

// Payment is B2C payment model
type Payment struct {
	PaymentID                uint    `gorm:"primaryKey;autoIncrement"`
	InitiatorID              string  `gorm:"index;type:varchar(50)"`
	Msisdn                   string  `gorm:"index;type:varchar(15)"`
	OrgShortCode             string  `gorm:"index;type:varchar(15)"`
	ReceiverPublicName       string  `gorm:"type:varchar(50)"`
	TransactionType          string  `gorm:"type:varchar(50)"`
	TransactionID            string  `gorm:"index;type:varchar(50);unique"`
	ConversationID           string  `gorm:"type:varchar(50)"`
	OriginatorConversationID string  `gorm:"type:varchar(50)"`
	ResultCode               string  `gorm:"type:varchar(2)"`
	ResultDescription        string  `gorm:"type:varchar(100)"`
	TransactionTimestamp     int64   `gorm:"type:int"`
	CreateTimestamp          int64   `gorm:"type:int;autoCreateTime"`
	Amount                   float32 `gorm:"type:float(10)"`
	WorkingAccountFunds      float32 `gorm:"type:float(10)"`
	UtilityAccountFunds      float32 `gorm:"type:float(10)"`
	ChargesPaidFunds         float32 `gorm:"type:float(10)"`
	RecipientRegistered      bool    `gorm:"type:tinyint(1)"`
	Succeeded                bool    `gorm:"type:tinyint(1)"`
	Processed                bool    `gorm:"type:tinyint(1)"`
}

// TableName is table name for model
func (*Payment) TableName() string {
	// Get table prefix
	prefix := os.Getenv("TABLE_PREFIX")
	if prefix != "" {
		return fmt.Sprintf("%s_%s", prefix, B2CTable)
	}
	return B2CTable
}

// GetB2CPaymentDB is wrapper to converts database model to protobuf b2c payment
func GetB2CPaymentDB(paymentPB *b2c.B2CPayment) (*Payment, error) {
	paymentDB := &Payment{
		InitiatorID:              paymentPB.InitiatorId,
		Msisdn:                   paymentPB.Msisdn,
		OrgShortCode:             paymentPB.OrgShortCode,
		ReceiverPublicName:       paymentPB.ReceiverPartyPublicName,
		TransactionType:          paymentPB.TransactionType,
		TransactionID:            paymentPB.TransactionId,
		ConversationID:           paymentPB.ConversationId,
		OriginatorConversationID: paymentPB.OriginatorConversationId,
		ResultCode:               paymentPB.ResultCode,
		ResultDescription:        paymentPB.ResultDescription,
		TransactionTimestamp:     paymentPB.TransactionTimestamp,
		Amount:                   paymentPB.Amount,
		WorkingAccountFunds:      paymentPB.WorkingAccountFunds,
		UtilityAccountFunds:      paymentPB.UtilityAccountFunds,
		ChargesPaidFunds:         paymentPB.ChargesPaidFunds,
		RecipientRegistered:      paymentPB.RecipientRegistered,
		Succeeded:                paymentPB.Succeeded,
		Processed:                paymentPB.Processed,
	}
	return paymentDB, nil
}

// GetB2CPaymentPB is wrapper to converts protobuf b2c payment to database model
func GetB2CPaymentPB(paymentDB *Payment) (*b2c.B2CPayment, error) {
	paymentPB := &b2c.B2CPayment{
		PaymentId:                fmt.Sprint(paymentDB.PaymentID),
		InitiatorId:              paymentDB.InitiatorID,
		Msisdn:                   paymentDB.Msisdn,
		OrgShortCode:             paymentDB.OrgShortCode,
		ReceiverPartyPublicName:  paymentDB.ReceiverPublicName,
		TransactionType:          paymentDB.TransactionType,
		TransactionId:            paymentDB.TransactionID,
		ConversationId:           paymentDB.ConversationID,
		OriginatorConversationId: paymentDB.OriginatorConversationID,
		ResultCode:               paymentDB.ResultCode,
		ResultDescription:        paymentDB.ResultDescription,
		TransactionTimestamp:     paymentDB.TransactionTimestamp,
		CreateTimestamp:          paymentDB.CreateTimestamp,
		Amount:                   paymentDB.Amount,
		WorkingAccountFunds:      paymentDB.WorkingAccountFunds,
		UtilityAccountFunds:      paymentDB.UtilityAccountFunds,
		ChargesPaidFunds:         paymentDB.ChargesPaidFunds,
		RecipientRegistered:      paymentDB.RecipientRegistered,
		Succeeded:                paymentDB.Succeeded,
		Processed:                paymentDB.Processed,
	}
	return paymentPB, nil
}
