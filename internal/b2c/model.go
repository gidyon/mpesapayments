package b2c

import (
	"fmt"
	"os"
	"time"

	"github.com/gidyon/mpesapayments/pkg/api/b2c"
	"gorm.io/gorm"
)

// B2CTable is table name for b2c transactions
const B2CTable = "b2c_transactions"

// Payment is B2C payment model
type Payment struct {
	PaymentID                uint      `gorm:"primaryKey;autoIncrement"`
	InitiatorID              string    `gorm:"index;type:varchar(50)"`
	Msisdn                   string    `gorm:"index;type:varchar(15)"`
	OrgShortCode             string    `gorm:"index;type:varchar(15)"`
	ReceiverPublicName       string    `gorm:"type:varchar(50)"`
	TransactionType          string    `gorm:"type:varchar(50)"`
	TransactionID            string    `gorm:"index;type:varchar(50);unique"`
	ConversationID           string    `gorm:"type:varchar(50)"`
	OriginatorConversationID string    `gorm:"type:varchar(50)"`
	ResultCode               string    `gorm:"type:varchar(2)"`
	ResultDescription        string    `gorm:"type:varchar(100)"`
	TransactionTime          time.Time `gorm:"autoCreateTime"`
	CreateAt                 time.Time `gorm:"primaryKey;autoCreateTime;->;<-:create;not null"`
	Amount                   float32   `gorm:"index;type:float(10)"`
	WorkingAccountFunds      float32   `gorm:"type:float(10)"`
	UtilityAccountFunds      float32   `gorm:"type:float(10)"`
	MpesaCharges             float32   `gorm:"type:float(10)"`
	OnfonCharges             float32   `gorm:"type:float(10)"`
	RecipientRegistered      bool      `gorm:"type:tinyint(1)"`
	Succeeded                bool      `gorm:"type:tinyint(1)"`
	Processed                bool      `gorm:"type:tinyint(1)"`
}

// TableName is table name for model
func (*Payment) TableName() string {
	table := os.Getenv("B2C_TRANSACTIONS_TABLE")
	if table != "" {
		return table
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
		CreateAt:                 time.Unix(paymentPB.TransactionTimestamp, 0),
		Amount:                   paymentPB.Amount,
		WorkingAccountFunds:      paymentPB.WorkingAccountFunds,
		UtilityAccountFunds:      paymentPB.UtilityAccountFunds,
		MpesaCharges:             paymentPB.MpesaCharges,
		OnfonCharges:             paymentPB.OnfonCharges,
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
		TransactionTimestamp:     paymentDB.TransactionTime.Unix(),
		CreateTimestamp:          paymentDB.CreateAt.Unix(),
		Amount:                   paymentDB.Amount,
		WorkingAccountFunds:      paymentDB.WorkingAccountFunds,
		UtilityAccountFunds:      paymentDB.UtilityAccountFunds,
		MpesaCharges:             paymentDB.MpesaCharges,
		OnfonCharges:             paymentDB.OnfonCharges,
		RecipientRegistered:      paymentDB.RecipientRegistered,
		Succeeded:                paymentDB.Succeeded,
		Processed:                paymentDB.Processed,
	}
	return paymentPB, nil
}

const statsTable = "b2c_daily_stats"

// DailyStat contains statistics for a day
type DailyStat struct {
	ID                     uint   `gorm:"primaryKey;autoIncrement"`
	OrgShortCode           string `gorm:"index;type:varchar(20);not null"`
	Date                   string `gorm:"index;type:varchar(10);not null"`
	TotalTransactions      int32  `gorm:"type:int(10);not null"`
	SuccessfulTransactions int32
	FailedTransactions     int32
	TotalAmountTransacted  float32        `gorm:"index;type:float(15)"`
	TotalCharges           float32        `gorm:"index;type:float(15)"`
	CreatedAt              time.Time      `gorm:"autoCreateTime"`
	UpdatedAt              time.Time      `gorm:"autoCreateTime"`
	DeletedAt              gorm.DeletedAt `gorm:"index"`
}

// TableName ...
func (*DailyStat) TableName() string {
	// Get table prefix
	table := os.Getenv("B2C_STATS_TABLE")
	if table != "" {
		return table
	}
	return statsTable
}

// GetDailyStatDB gets mpesa statistics model from protobuf message
func GetDailyStatDB(statPB *b2c.DailyStat) (*DailyStat, error) {
	return &DailyStat{
		OrgShortCode:           statPB.OrgShortCode,
		Date:                   statPB.Date,
		TotalTransactions:      statPB.TotalTransactions,
		SuccessfulTransactions: int32(statPB.SuccessfulTransactions),
		FailedTransactions:     int32(statPB.FailedTransactions),
		TotalAmountTransacted:  statPB.TotalAmountTransacted,
		TotalCharges:           statPB.TotalCharges,
	}, nil
}

// GetDailyStatPB gets mpesa statistics protobuf from model
func GetDailyStatPB(statDB *DailyStat) (*b2c.DailyStat, error) {
	return &b2c.DailyStat{
		StatId:                 fmt.Sprint(statDB.ID),
		OrgShortCode:           statDB.OrgShortCode,
		TotalTransactions:      statDB.TotalTransactions,
		SuccessfulTransactions: int64(statDB.SuccessfulTransactions),
		FailedTransactions:     int64(statDB.FailedTransactions),
		TotalAmountTransacted:  statDB.TotalAmountTransacted,
		TotalCharges:           statDB.TotalCharges,
		CreateTimeSeconds:      statDB.CreatedAt.Unix(),
		UpdateTimeSeconds:      statDB.UpdatedAt.Unix(),
	}, nil
}
