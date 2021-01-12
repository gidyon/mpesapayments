// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	grpc "google.golang.org/grpc"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	mock "github.com/stretchr/testify/mock"

	stk "github.com/gidyon/mpesapayments/pkg/api/stk"
)

// STKAPIMock is an autogenerated mock type for the STKAPIMock type
type STKAPIMock struct {
	mock.Mock
}

// CreateStkPayload provides a mock function with given fields: _a0, _a1, _a2
func (_m *STKAPIMock) CreateStkPayload(_a0 context.Context, _a1 *stk.CreateStkPayloadRequest, _a2 ...grpc.CallOption) func(*stk.StkPayload, error) (*stk.StkPayload, error) {
	_va := make([]interface{}, len(_a2))
	for _i := range _a2 {
		_va[_i] = _a2[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _a0, _a1)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 func(*stk.StkPayload, error) (*stk.StkPayload, error)
	if rf, ok := ret.Get(0).(func(context.Context, *stk.CreateStkPayloadRequest, ...grpc.CallOption) func(*stk.StkPayload, error) (*stk.StkPayload, error)); ok {
		r0 = rf(_a0, _a1, _a2...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func(*stk.StkPayload, error) (*stk.StkPayload, error))
		}
	}

	return r0
}

// GetStkPayload provides a mock function with given fields: _a0, _a1, _a2
func (_m *STKAPIMock) GetStkPayload(_a0 context.Context, _a1 *stk.GetStkPayloadRequest, _a2 ...grpc.CallOption) func(*stk.StkPayload, error) (*stk.StkPayload, error) {
	_va := make([]interface{}, len(_a2))
	for _i := range _a2 {
		_va[_i] = _a2[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _a0, _a1)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 func(*stk.StkPayload, error) (*stk.StkPayload, error)
	if rf, ok := ret.Get(0).(func(context.Context, *stk.GetStkPayloadRequest, ...grpc.CallOption) func(*stk.StkPayload, error) (*stk.StkPayload, error)); ok {
		r0 = rf(_a0, _a1, _a2...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func(*stk.StkPayload, error) (*stk.StkPayload, error))
		}
	}

	return r0
}

// InitiateSTKPush provides a mock function with given fields: _a0, _a1, _a2
func (_m *STKAPIMock) InitiateSTKPush(_a0 context.Context, _a1 *stk.InitiateSTKPushRequest, _a2 ...grpc.CallOption) func(*stk.InitiateSTKPushResponse, error) (*stk.InitiateSTKPushResponse, error) {
	_va := make([]interface{}, len(_a2))
	for _i := range _a2 {
		_va[_i] = _a2[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _a0, _a1)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 func(*stk.InitiateSTKPushResponse, error) (*stk.InitiateSTKPushResponse, error)
	if rf, ok := ret.Get(0).(func(context.Context, *stk.InitiateSTKPushRequest, ...grpc.CallOption) func(*stk.InitiateSTKPushResponse, error) (*stk.InitiateSTKPushResponse, error)); ok {
		r0 = rf(_a0, _a1, _a2...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func(*stk.InitiateSTKPushResponse, error) (*stk.InitiateSTKPushResponse, error))
		}
	}

	return r0
}

// ListStkPayloads provides a mock function with given fields: _a0, _a1, _a2
func (_m *STKAPIMock) ListStkPayloads(_a0 context.Context, _a1 *stk.ListStkPayloadsRequest, _a2 ...grpc.CallOption) func(*stk.ListStkPayloadsResponse, error) (*stk.ListStkPayloadsResponse, error) {
	_va := make([]interface{}, len(_a2))
	for _i := range _a2 {
		_va[_i] = _a2[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _a0, _a1)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 func(*stk.ListStkPayloadsResponse, error) (*stk.ListStkPayloadsResponse, error)
	if rf, ok := ret.Get(0).(func(context.Context, *stk.ListStkPayloadsRequest, ...grpc.CallOption) func(*stk.ListStkPayloadsResponse, error) (*stk.ListStkPayloadsResponse, error)); ok {
		r0 = rf(_a0, _a1, _a2...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func(*stk.ListStkPayloadsResponse, error) (*stk.ListStkPayloadsResponse, error))
		}
	}

	return r0
}

// ProcessStkPayload provides a mock function with given fields: _a0, _a1, _a2
func (_m *STKAPIMock) ProcessStkPayload(_a0 context.Context, _a1 *stk.ProcessStkPayloadRequest, _a2 ...grpc.CallOption) func(*emptypb.Empty, error) (*emptypb.Empty, error) {
	_va := make([]interface{}, len(_a2))
	for _i := range _a2 {
		_va[_i] = _a2[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _a0, _a1)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 func(*emptypb.Empty, error) (*emptypb.Empty, error)
	if rf, ok := ret.Get(0).(func(context.Context, *stk.ProcessStkPayloadRequest, ...grpc.CallOption) func(*emptypb.Empty, error) (*emptypb.Empty, error)); ok {
		r0 = rf(_a0, _a1, _a2...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func(*emptypb.Empty, error) (*emptypb.Empty, error))
		}
	}

	return r0
}

// PublishAllStkPayload provides a mock function with given fields: _a0, _a1, _a2
func (_m *STKAPIMock) PublishAllStkPayload(_a0 context.Context, _a1 *stk.PublishAllStkPayloadRequest, _a2 ...grpc.CallOption) func(*emptypb.Empty, error) (*emptypb.Empty, error) {
	_va := make([]interface{}, len(_a2))
	for _i := range _a2 {
		_va[_i] = _a2[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _a0, _a1)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 func(*emptypb.Empty, error) (*emptypb.Empty, error)
	if rf, ok := ret.Get(0).(func(context.Context, *stk.PublishAllStkPayloadRequest, ...grpc.CallOption) func(*emptypb.Empty, error) (*emptypb.Empty, error)); ok {
		r0 = rf(_a0, _a1, _a2...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func(*emptypb.Empty, error) (*emptypb.Empty, error))
		}
	}

	return r0
}

// PublishStkPayload provides a mock function with given fields: _a0, _a1, _a2
func (_m *STKAPIMock) PublishStkPayload(_a0 context.Context, _a1 *stk.PublishStkPayloadRequest, _a2 ...grpc.CallOption) func(*emptypb.Empty, error) (*emptypb.Empty, error) {
	_va := make([]interface{}, len(_a2))
	for _i := range _a2 {
		_va[_i] = _a2[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _a0, _a1)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 func(*emptypb.Empty, error) (*emptypb.Empty, error)
	if rf, ok := ret.Get(0).(func(context.Context, *stk.PublishStkPayloadRequest, ...grpc.CallOption) func(*emptypb.Empty, error) (*emptypb.Empty, error)); ok {
		r0 = rf(_a0, _a1, _a2...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func(*emptypb.Empty, error) (*emptypb.Empty, error))
		}
	}

	return r0
}
