// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	b2c "github.com/gidyon/mpesapayments/pkg/api/b2c"

	emptypb "google.golang.org/protobuf/types/known/emptypb"

	mock "github.com/stretchr/testify/mock"
)

// B2CAPIServer is an autogenerated mock type for the B2CAPIServer type
type B2CAPIServer struct {
	mock.Mock
}

// CreateB2CPayment provides a mock function with given fields: _a0, _a1
func (_m *B2CAPIServer) CreateB2CPayment(_a0 context.Context, _a1 *b2c.CreateB2CPaymentRequest) (*b2c.B2CPayment, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *b2c.B2CPayment
	if rf, ok := ret.Get(0).(func(context.Context, *b2c.CreateB2CPaymentRequest) *b2c.B2CPayment); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*b2c.B2CPayment)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *b2c.CreateB2CPaymentRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetB2CPayment provides a mock function with given fields: _a0, _a1
func (_m *B2CAPIServer) GetB2CPayment(_a0 context.Context, _a1 *b2c.GetB2CPaymentRequest) (*b2c.B2CPayment, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *b2c.B2CPayment
	if rf, ok := ret.Get(0).(func(context.Context, *b2c.GetB2CPaymentRequest) *b2c.B2CPayment); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*b2c.B2CPayment)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *b2c.GetB2CPaymentRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListB2CPayments provides a mock function with given fields: _a0, _a1
func (_m *B2CAPIServer) ListB2CPayments(_a0 context.Context, _a1 *b2c.ListB2CPaymentsRequest) (*b2c.ListB2CPaymentsResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *b2c.ListB2CPaymentsResponse
	if rf, ok := ret.Get(0).(func(context.Context, *b2c.ListB2CPaymentsRequest) *b2c.ListB2CPaymentsResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*b2c.ListB2CPaymentsResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *b2c.ListB2CPaymentsRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ProcessB2CPayment provides a mock function with given fields: _a0, _a1
func (_m *B2CAPIServer) ProcessB2CPayment(_a0 context.Context, _a1 *b2c.ProcessB2CPaymentRequest) (*emptypb.Empty, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *emptypb.Empty
	if rf, ok := ret.Get(0).(func(context.Context, *b2c.ProcessB2CPaymentRequest) *emptypb.Empty); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*emptypb.Empty)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *b2c.ProcessB2CPaymentRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// QueryAccountBalance provides a mock function with given fields: _a0, _a1
func (_m *B2CAPIServer) QueryAccountBalance(_a0 context.Context, _a1 *b2c.QueryAccountBalanceRequest) (*b2c.QueryAccountBalanceResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *b2c.QueryAccountBalanceResponse
	if rf, ok := ret.Get(0).(func(context.Context, *b2c.QueryAccountBalanceRequest) *b2c.QueryAccountBalanceResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*b2c.QueryAccountBalanceResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *b2c.QueryAccountBalanceRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// QueryTransactionStatus provides a mock function with given fields: _a0, _a1
func (_m *B2CAPIServer) QueryTransactionStatus(_a0 context.Context, _a1 *b2c.QueryTransactionStatusRequest) (*b2c.QueryResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *b2c.QueryResponse
	if rf, ok := ret.Get(0).(func(context.Context, *b2c.QueryTransactionStatusRequest) *b2c.QueryResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*b2c.QueryResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *b2c.QueryTransactionStatusRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ReverseTransaction provides a mock function with given fields: _a0, _a1
func (_m *B2CAPIServer) ReverseTransaction(_a0 context.Context, _a1 *b2c.ReverseTransactionRequest) (*emptypb.Empty, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *emptypb.Empty
	if rf, ok := ret.Get(0).(func(context.Context, *b2c.ReverseTransactionRequest) *emptypb.Empty); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*emptypb.Empty)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *b2c.ReverseTransactionRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// TransferFunds provides a mock function with given fields: _a0, _a1
func (_m *B2CAPIServer) TransferFunds(_a0 context.Context, _a1 *b2c.TransferFundsRequest) (*emptypb.Empty, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *emptypb.Empty
	if rf, ok := ret.Get(0).(func(context.Context, *b2c.TransferFundsRequest) *emptypb.Empty); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*emptypb.Empty)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *b2c.TransferFundsRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mustEmbedUnimplementedB2CAPIServer provides a mock function with given fields:
func (_m *B2CAPIServer) mustEmbedUnimplementedB2CAPIServer() {
	_m.Called()
}