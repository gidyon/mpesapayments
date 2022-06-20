package b2c_model

import (
	"database/sql"
	"os"
	"time"

	"gorm.io/gorm"
)

func init() {
	b2cTable = os.Getenv("B2C_TRANSACTIONS_TABLE")
	b2cStatsTable = os.Getenv("B2C_STATS_TABLE")
}

// B2CTable is table name for b2c transactions
const B2CTable = "b2c_transactions"

var b2cTable = ""

// Payment is B2C payment model
type Payment struct {
	ID                            uint   `gorm:"primaryKey;autoIncrement"`
	InitiatorID                   string `gorm:"index;type:varchar(50)"`
	InitiatorTransactionReference string `gorm:"index;type:varchar(50)"`
	InitiatorCustomerReference    string `gorm:"index;type:varchar(50)"`
	InitiatorCustomerNames        string `gorm:"type:varchar(50)"`

	Msisdn       string  `gorm:"index;type:varchar(15)"`
	OrgShortCode string  `gorm:"index;type:varchar(15)"`
	CommandId    string  `gorm:"index;type:varchar(30)"`
	Amount       float32 `gorm:"index;type:float(10)"`

	ConversationID           string `gorm:"index;type:varchar(50);not null"`
	OriginatorConversationID string `gorm:"index;type:varchar(50);not null"`
	ResponseDescription      string `gorm:"type:varchar(300)"`
	ResponseCode             string `gorm:"index;type:varchar(10)"`
	ResultCode               string `gorm:"index;type:varchar(10)"`
	ResultDescription        string `gorm:"type:varchar(300)"`

	WorkingAccountFunds float32 `gorm:"type:float(10)"`
	UtilityAccountFunds float32 `gorm:"type:float(10)"`
	MpesaCharges        float32 `gorm:"type:float(10)"`
	OnfonCharges        float32 `gorm:"type:float(10)"`
	RecipientRegistered bool    `gorm:"index;type:tinyint(1)"`
	MpesaReceiptId      string  `gorm:"index;type:varchar(50);unique"`
	ReceiverPublicName  string  `gorm:"type:varchar(50)"`

	Status    string `gorm:"index;type:varchar(30)"`
	Source    string `gorm:"index;type:varchar(30)"`
	Tag       string `gorm:"index;type:varchar(30)"`
	Succeeded string `gorm:"index;type:enum('YES','NO', 'UNKNOWN');default:NO"`
	Processed string `gorm:"index;type:enum('YES','NO');default:NO"`

	TransactionTime sql.NullTime `gorm:"index;type:datetime(6)"`
	UpdatedAt       time.Time    `gorm:"autoUpdateTime:nano;type:datetime(6)"`
	CreatedAt       time.Time    `gorm:"index;autoCreateTime:nano;type:datetime(6);not null"`
}

// TableName is table name for model
func (*Payment) TableName() string {
	if b2cTable != "" {
		return b2cTable
	}
	return B2CTable
}

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

const statsTable = "b2c_daily_stats"

var b2cStatsTable = ""

// TableName ...
func (*DailyStat) TableName() string {
	if b2cStatsTable != "" {
		return b2cStatsTable
	}
	return statsTable
}
