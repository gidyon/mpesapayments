package mocks

import (
	"fmt"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/micro/utils/errs"
	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"github.com/gidyon/mpesapayments/pkg/mocks/mocks"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/emptypb"
)

// LNMAPIMock is mock for mpesapayment.LipaNaMPESAClient
type LNMAPIMock interface {
	mpesapayment.LipaNaMPESAClient
}

// HealthyLNMAPI is mpesapayment.LipaNaMPESAClient mock object for successful scenarios
var HealthyLNMAPI = &mocks.LNMAPIMock{}

// UnhealthyLNMAPI is mpesapayment.LipaNaMPESAClient mock object for successful scenarios
var UnhealthyLNMAPI = &mocks.LNMAPIMock{}

// CreateMPESAPayment(context.Context, *CreateMPESAPaymentRequest) (*CreateMPESAPaymentResponse, error)
// GetMPESAPayment(context.Context, *GetMPESAPaymentRequest) (*MPESAPayment, error)
// ListMPESAPayments(context.Context, *ListMPESAPaymentsRequest) (*ListMPESAPaymentsResponse, error)
// AddScopes(context.Context, *AddScopesRequest) (*emptypb.Empty, error)
// GetScopes(context.Context, *GetScopesRequest) (*GetScopesResponse, error)
// ProcessMpesaPayment(context.Context, *ProcessMpesaPaymentRequest) (*emptypb.Empty, error)
// PublishMpesaPayment(context.Context, *PublishMpesaPaymentRequest) (*emptypb.Empty, error)
// PublishAllMpesaPayment(context.Context, *PublishAllMpesaPaymentRequest) (*emptypb.Empty, error)

func init() {
	// Healthy mock
	HealthyLNMAPI.On("CreateMPESAPayment", mock.Anything, mock.Anything).Return(
		&mpesapayment.CreateMPESAPaymentResponse{
			PaymentId: fmt.Sprint(randomdata.Number(1, 10)),
		}, nil,
	)

	HealthyLNMAPI.On("GetMPESAPayment", mock.Anything, mock.Anything, mock.Anything).Return(
		mockMpesaPayment(), nil,
	)

	HealthyLNMAPI.On("ListMPESAPayments", mock.Anything, mock.Anything, mock.Anything).Return(
		&mpesapayment.ListMPESAPaymentsResponse{
			MpesaPayments: []*mpesapayment.MPESAPayment{
				mockMpesaPayment(),
				mockMpesaPayment(),
				mockMpesaPayment(),
				mockMpesaPayment(),
			},
		}, nil,
	)

	HealthyLNMAPI.On("AddScopes", mock.Anything, mock.Anything, mock.Anything).Return(
		&emptypb.Empty{}, nil,
	)

	HealthyLNMAPI.On("GetScopes", mock.Anything, mock.Anything, mock.Anything).Return(
		&mpesapayment.GetScopesResponse{
			Scopes: &mpesapayment.Scopes{
				AllowedAccNumber: []string{randomdata.Adjective(), randomdata.Adjective()},
				AllowedPhones:    []string{randomdata.PhoneNumber()[:10], randomdata.PhoneNumber()[:10]},
			},
		}, nil,
	)

	HealthyLNMAPI.On("ProcessMpesaPayment", mock.Anything, mock.Anything, mock.Anything).Return(
		&emptypb.Empty{}, nil,
	)

	HealthyLNMAPI.On("PublishMpesaPayment", mock.Anything, mock.Anything, mock.Anything).Return(
		&emptypb.Empty{}, nil,
	)

	HealthyLNMAPI.On("PublishAllMpesaPayment", mock.Anything, mock.Anything, mock.Anything).Return(
		&emptypb.Empty{}, nil,
	)

	// UnHealthy mock
	UnhealthyLNMAPI.On("CreateMPESAPayment", mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "creating mpesa payment failed"),
	)

	UnhealthyLNMAPI.On("GetMPESAPayment", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "getting mpesa payment failed"),
	)

	UnhealthyLNMAPI.On("ListMPESAPayments", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "listing mpesa payments failed"),
	)

	UnhealthyLNMAPI.On("AddScopes", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "adding scopes failed"),
	)

	UnhealthyLNMAPI.On("GetScopes", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "getting scopes failed"),
	)

	UnhealthyLNMAPI.On("ProcessMpesaPayment", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "processing failed"),
	)

	UnhealthyLNMAPI.On("PublishMpesaPayment", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "publishing failed"),
	)

	UnhealthyLNMAPI.On("PublishAllMpesaPayment", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "publishing failed"),
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
