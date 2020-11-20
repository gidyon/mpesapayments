package mocks

import (
	"fmt"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/mocks/mocks"
	"github.com/gidyon/mpesapayments/pkg/api/stk"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/stretchr/testify/mock"
)

// STKAPIMock is mock for LipaNaMPESAServer
type STKAPIMock interface {
	stk.StkPushAPIClient
}

// StkAPI is mock object to be used for stk.StkPushAPIClient
var StkAPI = &mocks.STKAPIMock{}

// InitiateSTKPush(ctx context.Context, in *InitiateSTKPushRequest, opts ...grpc.CallOption) (*InitiateSTKPushResponse, error)
// GetStkPayload(ctx context.Context, in *GetStkPayloadRequest, opts ...grpc.CallOption) (*StkPayload, error)
// CreateStkPayload(ctx context.Context, in *CreateStkPayloadRequest, opts ...grpc.CallOption) (*StkPayload, error)
// ListStkPayloads(ctx context.Context, in *ListStkPayloadsRequest, opts ...grpc.CallOption) (*ListStkPayloadsResponse, error)
// ProcessStkPayload(ctx context.Context, in *ProcessStkPayloadRequest, opts ...grpc.CallOption) (*empty.Empty, error)
// PublishStkPayload(ctx context.Context, in *PublishStkPayloadRequest, opts ...grpc.CallOption) (*empty.Empty, error)
// PublishAllStkPayload(ctx context.Context, in *PublishAllStkPayloadRequest, opts ...grpc.CallOption) (*empty.Empty, error)

func init() {
	StkAPI.On("InitiateSTKPush", mock.Anything, mock.Anything).Return(
		&stk.InitiateSTKPushResponse{
			Progress: true,
			Message:  "please continue with transaction"}, nil,
	)

	StkAPI.On("GetStkPayload", mock.Anything, mock.Anything).Return(
		mockStkPayload(), nil,
	)

	StkAPI.On("CreateStkPayload", mock.Anything, mock.Anything).Return(
		mockStkPayload(), nil,
	)

	StkAPI.On("ListStkPayloads", mock.Anything, mock.Anything).Return(
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

	StkAPI.On("ProcessStkPayload", mock.Anything, mock.Anything).Return(
		&empty.Empty{}, nil,
	)

	StkAPI.On("PublishStkPayload", mock.Anything, mock.Anything).Return(
		&empty.Empty{}, nil,
	)

	StkAPI.On("PublishAllStkPayload", mock.Anything, mock.Anything).Return(
		&empty.Empty{}, nil,
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
