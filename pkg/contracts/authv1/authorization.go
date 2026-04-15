package authv1

import (
	"context"

	"google.golang.org/grpc"
)

type IdentityContext struct {
	RequestID     string `json:"request_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	ActorUserID   string `json:"actor_user_id,omitempty"`
	WorkspaceID   string `json:"workspace_id,omitempty"`
}

type AuthorizeRequest struct {
	Identity IdentityContext `json:"identity"`
	PageID   string          `json:"page_id,omitempty"`
	Action   string          `json:"action"`
}

type AuthorizeResponse struct {
	Allowed                  bool     `json:"allowed"`
	EffectiveWorkspaceRole   string   `json:"effective_workspace_role,omitempty"`
	EffectivePagePermissions []string `json:"effective_page_permissions,omitempty"`
	DenialReason             string   `json:"denial_reason,omitempty"`
}

type BatchAuthorizeRequest struct {
	Identity IdentityContext    `json:"identity"`
	Checks   []AuthorizeRequest `json:"checks"`
}

type BatchAuthorizeResponse struct {
	Results []AuthorizeResponse `json:"results"`
}

type AuthorizationServiceClient interface {
	Authorize(ctx context.Context, in *AuthorizeRequest, opts ...grpc.CallOption) (*AuthorizeResponse, error)
	BatchAuthorize(ctx context.Context, in *BatchAuthorizeRequest, opts ...grpc.CallOption) (*BatchAuthorizeResponse, error)
}

type authorizationServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewAuthorizationServiceClient(cc grpc.ClientConnInterface) AuthorizationServiceClient {
	return &authorizationServiceClient{cc: cc}
}

func (c *authorizationServiceClient) Authorize(ctx context.Context, in *AuthorizeRequest, opts ...grpc.CallOption) (*AuthorizeResponse, error) {
	opts = append([]grpc.CallOption{grpc.CallContentSubtype("json")}, opts...)
	out := new(AuthorizeResponse)
	if err := c.cc.Invoke(ctx, "/wiki.auth.v1.AuthorizationService/Authorize", in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *authorizationServiceClient) BatchAuthorize(ctx context.Context, in *BatchAuthorizeRequest, opts ...grpc.CallOption) (*BatchAuthorizeResponse, error) {
	opts = append([]grpc.CallOption{grpc.CallContentSubtype("json")}, opts...)
	out := new(BatchAuthorizeResponse)
	if err := c.cc.Invoke(ctx, "/wiki.auth.v1.AuthorizationService/BatchAuthorize", in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

type AuthorizationServiceServer interface {
	Authorize(context.Context, *AuthorizeRequest) (*AuthorizeResponse, error)
	BatchAuthorize(context.Context, *BatchAuthorizeRequest) (*BatchAuthorizeResponse, error)
}

func RegisterAuthorizationServiceServer(s grpc.ServiceRegistrar, srv AuthorizationServiceServer) {
	s.RegisterService(&AuthorizationService_ServiceDesc, srv)
}

var AuthorizationService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "wiki.auth.v1.AuthorizationService",
	HandlerType: (*AuthorizationServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Authorize",
			Handler:    _AuthorizationService_Authorize_Handler,
		},
		{
			MethodName: "BatchAuthorize",
			Handler:    _AuthorizationService_BatchAuthorize_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "pkg/contracts/authv1/authorization.go",
}

func _AuthorizationService_Authorize_Handler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(AuthorizeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthorizationServiceServer).Authorize(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/wiki.auth.v1.AuthorizationService/Authorize",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(AuthorizationServiceServer).Authorize(ctx, req.(*AuthorizeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AuthorizationService_BatchAuthorize_Handler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(BatchAuthorizeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthorizationServiceServer).BatchAuthorize(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/wiki.auth.v1.AuthorizationService/BatchAuthorize",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(AuthorizationServiceServer).BatchAuthorize(ctx, req.(*BatchAuthorizeRequest))
	}
	return interceptor(ctx, in, info, handler)
}
