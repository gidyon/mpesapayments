package mocks

import (
	"fmt"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/stk"
	"github.com/gidyon/mpesapayments/pkg/mocks/mocks"
	"github.com/gidyon/services/pkg/utils/errs"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
)

// STKAPIMock is mock for stk.StkPushAPIClient
type STKAPIMock interface {
	stk.StkPushAPIClient
}

// HealthStkAPI is mock object to be used for stk.StkPushAPIClient for successful scenarios
var HealthStkAPI = &mocks.STKAPIMock{}

// UnhealthyStkAPI is mock object to be used for stk.StkPushAPIClient for failing scenarios
var UnhealthyStkAPI = &mocks.STKAPIMock{}

// InitiateSTKPush(ctx context.Context, in *InitiateSTKPushRequest, opts ...grpc.CallOption) (*InitiateSTKPushResponse, error)
// GetStkPayload(ctx context.Context, in *GetStkPayloadRequest, opts ...grpc.CallOption) (*StkPayload, error)
// CreateStkPayload(ctx context.Context, in *CreateStkPayloadRequest, opts ...grpc.CallOption) (*StkPayload, error)
// ListStkPayloads(ctx context.Context, in *ListStkPayloadsRequest, opts ...grpc.CallOption) (*ListStkPayloadsResponse, error)
// ProcessStkPayload(ctx context.Context, in *ProcessStkPayloadRequest, opts ...grpc.CallOption) (*empty.Empty, error)
// PublishStkPayload(ctx context.Context, in *PublishStkPayloadRequest, opts ...grpc.CallOption) (*empty.Empty, error)
// PublishAllStkPayload(ctx context.Context, in *PublishAllStkPayloadRequest, opts ...grpc.CallOption) (*empty.Empty, error)

func init() {
	// Healthy mock
	HealthStkAPI.On("InitiateSTKPush", mock.Anything, mock.Anything, mock.Anything).Return(
		&stk.InitiateSTKPushResponse{
			Progress: true,
			Message:  "please continue with transaction"}, nil,
	)

	HealthStkAPI.On("GetStkPayload", mock.Anything, mock.Anything, mock.Anything).Return(
		mockStkPayload(), nil,
	)

	HealthStkAPI.On("CreateStkPayload", mock.Anything, mock.Anything, mock.Anything).Return(
		mockStkPayload(), nil,
	)

	HealthStkAPI.On("ListStkPayloads", mock.Anything, mock.Anything, mock.Anything).Return(
		&stk.ListStkPayloadsResponse{
			StkPayloads: []*stk.StkPayload{
				mockStkPayload(),
				mockStkPayload(),
				mockStkPayload(),
				mockStkPayload(),
				mockStkPayload(),
				mockStkPayload(),
			},
		}, nil,
	)

	HealthStkAPI.On("ProcessStkPayload", mock.Anything, mock.Anything, mock.Anything).Return(
		&empty.Empty{}, nil,
	)

	HealthStkAPI.On("PublishStkPayload", mock.Anything, mock.Anything, mock.Anything).Return(
		&empty.Empty{}, nil,
	)

	HealthStkAPI.On("PublishAllStkPayload", mock.Anything, mock.Anything, mock.Anything).Return(
		&empty.Empty{}, nil,
	)

	// Unhealthy mock
	UnhealthyStkAPI.On("InitiateSTKPush", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "initiating stk push failed"),
	)

	UnhealthyStkAPI.On("GetStkPayload", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "getting stk payload failed"),
	)

	UnhealthyStkAPI.On("CreateStkPayload", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "creating stk payload failed"),
	)

	UnhealthyStkAPI.On("ListStkPayloads", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "listing stk payloads failed"),
	)

	UnhealthyStkAPI.On("ProcessStkPayload", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "processing failed"),
	)

	UnhealthyStkAPI.On("PublishStkPayload", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "publishing failed"),
	)

	UnhealthyStkAPI.On("PublishAllStkPayload", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, errs.WrapMessage(codes.Unknown, "publishing failed"),
	)
}

func randomParagraph(l int) string {
	par := randomdata.Paragraph()
	if len(par) > l {
		return par[:l]
	}
	return par
}

func mockStkPayload() *stk.StkPayload {
	return &stk.StkPayload{
		PayloadId:          fmt.Sprint(randomdata.Number(1, 10)),
		MerchantRequestId:  randomdata.RandStringRunes(48),
		CheckoutRequestId:  randomdata.RandStringRunes(44),
		ResultCode:         fmt.Sprint(randomdata.Number(0, 9999)),
		ResultDesc:         randomParagraph(100),
		Amount:             fmt.Sprint(randomdata.Decimal(5, 10)),
		MpesaReceiptNumber: randomdata.RandStringRunes(32),
		TransactionDate:    randomdata.FullDate(),
		PhoneNumber:        randomdata.PhoneNumber()[:10],
	}
}
