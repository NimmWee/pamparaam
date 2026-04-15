package filev1

import (
	"context"

	authv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/authv1"
	"google.golang.org/grpc"
)

type GetFileMetadataRequest struct {
	Identity authv1.IdentityContext `json:"identity"`
	FileID   string                 `json:"file_id"`
	PageID   string                 `json:"page_id,omitempty"`
}

type GetFileMetadataResponse struct {
	Exists      bool   `json:"exists"`
	Status      string `json:"status"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
	ObjectKey   string `json:"object_key,omitempty"`
}

type FileMetadataServiceClient interface {
	GetFileMetadata(ctx context.Context, in *GetFileMetadataRequest, opts ...grpc.CallOption) (*GetFileMetadataResponse, error)
}

type fileMetadataServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewFileMetadataServiceClient(cc grpc.ClientConnInterface) FileMetadataServiceClient {
	return &fileMetadataServiceClient{cc: cc}
}

func (c *fileMetadataServiceClient) GetFileMetadata(ctx context.Context, in *GetFileMetadataRequest, opts ...grpc.CallOption) (*GetFileMetadataResponse, error) {
	opts = append([]grpc.CallOption{grpc.CallContentSubtype("json")}, opts...)
	out := new(GetFileMetadataResponse)
	if err := c.cc.Invoke(ctx, "/wiki.file.v1.FileMetadataService/GetFileMetadata", in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

type FileMetadataServiceServer interface {
	GetFileMetadata(context.Context, *GetFileMetadataRequest) (*GetFileMetadataResponse, error)
}

func RegisterFileMetadataServiceServer(s grpc.ServiceRegistrar, srv FileMetadataServiceServer) {
	s.RegisterService(&FileMetadataService_ServiceDesc, srv)
}

var FileMetadataService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "wiki.file.v1.FileMetadataService",
	HandlerType: (*FileMetadataServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetFileMetadata",
			Handler:    _FileMetadataService_GetFileMetadata_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "pkg/contracts/filev1/file.go",
}

func _FileMetadataService_GetFileMetadata_Handler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(GetFileMetadataRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FileMetadataServiceServer).GetFileMetadata(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/wiki.file.v1.FileMetadataService/GetFileMetadata",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(FileMetadataServiceServer).GetFileMetadata(ctx, req.(*GetFileMetadataRequest))
	}
	return interceptor(ctx, in, info, handler)
}
