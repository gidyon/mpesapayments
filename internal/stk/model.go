package stk_model

import (
	"database/sql"
	"fmt"
	"os"
	"time"
)

func init() {
	tablePrefix = os.Getenv("STK_TABLE_PREFIX")
}

// STKTransaction contains mpesa stk transaction details
type STKTransaction struct {
	ID                            uint         `gorm:"primaryKey;autoIncrement"`
	InitiatorID                   string       `gorm:"index;type:varchar(50)"`
	InitiatorTransactionReference string       `gorm:"index;type:varchar(50)"`
	InitiatorCustomerReference    string       `gorm:"index;type:varchar(50)"`
	InitiatorCustomerNames        string       `gorm:"type:varchar(50)"`
	PhoneNumber                   string       `gorm:"index;type:varchar(15);not null"`
	Amount                        string       `gorm:"type:float(10);not null"`
	ShortCode                     string       `gorm:"index;type:varchar(15)"`
	AccountReference              string       `gorm:"index;type:varchar(50)"`
	TransactionDesc               string       `gorm:"type:varchar(300)"`
	MerchantRequestID             string       `gorm:"index;type:varchar(50);not null"`
	CheckoutRequestID             string       `gorm:"index;type:varchar(50);not null"`
	StkResponseDescription        string       `gorm:"type:varchar(300)"`
	StkResponseCustomerMessage    string       `gorm:"type:varchar(300)"`
	StkResponseCode               string       `gorm:"index;type:varchar(10)"`
	ResultCode                    string       `gorm:"index;type:varchar(10)"`
	StkResultCode                 string       `gorm:"index;type:varchar(10)"`
	StkResultDesc                 string       `gorm:"type:varchar(300)"`
	ResultDesc                    string       `gorm:"type:varchar(300)"`
	MpesaReceiptId                string       `gorm:"index;type:varchar(50);unique"`
	StkStatus                     string       `gorm:"index;type:varchar(30)"`
	Source                        string       `gorm:"index;type:varchar(30)"`
	Tag                           string       `gorm:"index;type:varchar(30)"`
	Succeeded                     string       `gorm:"index;type:enum('YES','NO');default:NO"`
	Processed                     string       `gorm:"index;type:enum('YES','NO');default:NO"`
	TransactionTime               sql.NullTime `gorm:"index;type:datetime(6)"`
	CreatedAt                     time.Time    `gorm:"primaryKey;autoCreateTime;->;<-:create;not null"`
}

// StkTable is table for mpesa payments
const StkTable = "stk_transactions"

var tablePrefix = ""

// TableName returns the name of the table
func (*STKTransaction) TableName() string {
	// Get table prefix
	if tablePrefix != "" {
		return fmt.Sprintf("%s_%s", tablePrefix, StkTable)
	}
	return StkTable
}
