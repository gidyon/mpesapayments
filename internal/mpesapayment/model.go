package mpesapayment

import (
	"fmt"
	"os"

	"github.com/gidyon/micro/v2/utils/errs"
	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
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
