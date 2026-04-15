package mwsv1

import (
	"context"

	"google.golang.org/grpc"
)

type IdentityContext struct {
	RequestID     string `json:"request_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	ActorUserID   string `json:"actor_user_id,omitempty"`
	WorkspaceID   string `json:"workspace_id,omitempty"`
	AccessToken   string `json:"access_token,omitempty"`
}

type ResolveEmbedRequest struct {
	Identity          IdentityContext `json:"identity"`
	PageID            string          `json:"page_id"`
	MwsTableID        string          `json:"mws_table_id"`
	DisplayConfigJSON []byte          `json:"display_config_json,omitempty"`
	ForceRefresh      bool            `json:"force_refresh,omitempty"`
	AllowDegraded     bool            `json:"allow_degraded,omitempty"`
	StoredTitle       string          `json:"stored_title,omitempty"`
}

type ResolveEmbedResponse struct {
	Allowed         bool   `json:"allowed"`
	Title           string `json:"title,omitempty"`
	SchemaJSON      []byte `json:"schema_json,omitempty"`
	PreviewRowsJSON []byte `json:"preview_rows_json,omitempty"`
	PreviewState    string `json:"preview_state,omitempty"`
	CacheTTLSeconds int32  `json:"cache_ttl_seconds,omitempty"`
}

type RefreshEmbedPreviewRequest struct {
	PageID     string `json:"page_id"`
	MwsTableID string `json:"mws_table_id"`
}

type RefreshEmbedPreviewResponse struct {
	PreviewState string `json:"preview_state"`
}

type MWSIntegrationServiceClient interface {
	ResolveEmbed(ctx context.Context, in *ResolveEmbedRequest, opts ...grpc.CallOption) (*ResolveEmbedResponse, error)
	RefreshEmbedPreview(ctx context.Context, in *RefreshEmbedPreviewRequest, opts ...grpc.CallOption) (*RefreshEmbedPreviewResponse, error)
}

type mwsIntegrationServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewMWSIntegrationServiceClient(cc grpc.ClientConnInterface) MWSIntegrationServiceClient {
	return &mwsIntegrationServiceClient{cc: cc}
}

func (c *mwsIntegrationServiceClient) ResolveEmbed(ctx context.Context, in *ResolveEmbedRequest, opts ...grpc.CallOption) (*ResolveEmbedResponse, error) {
	opts = append([]grpc.CallOption{grpc.CallContentSubtype("json")}, opts...)
	out := new(ResolveEmbedResponse)
	if err := c.cc.Invoke(ctx, "/wiki.mws.v1.MWSIntegrationService/ResolveEmbed", in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mwsIntegrationServiceClient) RefreshEmbedPreview(ctx context.Context, in *RefreshEmbedPreviewRequest, opts ...grpc.CallOption) (*RefreshEmbedPreviewResponse, error) {
	opts = append([]grpc.CallOption{grpc.CallContentSubtype("json")}, opts...)
	out := new(RefreshEmbedPreviewResponse)
	if err := c.cc.Invoke(ctx, "/wiki.mws.v1.MWSIntegrationService/RefreshEmbedPreview", in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

type MWSIntegrationServiceServer interface {
	ResolveEmbed(context.Context, *ResolveEmbedRequest) (*ResolveEmbedResponse, error)
	RefreshEmbedPreview(context.Context, *RefreshEmbedPreviewRequest) (*RefreshEmbedPreviewResponse, error)
}

func RegisterMWSIntegrationServiceServer(s grpc.ServiceRegistrar, srv MWSIntegrationServiceServer) {
	s.RegisterService(&MWSIntegrationService_ServiceDesc, srv)
}

var MWSIntegrationService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "wiki.mws.v1.MWSIntegrationService",
	HandlerType: (*MWSIntegrationServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ResolveEmbed",
			Handler:    _MWSIntegrationService_ResolveEmbed_Handler,
		},
		{
			MethodName: "RefreshEmbedPreview",
			Handler:    _MWSIntegrationService_RefreshEmbedPreview_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "pkg/contracts/mwsv1/mws.go",
}

func _MWSIntegrationService_ResolveEmbed_Handler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(ResolveEmbedRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MWSIntegrationServiceServer).ResolveEmbed(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/wiki.mws.v1.MWSIntegrationService/ResolveEmbed",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(MWSIntegrationServiceServer).ResolveEmbed(ctx, req.(*ResolveEmbedRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MWSIntegrationService_RefreshEmbedPreview_Handler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(RefreshEmbedPreviewRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MWSIntegrationServiceServer).RefreshEmbedPreview(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/wiki.mws.v1.MWSIntegrationService/RefreshEmbedPreview",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(MWSIntegrationServiceServer).RefreshEmbedPreview(ctx, req.(*RefreshEmbedPreviewRequest))
	}
	return interceptor(ctx, in, info, handler)
}
