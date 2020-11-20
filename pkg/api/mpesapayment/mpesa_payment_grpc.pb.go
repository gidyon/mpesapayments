// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package mpesapayment

import (
	context "context"
	empty "github.com/golang/protobuf/ptypes/empty"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion7

// LipaNaMPESAClient is the client API for LipaNaMPESA service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type LipaNaMPESAClient interface {
	// Creates a record of mpesa payment.
	CreateMPESAPayment(ctx context.Context, in *CreateMPESAPaymentRequest, opts ...grpc.CallOption) (*CreateMPESAPaymentResponse, error)
	// Retrieves MPESA payment.
	GetMPESAPayment(ctx context.Context, in *GetMPESAPaymentRequest, opts ...grpc.CallOption) (*MPESAPayment, error)
	// Retrieves a collection of MPESA payments.
	ListMPESAPayments(ctx context.Context, in *ListMPESAPaymentsRequest, opts ...grpc.CallOption) (*ListMPESAPaymentsResponse, error)
	// Adds scopes to a user.
	AddScopes(ctx context.Context, in *AddScopesRequest, opts ...grpc.CallOption) (*empty.Empty, error)
	// Retrieves scopes for a user.
	GetScopes(ctx context.Context, in *GetScopesRequest, opts ...grpc.CallOption) (*GetScopesResponse, error)
	// Updates Mpesa transaction processed state to either true or false.
	ProcessMpesaPayment(ctx context.Context, in *ProcessMpesaPaymentRequest, opts ...grpc.CallOption) (*empty.Empty, error)
	// Publishes Mpesa statement for listeners to process. Safe to be called many times.
	PublishMpesaPayment(ctx context.Context, in *PublishMpesaPaymentRequest, opts ...grpc.CallOption) (*empty.Empty, error)
	// Publish all failed Mpesa transaction for listeners to process.
	PublishAllMpesaPayment(ctx context.Context, in *PublishAllMpesaPaymentRequest, opts ...grpc.CallOption) (*empty.Empty, error)
}

type lipaNaMPESAClient struct {
	cc grpc.ClientConnInterface
}

func NewLipaNaMPESAClient(cc grpc.ClientConnInterface) LipaNaMPESAClient {
	return &lipaNaMPESAClient{cc}
}

func (c *lipaNaMPESAClient) CreateMPESAPayment(ctx context.Context, in *CreateMPESAPaymentRequest, opts ...grpc.CallOption) (*CreateMPESAPaymentResponse, error) {
	out := new(CreateMPESAPaymentResponse)
	err := c.cc.Invoke(ctx, "/gidyon.mpesa.LipaNaMPESA/CreateMPESAPayment", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *lipaNaMPESAClient) GetMPESAPayment(ctx context.Context, in *GetMPESAPaymentRequest, opts ...grpc.CallOption) (*MPESAPayment, error) {
	out := new(MPESAPayment)
	err := c.cc.Invoke(ctx, "/gidyon.mpesa.LipaNaMPESA/GetMPESAPayment", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *lipaNaMPESAClient) ListMPESAPayments(ctx context.Context, in *ListMPESAPaymentsRequest, opts ...grpc.CallOption) (*ListMPESAPaymentsResponse, error) {
	out := new(ListMPESAPaymentsResponse)
	err := c.cc.Invoke(ctx, "/gidyon.mpesa.LipaNaMPESA/ListMPESAPayments", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *lipaNaMPESAClient) AddScopes(ctx context.Context, in *AddScopesRequest, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/gidyon.mpesa.LipaNaMPESA/AddScopes", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *lipaNaMPESAClient) GetScopes(ctx context.Context, in *GetScopesRequest, opts ...grpc.CallOption) (*GetScopesResponse, error) {
	out := new(GetScopesResponse)
	err := c.cc.Invoke(ctx, "/gidyon.mpesa.LipaNaMPESA/GetScopes", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *lipaNaMPESAClient) ProcessMpesaPayment(ctx context.Context, in *ProcessMpesaPaymentRequest, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/gidyon.mpesa.LipaNaMPESA/ProcessMpesaPayment", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *lipaNaMPESAClient) PublishMpesaPayment(ctx context.Context, in *PublishMpesaPaymentRequest, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/gidyon.mpesa.LipaNaMPESA/PublishMpesaPayment", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *lipaNaMPESAClient) PublishAllMpesaPayment(ctx context.Context, in *PublishAllMpesaPaymentRequest, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/gidyon.mpesa.LipaNaMPESA/PublishAllMpesaPayment", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// LipaNaMPESAServer is the server API for LipaNaMPESA service.
// All implementations must embed UnimplementedLipaNaMPESAServer
// for forward compatibility
type LipaNaMPESAServer interface {
	// Creates a record of mpesa payment.
	CreateMPESAPayment(context.Context, *CreateMPESAPaymentRequest) (*CreateMPESAPaymentResponse, error)
	// Retrieves MPESA payment.
	GetMPESAPayment(context.Context, *GetMPESAPaymentRequest) (*MPESAPayment, error)
	// Retrieves a collection of MPESA payments.
	ListMPESAPayments(context.Context, *ListMPESAPaymentsRequest) (*ListMPESAPaymentsResponse, error)
	// Adds scopes to a user.
	AddScopes(context.Context, *AddScopesRequest) (*empty.Empty, error)
	// Retrieves scopes for a user.
	GetScopes(context.Context, *GetScopesRequest) (*GetScopesResponse, error)
	// Updates Mpesa transaction processed state to either true or false.
	ProcessMpesaPayment(context.Context, *ProcessMpesaPaymentRequest) (*empty.Empty, error)
	// Publishes Mpesa statement for listeners to process. Safe to be called many times.
	PublishMpesaPayment(context.Context, *PublishMpesaPaymentRequest) (*empty.Empty, error)
	// Publish all failed Mpesa transaction for listeners to process.
	PublishAllMpesaPayment(context.Context, *PublishAllMpesaPaymentRequest) (*empty.Empty, error)
	mustEmbedUnimplementedLipaNaMPESAServer()
}

// UnimplementedLipaNaMPESAServer must be embedded to have forward compatible implementations.
type UnimplementedLipaNaMPESAServer struct {
}

func (UnimplementedLipaNaMPESAServer) CreateMPESAPayment(context.Context, *CreateMPESAPaymentRequest) (*CreateMPESAPaymentResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateMPESAPayment not implemented")
}
func (UnimplementedLipaNaMPESAServer) GetMPESAPayment(context.Context, *GetMPESAPaymentRequest) (*MPESAPayment, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetMPESAPayment not implemented")
}
func (UnimplementedLipaNaMPESAServer) ListMPESAPayments(context.Context, *ListMPESAPaymentsRequest) (*ListMPESAPaymentsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListMPESAPayments not implemented")
}
func (UnimplementedLipaNaMPESAServer) AddScopes(context.Context, *AddScopesRequest) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddScopes not implemented")
}
func (UnimplementedLipaNaMPESAServer) GetScopes(context.Context, *GetScopesRequest) (*GetScopesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetScopes not implemented")
}
func (UnimplementedLipaNaMPESAServer) ProcessMpesaPayment(context.Context, *ProcessMpesaPaymentRequest) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ProcessMpesaPayment not implemented")
}
func (UnimplementedLipaNaMPESAServer) PublishMpesaPayment(context.Context, *PublishMpesaPaymentRequest) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PublishMpesaPayment not implemented")
}
func (UnimplementedLipaNaMPESAServer) PublishAllMpesaPayment(context.Context, *PublishAllMpesaPaymentRequest) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PublishAllMpesaPayment not implemented")
}
func (UnimplementedLipaNaMPESAServer) mustEmbedUnimplementedLipaNaMPESAServer() {}

// UnsafeLipaNaMPESAServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to LipaNaMPESAServer will
// result in compilation errors.
type UnsafeLipaNaMPESAServer interface {
	mustEmbedUnimplementedLipaNaMPESAServer()
}

func RegisterLipaNaMPESAServer(s grpc.ServiceRegistrar, srv LipaNaMPESAServer) {
	s.RegisterService(&_LipaNaMPESA_serviceDesc, srv)
}

func _LipaNaMPESA_CreateMPESAPayment_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateMPESAPaymentRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LipaNaMPESAServer).CreateMPESAPayment(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gidyon.mpesa.LipaNaMPESA/CreateMPESAPayment",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LipaNaMPESAServer).CreateMPESAPayment(ctx, req.(*CreateMPESAPaymentRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LipaNaMPESA_GetMPESAPayment_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetMPESAPaymentRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LipaNaMPESAServer).GetMPESAPayment(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gidyon.mpesa.LipaNaMPESA/GetMPESAPayment",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LipaNaMPESAServer).GetMPESAPayment(ctx, req.(*GetMPESAPaymentRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LipaNaMPESA_ListMPESAPayments_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListMPESAPaymentsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LipaNaMPESAServer).ListMPESAPayments(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gidyon.mpesa.LipaNaMPESA/ListMPESAPayments",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LipaNaMPESAServer).ListMPESAPayments(ctx, req.(*ListMPESAPaymentsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LipaNaMPESA_AddScopes_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddScopesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LipaNaMPESAServer).AddScopes(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gidyon.mpesa.LipaNaMPESA/AddScopes",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LipaNaMPESAServer).AddScopes(ctx, req.(*AddScopesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LipaNaMPESA_GetScopes_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetScopesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LipaNaMPESAServer).GetScopes(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gidyon.mpesa.LipaNaMPESA/GetScopes",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LipaNaMPESAServer).GetScopes(ctx, req.(*GetScopesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LipaNaMPESA_ProcessMpesaPayment_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProcessMpesaPaymentRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LipaNaMPESAServer).ProcessMpesaPayment(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gidyon.mpesa.LipaNaMPESA/ProcessMpesaPayment",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LipaNaMPESAServer).ProcessMpesaPayment(ctx, req.(*ProcessMpesaPaymentRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LipaNaMPESA_PublishMpesaPayment_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PublishMpesaPaymentRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LipaNaMPESAServer).PublishMpesaPayment(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gidyon.mpesa.LipaNaMPESA/PublishMpesaPayment",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LipaNaMPESAServer).PublishMpesaPayment(ctx, req.(*PublishMpesaPaymentRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LipaNaMPESA_PublishAllMpesaPayment_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PublishAllMpesaPaymentRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LipaNaMPESAServer).PublishAllMpesaPayment(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/gidyon.mpesa.LipaNaMPESA/PublishAllMpesaPayment",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LipaNaMPESAServer).PublishAllMpesaPayment(ctx, req.(*PublishAllMpesaPaymentRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _LipaNaMPESA_serviceDesc = grpc.ServiceDesc{
	ServiceName: "gidyon.mpesa.LipaNaMPESA",
	HandlerType: (*LipaNaMPESAServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CreateMPESAPayment",
			Handler:    _LipaNaMPESA_CreateMPESAPayment_Handler,
		},
		{
			MethodName: "GetMPESAPayment",
			Handler:    _LipaNaMPESA_GetMPESAPayment_Handler,
		},
		{
			MethodName: "ListMPESAPayments",
			Handler:    _LipaNaMPESA_ListMPESAPayments_Handler,
		},
		{
			MethodName: "AddScopes",
			Handler:    _LipaNaMPESA_AddScopes_Handler,
		},
		{
			MethodName: "GetScopes",
			Handler:    _LipaNaMPESA_GetScopes_Handler,
		},
		{
			MethodName: "ProcessMpesaPayment",
			Handler:    _LipaNaMPESA_ProcessMpesaPayment_Handler,
		},
		{
			MethodName: "PublishMpesaPayment",
			Handler:    _LipaNaMPESA_PublishMpesaPayment_Handler,
		},
		{
			MethodName: "PublishAllMpesaPayment",
			Handler:    _LipaNaMPESA_PublishAllMpesaPayment_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "mpesa_payment.proto",
}
