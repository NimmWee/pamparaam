package pagev1

import (
	"context"

	authv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/authv1"
	"google.golang.org/grpc"
)

type GetRevisionHeadRequest struct {
	Identity authv1.IdentityContext `json:"identity"`
	PageID   string                 `json:"page_id"`
}

type GetRevisionHeadResponse struct {
	PageID               string `json:"page_id"`
	WorkspaceID          string `json:"workspace_id"`
	CurrentRevisionID    string `json:"current_revision_id"`
	CurrentRevisionNo    int64  `json:"current_revision_no"`
	DocumentSnapshotJSON []byte `json:"document_snapshot_json"`
}

type PatchOperation struct {
	Op        string `json:"op"`
	BlockID   string `json:"block_id,omitempty"`
	ValueJSON []byte `json:"value_json,omitempty"`
}

type CommitCollaborativeRevisionRequest struct {
	Identity                authv1.IdentityContext `json:"identity"`
	PageID                  string                 `json:"page_id"`
	BaseRevisionNo          int64                  `json:"base_revision_no"`
	PatchID                 string                 `json:"patch_id"`
	AcceptedPatchOps        []PatchOperation       `json:"accepted_patch_ops"`
	NewDocumentSnapshotJSON []byte                 `json:"new_document_snapshot_json"`
}

type CommitCollaborativeRevisionResponse struct {
	AcceptedRevisionID string `json:"accepted_revision_id"`
	AcceptedRevisionNo int64  `json:"accepted_revision_no"`
	DocumentHash       string `json:"document_hash"`
}

type PageRevisionServiceClient interface {
	GetRevisionHead(ctx context.Context, in *GetRevisionHeadRequest, opts ...grpc.CallOption) (*GetRevisionHeadResponse, error)
	CommitCollaborativeRevision(ctx context.Context, in *CommitCollaborativeRevisionRequest, opts ...grpc.CallOption) (*CommitCollaborativeRevisionResponse, error)
}

type pageRevisionServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewPageRevisionServiceClient(cc grpc.ClientConnInterface) PageRevisionServiceClient {
	return &pageRevisionServiceClient{cc: cc}
}

func (c *pageRevisionServiceClient) GetRevisionHead(ctx context.Context, in *GetRevisionHeadRequest, opts ...grpc.CallOption) (*GetRevisionHeadResponse, error) {
	opts = append([]grpc.CallOption{grpc.CallContentSubtype("json")}, opts...)
	out := new(GetRevisionHeadResponse)
	if err := c.cc.Invoke(ctx, "/wiki.page.v1.PageRevisionService/GetRevisionHead", in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *pageRevisionServiceClient) CommitCollaborativeRevision(ctx context.Context, in *CommitCollaborativeRevisionRequest, opts ...grpc.CallOption) (*CommitCollaborativeRevisionResponse, error) {
	opts = append([]grpc.CallOption{grpc.CallContentSubtype("json")}, opts...)
	out := new(CommitCollaborativeRevisionResponse)
	if err := c.cc.Invoke(ctx, "/wiki.page.v1.PageRevisionService/CommitCollaborativeRevision", in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

type PageRevisionServiceServer interface {
	GetRevisionHead(context.Context, *GetRevisionHeadRequest) (*GetRevisionHeadResponse, error)
	CommitCollaborativeRevision(context.Context, *CommitCollaborativeRevisionRequest) (*CommitCollaborativeRevisionResponse, error)
}

func RegisterPageRevisionServiceServer(s grpc.ServiceRegistrar, srv PageRevisionServiceServer) {
	s.RegisterService(&PageRevisionService_ServiceDesc, srv)
}

var PageRevisionService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "wiki.page.v1.PageRevisionService",
	HandlerType: (*PageRevisionServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetRevisionHead",
			Handler:    _PageRevisionService_GetRevisionHead_Handler,
		},
		{
			MethodName: "CommitCollaborativeRevision",
			Handler:    _PageRevisionService_CommitCollaborativeRevision_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "pkg/contracts/pagev1/page.go",
}

func _PageRevisionService_GetRevisionHead_Handler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(GetRevisionHeadRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PageRevisionServiceServer).GetRevisionHead(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/wiki.page.v1.PageRevisionService/GetRevisionHead",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(PageRevisionServiceServer).GetRevisionHead(ctx, req.(*GetRevisionHeadRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PageRevisionService_CommitCollaborativeRevision_Handler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(CommitCollaborativeRevisionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PageRevisionServiceServer).CommitCollaborativeRevision(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/wiki.page.v1.PageRevisionService/CommitCollaborativeRevision",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(PageRevisionServiceServer).CommitCollaborativeRevision(ctx, req.(*CommitCollaborativeRevisionRequest))
	}
	return interceptor(ctx, in, info, handler)
}
