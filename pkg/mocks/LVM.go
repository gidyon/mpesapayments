package mocks

import (
	"fmt"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/mocks/mocks"
	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/stretchr/testify/mock"
)

// LNMAPIMock is mock for LipaNaMPESAServer
type LNMAPIMock interface {
	mpesapayment.LipaNaMPESAServer
}

// LNMAPI is mock object for mpesapayment.LipaNaMPESAServer
var LNMAPI = &mocks.LNMAPIMock{}

// CreateMPESAPayment(context.Context, *CreateMPESAPaymentRequest) (*CreateMPESAPaymentResponse, error)
// GetMPESAPayment(context.Context, *GetMPESAPaymentRequest) (*MPESAPayment, error)
// ListMPESAPayments(context.Context, *ListMPESAPaymentsRequest) (*ListMPESAPaymentsResponse, error)
// AddScopes(context.Context, *AddScopesRequest) (*empty.Empty, error)
// GetScopes(context.Context, *GetScopesRequest) (*GetScopesResponse, error)
// ProcessMpesaPayment(context.Context, *ProcessMpesaPaymentRequest) (*empty.Empty, error)
// PublishMpesaPayment(context.Context, *PublishMpesaPaymentRequest) (*empty.Empty, error)
// PublishAllMpesaPayment(context.Context, *PublishAllMpesaPaymentRequest) (*empty.Empty, error)

func init() {
	LNMAPI.On("CreateMPESAPayment", mock.Anything, mock.Anything).Return(
		&mpesapayment.CreateMPESAPaymentResponse{
			PaymentId: fmt.Sprint(randomdata.Number(1, 10)),
		}, nil,
	)

	LNMAPI.On("GetMPESAPayment", mock.Anything, mock.Anything).Return(
		mockStkPayload(), nil,
	)

	LNMAPI.On("ListMPESAPayments", mock.Anything, mock.Anything).Return(
		&mpesapayment.ListMPESAPaymentsResponse{
			MpesaPayments: []*mpesapayment.MPESAPayment{
				mockMpesaPayment(),
				mockMpesaPayment(),
				mockMpesaPayment(),
				mockMpesaPayment(),
			},
		}, nil,
	)

	LNMAPI.On("AddScopes", mock.Anything, mock.Anything).Return(
		&empty.Empty{}, nil,
	)

	LNMAPI.On("GetScopes", mock.Anything, mock.Anything).Return(
		&mpesapayment.GetScopesResponse{
			Scopes: &mpesapayment.Scopes{
				AllowedAccNumber: []string{randomdata.Adjective(), randomdata.Adjective()},
				AllowedPhones:    []string{randomdata.PhoneNumber()[:10], randomdata.PhoneNumber()[:10]},
			},
		}, nil,
	)

	LNMAPI.On("ProcessMpesaPayment", mock.Anything, mock.Anything).Return(
		&empty.Empty{}, nil,
	)

	LNMAPI.On("PublishMpesaPayment", mock.Anything, mock.Anything).Return(
		&empty.Empty{}, nil,
	)

	LNMAPI.On("PublishAllMpesaPayment", mock.Anything, mock.Anything).Return(
		&empty.Empty{}, nil,
	)
}

var (
	txTypes          = []string{"PAY_BILL", "BUY_GOODS"}
	txBillRefNumbers = []string{"abc", "dec", "fgh"}
)

func mockMpesaPayment() *mpesapayment.MPESAPayment {
	return &mpesapayment.MPESAPayment{
		PaymentId:         fmt.Sprint(randomdata.Number(1, 10)),
		TxId:              randomdata.RandStringRunes(32),
		TxType:            txTypes[randomdata.Number(0, len(txTypes))],
		TxTimestamp:       time.Now().Unix(),
		Msisdn:            randomdata.PhoneNumber()[:10],
		Names:             randomdata.SillyName(),
		TxRefNumber:       txBillRefNumbers[randomdata.Number(0, len(txBillRefNumbers))],
		TxAmount:          float32(randomdata.Decimal(1000, 100000)),
		BusinessShortCode: int32(randomdata.Number(1000, 20000)),
	}
}
