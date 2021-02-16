package c2b

import (
	"fmt"
	"os"
	"time"

	"github.com/gidyon/micro/v2/utils/errs"
	"github.com/gidyon/mpesapayments/pkg/api/c2b"
	"gorm.io/gorm"
)

// C2Bs is table for mpesa payments
const C2Bs = "mawingu_payments_mpesa_v2"

// PaymentMpesa contains mpesa transaction details
type PaymentMpesa struct {
	PaymentID         uint      `gorm:"primaryKey;autoIncrement"`
	TransactionType   string    `gorm:"type:varchar(50);not null"`
	TransactionID     string    `gorm:"type:varchar(50);not null;unique"`
	MSISDN            string    `gorm:"index;type:varchar(15);not null"`
	Names             string    `gorm:"type:varchar(50)"`
	ReferenceNumber   string    `gorm:"index;type:varchar(20)"`
	Amount            float32   `gorm:"type:float(10);not null"`
	OrgAccountBalance float32   `gorm:"type:float(10)"`
	BusinessShortCode int32     `gorm:"index;type:varchar(10);not null"`
	TransactionTime   time.Time `gorm:"autoCreateTime"`
	CreateTime        time.Time `gorm:"autoCreateTime"`
	Processed         bool      `gorm:"type:tinyint(1);not null"`
}

// TableName returns the name of the table
func (*PaymentMpesa) TableName() string {
	// Get table prefix
	prefix := os.Getenv("TABLE_PREFIX")
	if prefix != "" {
		return fmt.Sprintf("%s_%s", prefix, C2Bs)
	}
	return C2Bs
}

// GetMpesaDB converts protobuf mpesa message to C2BMpesa
func GetMpesaDB(MpesaPB *c2b.C2BPayment) (*PaymentMpesa, error) {
	if MpesaPB == nil {
		return nil, errs.NilObject("mpesa payment")
	}

	mpesaDB := &PaymentMpesa{
		TransactionID:     MpesaPB.TransactionId,
		TransactionType:   MpesaPB.TransactionType,
		TransactionTime:   time.Unix(MpesaPB.TransactionTimeSeconds, 0),
		MSISDN:            MpesaPB.Msisdn,
		Names:             MpesaPB.Names,
		ReferenceNumber:   MpesaPB.RefNumber,
		Amount:            MpesaPB.Amount,
		OrgAccountBalance: MpesaPB.OrgBalance,
		BusinessShortCode: MpesaPB.BusinessShortCode,
		Processed:         MpesaPB.Processed,
	}

	return mpesaDB, nil
}

// GetMpesaPB returns the protobuf message of mpesa payment model
func GetMpesaPB(MpesaDB *PaymentMpesa) (*c2b.C2BPayment, error) {
	if MpesaDB == nil {
		return nil, errs.NilObject("mpesa payment")
	}

	mpesaPB := &c2b.C2BPayment{
		PaymentId:              fmt.Sprint(MpesaDB.PaymentID),
		TransactionType:        MpesaDB.TransactionType,
		TransactionId:          MpesaDB.TransactionID,
		TransactionTimeSeconds: MpesaDB.TransactionTime.Unix(),
		Msisdn:                 MpesaDB.MSISDN,
		Names:                  MpesaDB.Names,
		RefNumber:              MpesaDB.ReferenceNumber,
		Amount:                 MpesaDB.Amount,
		OrgBalance:             MpesaDB.OrgAccountBalance,
		BusinessShortCode:      MpesaDB.BusinessShortCode,
		Processed:              MpesaDB.Processed,
		CreateTimeSeconds:      MpesaDB.CreateTime.Unix(),
	}

	return mpesaPB, nil
}

const statsTable = "daily_stats"

// Stat is model containing transactions statistic
type Stat struct {
	StatID            uint           `gorm:"primaryKey;autoIncrement"`
	ShortCode         string         `gorm:"index;type:varchar(20);not null"`
	AccountName       string         `gorm:"index;type:varchar(20);not null"`
	Date              string         `gorm:"index;type:varchar(10);not null"`
	TotalTransactions int32          `gorm:"type:int(10);not null"`
	TotalAmount       float32        `gorm:"type:float(12);not null"`
	CreatedAt         time.Time      `gorm:"autoCreateTime"`
	UpdatedAt         time.Time      `gorm:"autoCreateTime"`
	DeletedAt         gorm.DeletedAt `gorm:"index"`
}

// TableName ...
func (*Stat) TableName() string {
	// Get table prefix
	prefix := os.Getenv("TABLE_PREFIX")
	if prefix != "" {
		return fmt.Sprintf("%s_%s", prefix, statsTable)
	}
	return statsTable
}

// GetStatDB gets mpesa statistics model from protobuf message
func GetStatDB(statPB *c2b.Stat) (*Stat, error) {
	return &Stat{
		ShortCode:         statPB.ShortCode,
		AccountName:       statPB.AccountName,
		Date:              statPB.ShortCode,
		TotalTransactions: statPB.TotalTransactions,
		TotalAmount:       statPB.TotalAmount,
	}, nil
}

// GetStatPB gets mpesa statistics protobuf from model
func GetStatPB(statDB *Stat) (*c2b.Stat, error) {
	return &c2b.Stat{
		StatId:            fmt.Sprint(statDB.StatID),
		Date:              statDB.ShortCode,
		ShortCode:         statDB.ShortCode,
		AccountName:       statDB.AccountName,
		TotalTransactions: statDB.TotalTransactions,
		TotalAmount:       statDB.TotalAmount,
		CreateDateSeconds: statDB.CreatedAt.Unix(),
		UpdateTimeSeconds: statDB.UpdatedAt.Unix(),
	}, nil
}

// Scopes is model user scopes
type Scopes struct {
	UserID    string         `gorm:"primaryKey;type:varchar(15)"`
	Scopes    []byte         `gorm:"type:json"`
	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoCreateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName ...
func (*Scopes) TableName() string {
	return "scopes"
}
