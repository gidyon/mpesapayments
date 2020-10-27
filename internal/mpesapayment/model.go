package mpesapayment

import (
	"fmt"

	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"github.com/gidyon/services/pkg/utils/errs"
)

// MpesaTables is table for mpesa payments
const MpesaTables = "payments_mpesa"

// Model contains mpesa transaction details
type Model struct {
	PaymentID         uint    `gorm:"primaryKey;autoIncrement"`
	TxType            string  `gorm:"type:varchar(50);not null"`
	TxID              string  `gorm:"type:varchar(50);not null;unique"`
	MSISDN            string  `gorm:"index;type:varchar(15);not null"`
	Names             string  `gorm:"type:varchar(50)"`
	TxRefNumber       string  `gorm:"index;type:varchar(20)"`
	TxTimestamp       int64   `gorm:"type:int(15);not null"`
	TxAmount          float32 `gorm:"type:float(10);not null"`
	OrgAccountBalance float32 `gorm:"type:float(10)"`
	BusinessShortCode int32   `gorm:"index;type:varchar(10);not null"`
	Processed         bool    `gorm:"type:tinyint(1);not null;default:0"`
}

// TableName returns the name of the table
func (*Model) TableName() string {
	return MpesaTables
}

// GetMpesaDB converts protobuf mpesa message to MPESAModel
func GetMpesaDB(MpesaPB *mpesapayment.MPESAPayment) (*Model, error) {
	if MpesaPB == nil {
		return nil, errs.NilObject("mpesa payment")
	}

	mpesaDB := &Model{
		TxID:              MpesaPB.TxId,
		TxType:            MpesaPB.TxType,
		TxTimestamp:       MpesaPB.TxTimestamp,
		MSISDN:            MpesaPB.Msisdn,
		Names:             MpesaPB.Names,
		TxRefNumber:       MpesaPB.TxRefNumber,
		TxAmount:          MpesaPB.TxAmount,
		OrgAccountBalance: MpesaPB.OrgBalance,
		BusinessShortCode: MpesaPB.BusinessShortCode,
		Processed:         MpesaPB.Processed,
	}

	return mpesaDB, nil
}

// GetMpesaPB returns the protobuf message of mpesa payment model
func GetMpesaPB(MpesaDB *Model) (*mpesapayment.MPESAPayment, error) {
	if MpesaDB == nil {
		return nil, errs.NilObject("mpesa payment")
	}

	mpesaPB := &mpesapayment.MPESAPayment{
		PaymentId:         fmt.Sprint(MpesaDB.PaymentID),
		TxType:            MpesaDB.TxType,
		TxId:              MpesaDB.TxID,
		TxTimestamp:       MpesaDB.TxTimestamp,
		Msisdn:            MpesaDB.MSISDN,
		Names:             MpesaDB.Names,
		TxRefNumber:       MpesaDB.TxRefNumber,
		TxAmount:          MpesaDB.TxAmount,
		OrgBalance:        MpesaDB.OrgAccountBalance,
		BusinessShortCode: MpesaDB.BusinessShortCode,
		Processed:         MpesaDB.Processed,
	}

	return mpesaPB, nil
}
