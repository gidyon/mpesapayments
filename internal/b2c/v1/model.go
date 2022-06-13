package b2c

import (
	"fmt"
	"time"

	b2c "github.com/gidyon/mpesapayments/pkg/api/b2c/v1"

	"gorm.io/gorm"
)

// B2CTable is table name for b2c transactions
const B2CTable = "b2c_transactions"

var b2cTable = ""

// Payment is B2C payment model
type Payment struct {
	ID                       uint      `gorm:"primaryKey;autoIncrement"`
	InitiatorID              string    `gorm:"index;type:varchar(50)"`
	Msisdn                   string    `gorm:"index;type:varchar(15)"`
	TransactionReference     string    `gorm:"index;type:varchar(50)"`
	CustomerReference        string    `gorm:"index;type:varchar(50)"`
	CustomerNames            string    `gorm:"type:varchar(50)"`
	OrgShortCode             string    `gorm:"index;type:varchar(15)"`
	ReceiverPublicName       string    `gorm:"type:varchar(50)"`
	TransactionType          string    `gorm:"index;type:varchar(50)"`
	TransactionID            string    `gorm:"index;type:varchar(50);unique"`
	ConversationID           string    `gorm:"type:varchar(50)"`
	OriginatorConversationID string    `gorm:"type:varchar(50)"`
	ResultCode               string    `gorm:"index;type:varchar(5)"`
	ResultDescription        string    `gorm:"type:varchar(100)"`
	TransactionTime          time.Time `gorm:"index:;type:datetime(6)"`
	Amount                   float32   `gorm:"index;type:float(10)"`
	WorkingAccountFunds      float32   `gorm:"type:float(10)"`
	UtilityAccountFunds      float32   `gorm:"type:float(10)"`
	MpesaCharges             float32   `gorm:"type:float(10)"`
	OnfonCharges             float32   `gorm:"type:float(10)"`
	RecipientRegistered      bool      `gorm:"index;type:tinyint(1)"`
	Succeeded                bool      `gorm:"index;type:tinyint(1)"`
	Processed                bool      `gorm:"index;type:tinyint(1)"`
	CreatedAt                time.Time `gorm:"primaryKey;autoCreateTime;->;<-:create;not null"`
}

// TableName is table name for model
func (*Payment) TableName() string {
	if b2cTable != "" {
		return b2cTable
	}
	return B2CTable
}

// B2CPaymentDB is wrapper to converts database model to protobuf b2c payment
func B2CPaymentDB(pb *b2c.B2CPayment) (*Payment, error) {
	db := &Payment{
		ID:                       0,
		InitiatorID:              pb.InitiatorId,
		Msisdn:                   pb.Msisdn,
		TransactionReference:     pb.TransactionReference,
		CustomerReference:        pb.CustomerReference,
		CustomerNames:            pb.CustomerNames,
		OrgShortCode:             pb.OrgShortCode,
		ReceiverPublicName:       pb.ReceiverPartyPublicName,
		TransactionType:          pb.TransactionType,
		TransactionID:            pb.TransactionId,
		ConversationID:           pb.ConversationId,
		OriginatorConversationID: pb.OriginatorConversationId,
		ResultCode:               pb.ResultCode,
		ResultDescription:        pb.ResultDescription,
		TransactionTime:          time.Unix(pb.TransactionTimestamp, 0),
		Amount:                   pb.Amount,
		WorkingAccountFunds:      pb.WorkingAccountFunds,
		UtilityAccountFunds:      pb.UtilityAccountFunds,
		MpesaCharges:             pb.MpesaCharges,
		OnfonCharges:             pb.OnfonCharges,
		RecipientRegistered:      pb.RecipientRegistered,
		Succeeded:                pb.Succeeded,
		Processed:                pb.Processed,
		CreatedAt:                time.Unix(pb.TransactionTimestamp, 0),
	}
	return db, nil
}

// B2CPaymentPB is wrapper to converts protobuf b2c payment to database model
func B2CPaymentPB(db *Payment) (*b2c.B2CPayment, error) {
	pb := &b2c.B2CPayment{
		PaymentId:                fmt.Sprint(db.ID),
		InitiatorId:              db.InitiatorID,
		OrgShortCode:             db.OrgShortCode,
		Msisdn:                   db.Msisdn,
		TransactionReference:     db.TransactionReference,
		CustomerReference:        db.CustomerReference,
		CustomerNames:            db.CustomerNames,
		ReceiverPartyPublicName:  db.ReceiverPublicName,
		TransactionType:          db.TransactionType,
		TransactionId:            db.TransactionID,
		ConversationId:           db.ConversationID,
		OriginatorConversationId: db.OriginatorConversationID,
		ResultCode:               db.ResultCode,
		ResultDescription:        db.ResultDescription,
		TransactionTimestamp:     db.TransactionTime.Unix(),
		Amount:                   db.Amount,
		WorkingAccountFunds:      db.WorkingAccountFunds,
		UtilityAccountFunds:      db.UtilityAccountFunds,
		MpesaCharges:             db.MpesaCharges,
		OnfonCharges:             db.OnfonCharges,
		RecipientRegistered:      db.RecipientRegistered,
		Succeeded:                db.Succeeded,
		Processed:                db.Processed,
		CreateDate:               db.CreatedAt.UTC().Format(time.RFC3339),
	}
	return pb, nil
}

const statsTable = "b2c_daily_stats"

var btcStatsTable = ""

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
	if btcStatsTable != "" {
		return btcStatsTable
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
