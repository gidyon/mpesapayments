package mocks

import (
	"fmt"
	"strings"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/micro/utils/errs"
	"github.com/gidyon/mpesapayments/pkg/api/c2b"
	"github.com/gidyon/mpesapayments/pkg/mocks/mocks"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/emptypb"
)

// LNMAPIMock is mock for c2b.LipaNaMPESAClient
type LNMAPIMock interface {
	c2b.LipaNaMPESAClient
}

// HealthyLNMAPI is c2b.LipaNaMPESAClient mock object for successful scenarios
var HealthyLNMAPI = &mocks.LNMAPIMock{}

// UnhealthyLNMAPI is c2b.LipaNaMPESAClient mock object for successful scenarios
var UnhealthyLNMAPI = &mocks.LNMAPIMock{}

// CreateC2B(context.Context, *CreateC2BRequest) (*CreateC2BResponse, error)
// GetC2B(context.Context, *GetC2BRequest) (*C2B, error)
// ListC2Bs(context.Context, *ListC2BsRequest) (*ListC2BsResponse, error)
// AddScopes(context.Context, *AddScopesRequest) (*emptypb.Empty, error)
// GetScopes(context.Context, *GetScopesRequest) (*GetScopesResponse, error)
// ProcessC2B(context.Context, *ProcessC2BRequest) (*emptypb.Empty, error)
// PublishC2B(context.Context, *PublishC2BRequest) (*emptypb.Empty, error)
// PublishAllC2B(context.Context, *PublishAllC2BRequest) (*emptypb.Empty, error)

func init() {
	// Healthy mock
	HealthyLNMAPI.On("CreateC2B", mock.Anything, mock.Anything).Return(
		&c2b.CreateC2BPaymentResponse{
			PaymentId: fmt.Sprint(randomdata.Number(1, 10)),
		}, nil,
	)

	HealthyLNMAPI.On("GetC2B", mock.Anything, mock.Anything, mock.Anything).Return(
		mockC2B(), nil,
	)

	HealthyLNMAPI.On("ListC2Bs", mock.Anything, mock.Anything, mock.Anything).Return(
		&c2b.ListC2BPaymentsResponse{
			MpesaPayments: []*c2b.C2BPayment{
				mockC2B(),
				mockC2B(),
				mockC2B(),
				mockC2B(),
			},
		}, nil,
	)

	HealthyLNMAPI.On("AddScopes", mock.Anything, mock.Anything, mock.Anything).Return(
		&emptypb.Empty{}, nil,
	)

	HealthyLNMAPI.On("GetScopes", mock.Anything, mock.Anything, mock.Anything).Return(
		&c2b.GetScopesResponse{
			Scopes: &c2b.Scopes{
				AllowedAccNumber: []string{randomdata.Adjective(), randomdata.Adjective()},
				AllowedPhones:    []string{randomdata.PhoneNumber()[:10], randomdata.PhoneNumber()[:10]},
			},
		}, nil,
	)

	HealthyLNMAPI.On("ProcessC2B", mock.Anything, mock.Anything, mock.Anything).Return(
		&emptypb.Empty{}, nil,
	)

	HealthyLNMAPI.On("PublishC2B", mock.Anything, mock.Anything, mock.Anything).Return(
		&emptypb.Empty{}, nil,
	)

	HealthyLNMAPI.On("PublishAllC2B", mock.Anything, mock.Anything, mock.Anything).Return(
		&emptypb.Empty{}, nil,
	)

	// UnHealthy mock
	UnhealthyLNMAPI.On("CreateC2B", mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "creating mpesa payment failed"),
	)

	UnhealthyLNMAPI.On("GetC2B", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "getting mpesa payment failed"),
	)

	UnhealthyLNMAPI.On("ListC2Bs", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "listing mpesa payments failed"),
	)

	UnhealthyLNMAPI.On("AddScopes", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "adding scopes failed"),
	)

	UnhealthyLNMAPI.On("GetScopes", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "getting scopes failed"),
	)

	UnhealthyLNMAPI.On("ProcessC2B", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "processing failed"),
	)

	UnhealthyLNMAPI.On("PublishC2B", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "publishing failed"),
	)

	UnhealthyLNMAPI.On("PublishAllC2B", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "publishing failed"),
	)
}

var (
	txTypes          = []string{"PAY_BILL", "BUY_GOODS"}
	txBillRefNumbers = []string{"abc", "dec", "fgh"}
)

func mockC2B() *c2b.C2BPayment {
	return &c2b.C2BPayment{
		TransactionId:        strings.ToUpper(randomdata.RandStringRunes(32)),
		TransactionType:      txTypes[randomdata.Number(0, len(txTypes))],
		TransactionTimestamp: time.Now().Unix(),
		Msisdn:               randomdata.PhoneNumber()[:10],
		Names:                randomdata.SillyName(),
		RefNumber:            txBillRefNumbers[randomdata.Number(0, len(txBillRefNumbers))],
		Amount:               float32(randomdata.Decimal(1000, 100000)),
		BusinessShortCode:    int32(randomdata.Number(1000, 20000)),
	}
}
