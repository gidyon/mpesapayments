// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package v2

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion7

// StkPushV2Client is the client API for StkPushV2 service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type StkPushV2Client interface {
	// Initiates mpesa payment.
	InitiateSTKPush(ctx context.Context, in *InitiateSTKPushRequest, opts ...grpc.CallOption) (*InitiateSTKPushResponse, error)
	// Retrieves a single stk payload
	GetStkTransaction(ctx context.Context, in *GetStkTransactionRequest, opts ...grpc.CallOption) (*StkTransaction, error)
	// Retrieves a collection of stk push payloads
	ListStkTransactions(ctx context.Context, in *ListStkTransactionsRequest, opts ...grpc.CallOption) (*ListStkTransactionsResponse, error)
	// Processes stk push payload updating its status
	ProcessStkTransaction(ctx context.Context, in *ProcessStkTransactionRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// Publishes stk push payload for consumers
	PublishStkTransaction(ctx context.Context, in *PublishStkTransactionRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

type stkPushV2Client struct {
	cc grpc.ClientConnInterface
}

func NewStkPushV2Client(cc grpc.ClientConnInterface) StkPushV2Client {
	return &stkPushV2Client{cc}
}

func (c *stkPushV2Client) InitiateSTKPush(ctx context.Context, in *InitiateSTKPushRequest, opts ...grpc.CallOption) (*InitiateSTKPushResponse, error) {
	out := new(InitiateSTKPushResponse)
	err := c.cc.Invoke(ctx, "/gidyon.mpesa.StkPushV2/InitiateSTKPush", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *stkPushV2Client) GetStkTransaction(ctx context.Context, in *GetStkTransactionRequest, opts ...grpc.CallOption) (*StkTransaction, error) {
	out := new(StkTransaction)
	err := c.cc.Invoke(ctx, "/gidyon.mpesa.StkPushV2/GetStkTransaction", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *stkPushV2Client) ListStkTransactions(ctx context.Context, in *ListStkTransactionsRequest, opts ...grpc.CallOption) (*ListStkTransactionsResponse, error) {
	out := new(ListStkTransactionsResponse)
	err := c.cc.Invoke(ctx, "/gidyon.mpesa.StkPushV2/ListStkTransactions", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *stkPushV2Client) ProcessStkTransaction(ctx context.Context, in *ProcessStkTransactionRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/gidyon.mpesa.StkPushV2/ProcessStkTransaction", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *stkPushV2Client) PublishStkTransaction(ctx context.Context, in *PublishStkTransactionRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/gidyon.mpesa.StkPushV2/PublishStkTransaction", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StkPushV2Server is the server API for StkPushV2 service.
// All implementations must embed UnimplementedStkPushV2Server
// for forward compatibility
type StkPushV2Server interface {
	// Initiates mpesa payment.
	InitiateSTKPush(context.Context, *InitiateSTKPushRequest) (*InitiateSTKPushResponse, error)
	// Retrieves a single stk payload
	GetStkTransaction(context.Context, *GetStkTransactionRequest) (*StkTransaction, error)
	// Retrieves a collection of stk push payloads
	ListStkTransactions(context.Context, *ListStkTransactionsRequest) (*ListStkTransactionsResponse, error)
	// Processes stk push payload updating its status
	ProcessStkTransaction(context.Context, *ProcessStkTransactionRequest) (*emptypb.Empty, error)
	// Publishes stk push payload for consumers
	PublishStkTransaction(context.Context, *PublishStkTransactionRequest) (*emptypb.Empty, error)
	mustEmbedUnimplementedStkPushV2Server()
}

// UnimplementedStkPushV2Server must be embedded to have forward compatible implementations.
type UnimplementedStkPushV2Server struct {
}

func (UnimplementedStkPushV2Server) InitiateSTKPush(context.Context, *InitiateSTKPushRequest) (*InitiateSTKPushResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method InitiateSTKPush not implemented")
}
func (UnimplementedStkPushV2Server) GetStkTransaction(context.Context, *GetStkTransactionRequest) (*StkTransaction, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetStkTransaction not implemented")
}
func (UnimplementedStkPushV2Server) ListStkTransactions(context.Context, *ListStkTransactionsRequest) (*ListStkTransactionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListStkTransactions not implemented")
}
func (UnimplementedStkPushV2Server) ProcessStkTransaction(context.Context, *ProcessStkTransactionRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ProcessStkTransaction not implemented")
}
func (UnimplementedStkPushV2Server) PublishStkTransaction(context.Context, *PublishStkTransactionRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PublishStkTransaction not implemented")
}
func (UnimplementedStkPushV2Server) mustEmbedUnimplementedStkPushV2Server() {}

// UnsafeStkPushV2Server may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to StkPushV2Server will
// result in compilation errors.
type UnsafeStkPushV2Server interface {
	mustEmbedUnimplementedStkPushV2Server()
}

func RegisterStkPushV2Server(s grpc.ServiceRegistrar, srv StkPushV2Server) {
	s.RegisterService(&_StkPushV2_serviceDesc, srv)
}

func _StkPushV2_InitiateSTKPush_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(InitiateSTKPushRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StkPushV2Server).InitiateSTKPush(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gidyon.mpesa.StkPushV2/InitiateSTKPush",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StkPushV2Server).InitiateSTKPush(ctx, req.(*InitiateSTKPushRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _StkPushV2_GetStkTransaction_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetStkTransactionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StkPushV2Server).GetStkTransaction(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gidyon.mpesa.StkPushV2/GetStkTransaction",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StkPushV2Server).GetStkTransaction(ctx, req.(*GetStkTransactionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _StkPushV2_ListStkTransactions_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListStkTransactionsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StkPushV2Server).ListStkTransactions(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gidyon.mpesa.StkPushV2/ListStkTransactions",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StkPushV2Server).ListStkTransactions(ctx, req.(*ListStkTransactionsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _StkPushV2_ProcessStkTransaction_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProcessStkTransactionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StkPushV2Server).ProcessStkTransaction(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gidyon.mpesa.StkPushV2/ProcessStkTransaction",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StkPushV2Server).ProcessStkTransaction(ctx, req.(*ProcessStkTransactionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _StkPushV2_PublishStkTransaction_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PublishStkTransactionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StkPushV2Server).PublishStkTransaction(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gidyon.mpesa.StkPushV2/PublishStkTransaction",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StkPushV2Server).PublishStkTransaction(ctx, req.(*PublishStkTransactionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _StkPushV2_serviceDesc = grpc.ServiceDesc{
	ServiceName: "gidyon.mpesa.StkPushV2",
	HandlerType: (*StkPushV2Server)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "InitiateSTKPush",
			Handler:    _StkPushV2_InitiateSTKPush_Handler,
		},
		{
			MethodName: "GetStkTransaction",
			Handler:    _StkPushV2_GetStkTransaction_Handler,
		},
		{
			MethodName: "ListStkTransactions",
			Handler:    _StkPushV2_ListStkTransactions_Handler,
		},
		{
			MethodName: "ProcessStkTransaction",
			Handler:    _StkPushV2_ProcessStkTransaction_Handler,
		},
		{
			MethodName: "PublishStkTransaction",
			Handler:    _StkPushV2_PublishStkTransaction_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "stk.v2.proto",
}
