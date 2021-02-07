package mpesapayment

import (
	"fmt"
	"os"
	"time"

	"github.com/gidyon/micro/v2/utils/errs"
	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"gorm.io/gorm"
)

// MpesaPayments is table for mpesa payments
const MpesaPayments = "payments_mpesa"

// PaymentMpesa contains mpesa transaction details
type PaymentMpesa struct {
	PaymentID            uint    `gorm:"primaryKey;autoIncrement"`
	TransactionType      string  `gorm:"type:varchar(50);not null"`
	TransactionID        string  `gorm:"type:varchar(50);not null;unique"`
	MSISDN               string  `gorm:"index;type:varchar(15);not null"`
	Names                string  `gorm:"type:varchar(50)"`
	ReferenceNumber      string  `gorm:"index;type:varchar(20)"`
	Amount               float32 `gorm:"type:float(10);not null"`
	OrgAccountBalance    float32 `gorm:"type:float(10)"`
	BusinessShortCode    int32   `gorm:"index;type:varchar(10);not null"`
	TransactionTimestamp int64   `gorm:"type:int(15);not null"`
	CreateTimesatamp     int64   `gorm:"autoCreateTime"`
	Processed            bool    `gorm:"type:tinyint(1);not null"`
}

// TableName returns the name of the table
func (*PaymentMpesa) TableName() string {
	// Get table prefix
	prefix := os.Getenv("TABLE_PREFIX")
	if prefix != "" {
		return fmt.Sprintf("%s_%s", prefix, MpesaPayments)
	}
	return MpesaPayments
}

// GetMpesaDB converts protobuf mpesa message to MPESAPaymentMpesa
func GetMpesaDB(MpesaPB *mpesapayment.MPESAPayment) (*PaymentMpesa, error) {
	if MpesaPB == nil {
		return nil, errs.NilObject("mpesa payment")
	}

	mpesaDB := &PaymentMpesa{
		TransactionID:        MpesaPB.TransactionId,
		TransactionType:      MpesaPB.TransactionType,
		TransactionTimestamp: MpesaPB.TransactionTimestamp,
		MSISDN:               MpesaPB.Msisdn,
		Names:                MpesaPB.Names,
		ReferenceNumber:      MpesaPB.RefNumber,
		Amount:               MpesaPB.Amount,
		OrgAccountBalance:    MpesaPB.OrgBalance,
		BusinessShortCode:    MpesaPB.BusinessShortCode,
		Processed:            MpesaPB.Processed,
	}

	return mpesaDB, nil
}

// GetMpesaPB returns the protobuf message of mpesa payment model
func GetMpesaPB(MpesaDB *PaymentMpesa) (*mpesapayment.MPESAPayment, error) {
	if MpesaDB == nil {
		return nil, errs.NilObject("mpesa payment")
	}

	mpesaPB := &mpesapayment.MPESAPayment{
		PaymentId:            fmt.Sprint(MpesaDB.PaymentID),
		TransactionType:      MpesaDB.TransactionType,
		TransactionId:        MpesaDB.TransactionID,
		TransactionTimestamp: MpesaDB.TransactionTimestamp,
		Msisdn:               MpesaDB.MSISDN,
		Names:                MpesaDB.Names,
		RefNumber:            MpesaDB.ReferenceNumber,
		Amount:               MpesaDB.Amount,
		OrgBalance:           MpesaDB.OrgAccountBalance,
		BusinessShortCode:    MpesaDB.BusinessShortCode,
		Processed:            MpesaDB.Processed,
		CreateTimestamp:      MpesaDB.CreateTimesatamp,
	}

	return mpesaPB, nil
}

const statsTable = "daily_stats"

// Stat is model containing transactions statistic
type Stat struct {
	StatID            uint    `gorm:"primaryKey;autoIncrement"`
	ShortCode         string  `gorm:"index;type:varchar(20);not null"`
	AccountName       string  `gorm:"index;type:varchar(20);not null"`
	Date              string  `gorm:"index;type:varchar(10);not null"`
	TotalTransactions int32   `gorm:"type:int(10);not null"`
	TotalAmount       float32 `gorm:"type:float(12);not null"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         gorm.DeletedAt `gorm:"index"`
}

// TableName ...
func (*Stat) TableName() string {
	return statsTable
}

// GetStatDB gets mpesa statistics model from protobuf message
func GetStatDB(statPB *mpesapayment.Stat) (*Stat, error) {
	return &Stat{
		ShortCode:         statPB.ShortCode,
		AccountName:       statPB.AccountName,
		Date:              statPB.ShortCode,
		TotalTransactions: statPB.TotalTransactions,
		TotalAmount:       statPB.TotalAmount,
	}, nil
}

// GetStatPB gets mpesa statistics protobuf from model
func GetStatPB(statDB *Stat) (*mpesapayment.Stat, error) {
	return &mpesapayment.Stat{
		StatId:            fmt.Sprint(statDB.StatID),
		Date:              statDB.ShortCode,
		ShortCode:         statDB.ShortCode,
		AccountName:       statDB.AccountName,
		TotalTransactions: statDB.TotalTransactions,
		TotalAmount:       statDB.TotalAmount,
	}, nil
}
