# Internal gRPC Contracts

Typed gRPC is used only where strong service-to-service contracts materially help the MVP.

## Shared Conventions

- Metadata headers:
  - `x-request-id`
  - `x-correlation-id`
  - `x-actor-user-id`
- Standard error mapping:
  - `InvalidArgument`: malformed request
  - `PermissionDenied`: authorization failed
  - `NotFound`: resource missing
  - `FailedPrecondition`: stale revision or invalid state transition
  - `Unavailable`: downstream service unavailable

## Shared Value Objects

```proto
message IdentityContext {
  string request_id = 1;
  string correlation_id = 2;
  string actor_user_id = 3;
  string workspace_id = 4;
}

message PageRevisionPointer {
  string page_id = 1;
  string revision_id = 2;
  int64 revision_no = 3;
}
```

## Auth Service

```proto
service AuthorizationService {
  rpc Authorize(AuthorizeRequest) returns (AuthorizeResponse);
  rpc BatchAuthorize(BatchAuthorizeRequest) returns (BatchAuthorizeResponse);
}

message AuthorizeRequest {
  IdentityContext identity = 1;
  string page_id = 2;
  string action = 3;
}

message AuthorizeResponse {
  bool allowed = 1;
  string effective_workspace_role = 2;
  repeated string effective_page_permissions = 3;
  string denial_reason = 4;
}

message BatchAuthorizeRequest {
  IdentityContext identity = 1;
  repeated AuthorizeRequest checks = 2;
}

message BatchAuthorizeResponse {
  repeated AuthorizeResponse results = 1;
}
```

Callers:

- API Gateway
- Page Service
- Collaboration Service
- File Service
- Knowledge Graph/Search Service
- MWS Integration Service

Actions covered in MVP:

- `page.view`
- `page.edit`
- `page.publish`
- `page.restore`
- `page.embed_table`
- `file.upload`
- `file.read`
- `search.query`

## Page Service

```proto
service PageRevisionService {
  rpc GetRevisionHead(GetRevisionHeadRequest) returns (GetRevisionHeadResponse);
  rpc CommitCollaborativeRevision(CommitCollaborativeRevisionRequest) returns (CommitCollaborativeRevisionResponse);
}

message GetRevisionHeadRequest {
  IdentityContext identity = 1;
  string page_id = 2;
}

message GetRevisionHeadResponse {
  PageRevisionPointer current_draft = 1;
  bytes document_snapshot_json = 2;
}

message PatchOperation {
  string op = 1;
  string block_id = 2;
  bytes value_json = 3;
}

message CommitCollaborativeRevisionRequest {
  IdentityContext identity = 1;
  string page_id = 2;
  int64 base_revision_no = 3;
  string patch_id = 4;
  repeated PatchOperation accepted_patch_ops = 5;
  bytes new_document_snapshot_json = 6;
}

message CommitCollaborativeRevisionResponse {
  PageRevisionPointer accepted_revision = 1;
  string document_hash = 2;
}
```

Caller:

- Collaboration Service

## MWS Integration Service

```proto
service MWSIntegrationService {
  rpc ResolveEmbed(ResolveEmbedRequest) returns (ResolveEmbedResponse);
  rpc RefreshEmbedPreview(RefreshEmbedPreviewRequest) returns (RefreshEmbedPreviewResponse);
}

message ResolveEmbedRequest {
  IdentityContext identity = 1;
  string page_id = 2;
  string mws_table_id = 3;
  bytes display_config_json = 4;
  bool force_refresh = 5;
}

message ResolveEmbedResponse {
  bool allowed = 1;
  string title = 2;
  bytes schema_json = 3;
  bytes preview_rows_json = 4;
  string preview_state = 5; // ready, cached, degraded
  int32 cache_ttl_seconds = 6;
}

message RefreshEmbedPreviewRequest {
  string page_id = 1;
  string mws_table_id = 2;
}

message RefreshEmbedPreviewResponse {
  string preview_state = 1;
}
```

Caller:

- Page Service

## File Service

```proto
service FileMetadataService {
  rpc GetFileMetadata(GetFileMetadataRequest) returns (GetFileMetadataResponse);
}

message GetFileMetadataRequest {
  IdentityContext identity = 1;
  string file_id = 2;
  string page_id = 3;
}

message GetFileMetadataResponse {
  bool exists = 1;
  string status = 2; // uploading, ready, deleted
  string filename = 3;
  string content_type = 4;
  int64 size_bytes = 5;
  string object_key = 6;
}
```

Caller:

- Page Service

## Services Without Required gRPC in MVP

- Knowledge Graph/Search Service
  - reads happen over REST
  - projections consume NATS asynchronously
- Collaboration Service
  - public transport is WebSocket
  - downstream consumers observe collaboration activity over NATS
