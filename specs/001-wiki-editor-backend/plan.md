# Implementation Plan: Wiki Editor Backend

**Branch**: `master` | **Date**: 2026-04-13 | **Spec**: [spec.md](C:/MTC/specs/001-wiki-editor-backend/spec.md)
**Input**: Feature specification from `/specs/001-wiki-editor-backend/spec.md`

**Note**: This plan covers Phase 0 research and Phase 1 design artifacts for a demo-ready
hackathon MVP backend.

## Summary

Build a production-credible wiki editor backend that lets workspace users create connected
pages, autosave and recover drafts, publish versioned knowledge, embed live MWS tables,
collaborate in realtime, attach files, and discover backlinks and search results. The MVP
uses eight runtime components with one deferred capability decision: API Gateway, Auth
Service, Page Service, Collaboration Service, Knowledge Graph/Search Service, MWS
Integration Service, File Service, and shared infrastructure dependencies. No standalone
Comment service is planned for MVP; activity can be projected later from domain events
without changing the core service boundaries.

## Technical Context

**Language/Version**: Go 1.23 baseline across all services  
**Primary Dependencies**: `chi`, `pgx/v5`, `grpc-go`, `nats.go`, `go-redis/v9`,
`minio-go`, `prometheus/client_golang`, `golang-migrate`, `testify`  
**Storage**: PostgreSQL per service as system of record; Redis for cache, presence, locks,
ephemeral collaboration state, and MWS preview cache; MinIO for binary object storage  
**Testing**: `go test`, `testify`, contract tests for REST/WebSocket, integration tests
with Postgres/Redis/NATS/MinIO, Docker Compose smoke tests  
**Target Platform**: Containerized Linux services orchestrated by Docker Compose  
**Project Type**: Microservice backend with REST/OpenAPI, WebSocket collaboration, NATS
events, and selective internal gRPC  
**Performance Goals**: Autosave and patch commit p95 under 500 ms locally; collaboration
presence and accepted patch propagation under 2 seconds for 20 active editors on one page;
search queries under 1 second for workspace-scoped demo data  
**Constraints**: Mandatory MVP collaboration features, canonical JSON draft snapshots,
append-only revisions, optimistic locking, outbox pattern, no business logic in handlers,
no SQL in the domain layer, demo-ready Docker Compose runtime  
**Scale/Scope**: 7 core services plus infrastructure, 10 workspaces, 500 pages, 20
concurrent editors on a single page, and 5 GB of attached demo assets for the hackathon
environment

## Service & Contract Context

**Owning Service**: System-level plan covering API Gateway, Auth Service, Page Service,
Collaboration Service, Knowledge Graph/Search Service, MWS Integration Service, and File
Service  
**Bounded Context**: Wiki authoring, collaborative editing, knowledge navigation, access
control, table embedding, and file attachment workflows  
**Data Ownership**: Each service owns its PostgreSQL schema/tables; Redis keys are
namespaced by service; no service reads another service's database directly  
**Inbound Interfaces**: Public REST endpoints through API Gateway, WebSocket collaboration
through API Gateway to Collaboration Service, internal gRPC for authz and collaboration
commit flows, NATS subjects for projections and asynchronous updates  
**Outbound Integrations**: NATS for domain events, Redis for ephemeral state and cache,
MinIO for object storage, MWS for table metadata and preview access  
**Realtime Impact**: Presence, server-authoritative patch sequencing, autosave fallback,
rebase-required signaling, and short-lived local sync disruption handling

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] Service ownership is explicit; no shared-database access across services is proposed.
- [x] The design keeps business logic out of HTTP/gRPC handlers and SQL out of the domain layer.
- [x] The plan uses the mandated stack or documents a justified exception.
- [x] REST, WebSocket, gRPC, and NATS contracts are identified where relevant.
- [x] Optimistic concurrency, draft/published state, revision autosave, and outbox behavior
      are covered where relevant.
- [x] Observability is planned: structured logs, metrics, health/readiness, correlation IDs.
- [x] Security is planned: JWT auth, workspace/page RBAC, input validation, secret-safe logging.
- [x] Docker Compose demo readiness and end-to-end validation are included.

Post-design review: no gate failures. The plan remains constitution-compliant without
justified exceptions.

## Project Structure

### Documentation (this feature)

```text
specs/001-wiki-editor-backend/
|-- plan.md
|-- research.md
|-- data-model.md
|-- quickstart.md
|-- contracts/
|   |-- public-api.openapi.yaml
|   |-- websocket-protocol.md
|   |-- events.md
|   `-- internal-grpc.md
`-- tasks.md
```

### Source Code (repository root)

```text
services/
|-- api-gateway/
|   |-- cmd/
|   |-- internal/
|   |   |-- domain/
|   |   |-- usecase/
|   |   |-- ports/
|   |   `-- adapters/
|   `-- api/rest/
|-- auth-service/
|   |-- cmd/
|   |-- internal/{domain,usecase,ports,adapters}/
|   |-- api/rest/
|   `-- api/grpc/
|-- page-service/
|   |-- cmd/
|   |-- internal/{domain,usecase,ports,adapters}/
|   |-- api/rest/
|   `-- api/grpc/
|-- collaboration-service/
|   |-- cmd/
|   |-- internal/{domain,usecase,ports,adapters}/
|   |-- api/websocket/
|   `-- api/grpc/
|-- knowledge-graph-search-service/
|   |-- cmd/
|   |-- internal/{domain,usecase,ports,adapters}/
|   `-- api/rest/
|-- mws-integration-service/
|   |-- cmd/
|   |-- internal/{domain,usecase,ports,adapters}/
|   `-- api/grpc/
|-- file-service/
|   |-- cmd/
|   |-- internal/{domain,usecase,ports,adapters}/
|   |-- api/rest/
|   `-- api/grpc/
pkg/
|-- observability/
|-- messaging/
|-- authn/
|-- transport/
`-- contracts/
deploy/
|-- docker-compose.yml
`-- docker/
tests/
|-- contract/
|-- integration/
|-- realtime/
`-- compose/
```

**Structure Decision**: Keep one repository with one directory per service and shared
infrastructure packages under `pkg/`. This keeps the hackathon MVP operable in one repo
while preserving clean architecture and service ownership boundaries.

## Service Design

### API Gateway

**Responsibilities**
- Terminate external HTTP and WebSocket traffic.
- Verify JWTs with Auth Service-issued keys and attach request/correlation IDs.
- Route REST traffic to owning services and WebSocket upgrades to Collaboration Service.
- Expose aggregated OpenAPI and health endpoints.

**Owned Data**
- No business data.
- In-memory route configuration and cached JWKS/public keys only.

**External APIs**
- Public REST base path `/api/v1`.
- WebSocket entrypoint `/ws/collab`.
- `/health/live`, `/health/ready`, `/metrics`, `/openapi.yaml`.

**Internal Events Consumed/Published**
- Consumes Auth key rotation events optionally.
- No required domain event ownership for MVP.

**Storage Needs**
- None beyond config/environment.

**Sync vs Async Interactions**
- Sync: REST to Auth/Page/Search/File, WebSocket proxy to Collaboration.
- Async: none in MVP.

**Failure/Degradation Behavior**
- If downstream service is unavailable, return service-specific error envelopes.
- If Auth JWKS cache is fresh, read-only token validation continues briefly; write actions
  still fail closed if authorization checks cannot be completed.

### Auth Service

**Responsibilities**
- Issue and validate JWTs for demo users.
- Own workspace memberships, workspace roles, and page-level overrides.
- Answer typed authorization checks for page, file, search, and embedding operations.

**Owned Data**
- Users
- Workspace memberships
- Page permission overrides
- JWT refresh sessions / revocation records

**External APIs**
- `POST /auth/login`
- `POST /auth/refresh`
- `GET /auth/me`
- `GET /.well-known/jwks.json`

**Internal Events Consumed/Published**
- Publishes `auth.membership.changed` and `auth.page_acl.changed`.
- No required consumed events for MVP.

**Storage Needs**
- PostgreSQL for identities, memberships, grants, sessions.
- Redis for short-lived refresh/session cache and rate limiting.

**Sync vs Async Interactions**
- Sync: gRPC `Authorize` checks from Gateway, Page, File, Collaboration, Search.
- Async: membership and ACL changes emitted to NATS for search filtering projections.

**Failure/Degradation Behavior**
- Write operations fail closed if authorization cannot be confirmed.
- Existing JWT signature validation may continue at Gateway using cached keys, but no
  privileged action is allowed without an online authorization decision.

### Page Service

**Responsibilities**
- Own pages, canonical draft snapshots, append-only revisions, publish lifecycle, attachment
  references, embed references, slash-menu metadata, and canonical link extraction.
- Enforce optimistic revision gating for autosave and publish requests.
- Persist outbox records for all state-changing page events.

**Owned Data**
- Pages
- Current draft head
- Draft revisions
- Published revisions
- Embedded table references
- Attachment references
- Extracted canonical links
- Outbox records

**External APIs**
- `POST /pages`
- `GET /pages/{pageId}`
- `PATCH /pages/{pageId}/draft`
- `POST /pages/{pageId}/publish`
- `GET /pages/{pageId}/versions`
- `GET /editor/metadata`

**Internal Events Consumed/Published**
- Consumes `files.upload.completed` to validate attachment references.
- Consumes `mws.embed.resolved` and `mws.embed.refreshed`.
- Publishes `page.created`, `page.draft.saved`, `page.published`, `page.archived`,
  `page.links.extracted`, `page.attachments.changed`.

**Storage Needs**
- PostgreSQL JSONB snapshots and normalized projection tables for links, embeds,
  attachments, and draft/published heads.
- Redis for idempotency keys and short-lived draft conflict helpers only.

**Sync vs Async Interactions**
- Sync: Auth authorize checks, MWS embed resolution during insert/update, File metadata
  validation, collaboration commit RPCs.
- Async: downstream search/backlink projections via NATS outbox relay.

**Failure/Degradation Behavior**
- If MWS resolution fails for a new embed, reject the embed mutation.
- If Search Service is unavailable, page saves still succeed and projections catch up later.
- If File Service is unavailable, attachment finalization fails but draft content remains safe.

### Collaboration Service

**Responsibilities**
- Own active page sessions, presence, patch ordering, patch validation, and broadcast of
  accepted live updates.
- Maintain server-authoritative session head state derived from the latest accepted page
  revision.
- Signal `rebase_required` when patches or reconnects are stale.

**Owned Data**
- Active collaboration sessions
- Presence members
- Session locks and ephemeral session head cache
- Patch acknowledgment ledger for reconnect safety

**External APIs**
- WebSocket session lifecycle routed from Gateway.

**Internal Events Consumed/Published**
- Consumes `page.draft.saved` and `page.published` to refresh session heads.
- Publishes `collab.session.started`, `collab.presence.changed`, `collab.patch.accepted`,
  `collab.patch.rejected`, `collab.session.ended`.

**Storage Needs**
- Redis for presence, session head cache, distributed locks, and short-lived replay state.
- Optional PostgreSQL table for audit/session recovery is deferred; not needed for MVP.

**Sync vs Async Interactions**
- Sync: Auth authorize checks on join/write, Page Service gRPC commit of accepted patches.
- Async: presence and patch events to NATS for activity or telemetry consumers.

**Failure/Degradation Behavior**
- If Redis is unavailable, block new collaboration sessions and fall back to REST autosave.
- If Page Service commit fails, reject patch and send rebase instruction.
- If WebSocket drops, editor continues local editing and uses REST autosave fallback.

### Knowledge Graph / Search Service

**Responsibilities**
- Build backlinks and search read models from Page/Auth events.
- Serve search results and backlink queries filtered by projected permissions.
- Optionally project a lightweight activity feed later without changing core ownership.

**Owned Data**
- Search documents
- Link edges
- Backlink indexes
- Permission-filter projections for query filtering
- Optional activity projection table (deferred feature toggle)

**External APIs**
- `GET /search`
- `GET /pages/{pageId}/backlinks`

**Internal Events Consumed/Published**
- Consumes `page.created`, `page.draft.saved`, `page.published`, `page.links.extracted`,
  `page.archived`, `auth.membership.changed`, `auth.page_acl.changed`.
- Publishes `search.reindex.failed` and optional `activity.projected`.

**Storage Needs**
- PostgreSQL with FTS indexes for page content/title metadata and link edge tables.
- Redis for short-lived search result caching only if needed.

**Sync vs Async Interactions**
- Sync: search and backlink read APIs through Gateway.
- Async: all indexing and graph updates from NATS events.

**Failure/Degradation Behavior**
- Search/backlinks may lag behind the latest accepted page revision.
- If the service is unavailable, editing and publishing still succeed; discovery features degrade.

### MWS Integration Service

**Responsibilities**
- Validate MWS table access and resolve table metadata for embeds.
- Cache schema and preview data without storing canonical table content.
- Refresh stale embed previews asynchronously and return degraded states on MWS failure.

**Owned Data**
- Table reference cache keys
- Schema cache
- Preview cache
- Embed refresh attempt status

**External APIs**
- No direct public API in MVP.

**Internal Events Consumed/Published**
- Consumes `page.draft.saved` or embed refresh requests when embedded tables change.
- Publishes `mws.embed.resolved`, `mws.embed.refreshed`, `mws.embed.degraded`.

**Storage Needs**
- Redis for short-lived schema and preview cache.
- Optional PostgreSQL audit table for refresh attempts is low priority and can be skipped.

**Sync vs Async Interactions**
- Sync: gRPC `ResolveEmbed` for insert/update validation and best-effort preview fetch.
- Async: background refresh and degradation events through NATS.

**Failure/Degradation Behavior**
- Existing pages still load with degraded placeholders and last-known preview metadata.
- New embed creation/update fails if access cannot be validated.
- Cache misses during MWS outage return degraded placeholder, not page failure.

### File Service

**Responsibilities**
- Manage attachment upload sessions, MinIO object lifecycle, and file metadata.
- Return presigned upload/download URLs.
- Enforce page-scoped access through Auth checks and attachment ownership references.

**Owned Data**
- File metadata
- Upload sessions
- Object keys and checksum/status

**External APIs**
- `POST /files/uploads`
- `POST /files/uploads/{uploadId}/complete`
- `GET /files/{fileId}`

**Internal Events Consumed/Published**
- Publishes `files.upload.completed`, `files.deleted`.
- Consumes `page.archived` optionally for retention/cleanup workflows.

**Storage Needs**
- PostgreSQL for file metadata and upload sessions.
- MinIO buckets for binary objects.
- Redis for short-lived upload session cache if needed.

**Sync vs Async Interactions**
- Sync: Auth authorize checks, MinIO presign/finalize operations, Page Service attachment
  association checks.
- Async: cleanup jobs and upload completion events.

**Failure/Degradation Behavior**
- If MinIO is unavailable, upload init/finalize fails while page editing remains available.
- Orphan uploads are cleaned asynchronously later.

## Key Flows

### Autosave Lifecycle

1. Client loads page and receives current draft snapshot, current draft revision ID, and
   slash-menu metadata from Page Service through Gateway.
2. User edits locally; editor keeps a local snapshot and pending patch queue.
3. If a live collaboration session exists, patches go to Collaboration Service over
   WebSocket with `base_revision`.
4. Collaboration Service validates patch shape, applies it to session head, and calls Page
   Service gRPC to commit the new draft snapshot with optimistic revision gating.
5. Page Service stores a new append-only draft revision, updates current draft head,
   extracts links/embed metadata, writes outbox entries, and returns new revision ID.
6. Collaboration Service broadcasts accepted patch and new revision to session members.
7. If WebSocket is unavailable, client falls back to REST `PATCH /pages/{id}/draft` with a
   full snapshot and `base_revision`.
8. If the revision is stale, Page Service rejects with the latest accepted revision and the
   editor rebases local changes.

### Publish / Versioning Lifecycle

1. Client requests publish with current draft revision ID.
2. Page Service checks `publish` permission via Auth.
3. Page Service creates an immutable published revision from the current draft head,
   advances the published pointer, preserves prior published revisions, and emits
   `page.published`.
4. Knowledge Graph/Search Service updates published search/backlink projections
   asynchronously.
5. Collaboration Service receives the publish event and refreshes session head metadata.

### Backlink Extraction Flow

1. Every accepted draft or publish revision triggers canonical link extraction in Page Service.
2. Page Service stores extracted forward links in its own normalized tables and emits
   `page.links.extracted`.
3. Knowledge Graph/Search Service consumes the event, updates forward/backlink edges and
   search documents, and filters them by projected permissions.
4. Backlink queries return from the Knowledge Graph/Search Service; eventual consistency is
   acceptable after a successful page save.

### WebSocket Collaboration Flow

1. Client opens `/ws/collab` with JWT and page ID through Gateway.
2. Gateway verifies JWT signature, forwards request ID, and upgrades to Collaboration Service.
3. Collaboration Service authorizes session join with Auth and loads current page revision
   from Page Service if session head is cold.
4. Presence is stored in Redis; session members receive `presence_state`.
5. Client submits patches with `session_id`, `page_id`, `base_revision`, and patch payload.
6. Collaboration Service applies server-authoritative ordering, commits to Page Service,
   then broadcasts `patch.accepted`; stale patches receive `patch_rejected` or
   `rebase_required`.
7. Disconnects remove presence; reconnects rejoin using the latest known revision.

### MWS Embed Resolution Flow

1. Editor requests embed insertion via Page Service.
2. Page Service asks Auth to validate page edit permission, then asks MWS Integration
   Service to validate table access and resolve schema/preview metadata.
3. If MWS is reachable and access is valid, Page Service stores embed reference metadata and
   display config only, then returns the updated draft.
4. If MWS is unavailable for an existing embed, Page reads still return the page plus a
   degraded placeholder and cached preview/schema data if present.
5. MWS Integration Service refreshes previews asynchronously and emits events on state change.

### File Upload Flow

1. Client requests upload session for a page through Gateway/File Service.
2. File Service checks page edit permission with Auth.
3. File Service issues a presigned MinIO upload URL and temporary upload session.
4. Client uploads directly to MinIO.
5. Client calls upload complete; File Service verifies object existence/checksum, persists
   metadata, and publishes `files.upload.completed`.
6. Page Service associates the file reference to a page block or attachment list in a later
   draft save or attach command.

### RBAC Enforcement Points

- Gateway: JWT signature validation and request identity propagation.
- Auth Service: source of truth for workspace roles and page-level overrides.
- Page Service: create/edit/archive/publish/restore/link/embed/attach operations.
- Collaboration Service: join session, submit patch, view presence.
- File Service: upload, finalize, download.
- Knowledge Graph/Search Service: query filtering using projected ACL/membership data.
- MWS Integration Service: require both page edit permission and MWS table access permission
  before allowing embed creation or modification.

## Observability Strategy

- Use JSON structured logging with `request_id`, `correlation_id`, `user_id`, `workspace_id`,
  `page_id`, `service`, and `operation` fields.
- Propagate correlation/request IDs through REST headers, gRPC metadata, NATS message headers,
  and WebSocket session context.
- Expose Prometheus `/metrics` on every service plus the gateway.
- Expose `/health/live`, `/health/ready`, and dependency-aware readiness checks per service.
- Track key metrics:
  - HTTP request duration and status
  - WebSocket session count, patch acceptance/rejection, rebase count
  - Draft save latency and publish latency
  - Search lag and indexing queue depth
  - MWS cache hit rate and degraded embed count
  - File upload success/failure count
- Keep logs free of page content, JWT secrets, and file payload details.

## Docker Compose Runtime Topology

- `gateway`
- `auth-service`
- `page-service`
- `collaboration-service`
- `knowledge-graph-search-service`
- `mws-integration-service`
- `file-service`
- `postgres-auth`
- `postgres-page`
- `postgres-search`
- `postgres-file`
- `redis`
- `nats`
- `minio`
- `mws-mock` for hackathon/demo mode
- `prometheus`

Runtime notes:
- Each service gets its own Postgres database/schema to preserve ownership.
- Redis is shared by service namespace only; no cross-service data model coupling.
- NATS subjects are namespaced by bounded context.
- Docker Compose includes one seed command or init container to create demo users,
  workspaces, pages, and sample MWS tables.

## Complexity Tracking

No constitution violations identified. A standalone Comment service is intentionally omitted
from MVP because activity can be projected later from existing events, and comments are not
mandatory for the demo scope.
