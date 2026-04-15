---

description: "Task list for wiki editor backend MVP implementation"
---

# Tasks: Wiki Editor Backend

**Input**: Design documents from `/specs/001-wiki-editor-backend/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Critical contract, integration, realtime, and compose smoke tests are REQUIRED
for autosave conflicts, publish/restore, collaboration sync, backlink rebuild, MWS embed
resolution, RBAC enforcement, file upload, and search indexing.

**Organization**: Tasks are grouped by phase and then by service while preserving user story
ownership. Parallel work is marked only when files and blocking dependencies do not overlap.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel after listed dependencies are satisfied
- **[Story]**: User story label for story phases only (`[US1]` ... `[US5]`)
- Every task includes a concrete file path

## Path Conventions

- Services: `services/<service-name>/...`
- Shared packages: `pkg/...`
- Deployment/runtime: `deploy/...`, `scripts/...`
- Tests: `tests/contract/...`, `tests/integration/...`, `tests/realtime/...`, `tests/compose/...`

## Phase 1: Setup (Shared Platform Initialization)

**Purpose**: Create the repo/runtime skeleton so all services can be built and run locally.

### Repository and Build

- [X] T001 Create Go workspace and service module skeleton in `go.work`, `services/api-gateway/go.mod`, `services/auth-service/go.mod`, `services/page-service/go.mod`, `services/collaboration-service/go.mod`, `services/knowledge-graph-search-service/go.mod`, `services/mws-integration-service/go.mod`, and `services/file-service/go.mod`
- [X] T002 Create shared package skeletons in `pkg/observability/logger.go`, `pkg/messaging/nats.go`, `pkg/authn/context.go`, and `pkg/transport/httpserver.go`
- [X] T003 [P] Add root automation targets in `Makefile`
- [X] T004 [P] Add environment template and service port map in `.env.example`

### Containers and Bootstrapping

- [X] T005 [P] Add per-service Dockerfiles in `services/api-gateway/Dockerfile`, `services/auth-service/Dockerfile`, `services/page-service/Dockerfile`, `services/collaboration-service/Dockerfile`, `services/knowledge-graph-search-service/Dockerfile`, `services/mws-integration-service/Dockerfile`, and `services/file-service/Dockerfile`
- [X] T006 Define Docker Compose topology in `deploy/docker-compose.yml`
- [X] T007 Add migration runner scripts in `scripts/migrate.sh` and `scripts/migrate.ps1`
- [X] T008 Add local bootstrap scripts in `scripts/bootstrap-demo.sh` and `scripts/bootstrap-demo.ps1`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish shared contracts, auth, infrastructure wiring, and observability
that all user stories depend on.

**CRITICAL**: No user story implementation starts until this phase is complete.

### Contracts

- [X] T009 Finalize public REST contract structure in `specs/001-wiki-editor-backend/contracts/public-api.openapi.yaml`
- [X] T010 [P] Finalize WebSocket collaboration contract in `specs/001-wiki-editor-backend/contracts/websocket-protocol.md`
- [X] T011 [P] Finalize internal gRPC and NATS event contracts in `specs/001-wiki-editor-backend/contracts/internal-grpc.md` and `specs/001-wiki-editor-backend/contracts/events.md`

### Shared Runtime Packages

- [X] T012 Implement structured logging, metrics registry, and correlation middleware in `pkg/observability/logger.go`, `pkg/observability/metrics.go`, and `pkg/transport/request_id.go`
- [X] T013 Implement PostgreSQL, Redis, NATS, and MinIO bootstrap adapters in `pkg/transport/postgres.go`, `pkg/transport/redis.go`, `pkg/messaging/nats.go`, and `pkg/transport/minio.go`
- [X] T014 Implement outbox relay and retry helpers in `pkg/messaging/outbox.go` and `pkg/messaging/retry.go`

### Auth and Gateway Foundation

- [X] T015 Define Auth Service schema for users, workspaces, memberships, refresh sessions, and page grants in `services/auth-service/migrations/0001_auth_foundation.up.sql`
- [X] T016 Implement role model and authorization use cases for `owner`, `admin`, `editor`, `commenter`, and `viewer` in `services/auth-service/internal/domain/role.go`, `services/auth-service/internal/usecase/authorize.go`, and `services/auth-service/internal/adapters/postgres/repository.go`
- [X] T017 Implement JWT login/refresh/JWKS and gRPC authorize endpoints in `services/auth-service/api/rest/handler.go`, `services/auth-service/api/grpc/server.go`, and `services/auth-service/cmd/server/main.go`
- [X] T018 Implement API Gateway auth middleware, route registration, and OpenAPI serving in `services/api-gateway/internal/adapters/http/router.go`, `services/api-gateway/internal/adapters/http/middleware.go`, and `services/api-gateway/cmd/server/main.go`
- [X] T019 Add demo seed command for users, workspaces, and memberships in `services/auth-service/cmd/seed-demo/main.go`
- [X] T020 Add health/readiness/liveness and metrics bootstrap for Gateway and Auth Service in `services/api-gateway/cmd/server/main.go` and `services/auth-service/cmd/server/main.go`

**Checkpoint**: Platform, contracts, auth, and gateway are ready; user story work can begin.

---

## Phase 3: User Story 1 - Create Connected Pages (Priority: P1) MVP Slice

**Goal**: Users can create/read pages with block-based content and embed live MWS tables.

**Independent Test**: Create a page, store block-based content, insert a live MWS table,
reload the page, and verify the table remains a live embed rather than a snapshot.

### Tests for User Story 1

- [X] T021 [P] [US1] Add page CRUD contract tests in `tests/contract/page_api_contract_test.go`
- [X] T022 [P] [US1] Add MWS embed resolution integration tests in `tests/integration/mws_embed_resolution_test.go`

### Page Service

- [X] T023 [P] [US1] Create Page Service schema for pages, page revisions, embedded table refs, attachment refs, page links, and outbox records in `services/page-service/migrations/0001_page_core.up.sql`
- [X] T024 [P] [US1] Implement page aggregate and canonical block snapshot model in `services/page-service/internal/domain/page.go` and `services/page-service/internal/domain/document.go`
- [X] T025 [P] [US1] Implement JSONB revision repository and projection repositories in `services/page-service/internal/adapters/postgres/page_repository.go` and `services/page-service/internal/adapters/postgres/projection_repository.go`
- [X] T026 [US1] Implement create/get page and embed-aware draft retrieval use cases in `services/page-service/internal/usecase/create_page.go` and `services/page-service/internal/usecase/get_page.go`
- [X] T027 [US1] Implement page create/get REST handlers in `services/page-service/api/rest/page_handler.go`

### MWS Integration Service

- [X] T028 [P] [US1] Implement MWS client adapter and token-aware access validation in `services/mws-integration-service/internal/adapters/mws_client.go`
- [X] T029 [US1] Implement schema/preview fetch, embed descriptor creation, and degraded preview handling in `services/mws-integration-service/internal/usecase/resolve_embed.go` and `services/mws-integration-service/api/grpc/server.go`

### Gateway and Events

- [X] T030 [US1] Wire page and MWS routes through the gateway in `services/api-gateway/internal/adapters/http/router.go`
- [X] T031 [US1] Emit `page.created` and initial `page.draft.saved` outbox events from Page Service in `services/page-service/internal/usecase/create_page.go` and `services/page-service/cmd/server/main.go`

**Checkpoint**: Page creation, block persistence, and live MWS embedding are testable end to end.

---

## Phase 4: User Story 2 - Save, Recover, and Publish Safely (Priority: P1)

**Goal**: Users can autosave safely, recover drafts after failure, publish pages, view
version history, and restore prior versions.

**Independent Test**: Save a page repeatedly with revision gating, trigger a stale-write
conflict, recover the latest draft after failure, publish the page, and restore an older
version into a new draft head.

### Tests for User Story 2

- [X] T032 [P] [US2] Add autosave conflict contract tests in `tests/contract/autosave_conflict_api_test.go`
- [X] T033 [P] [US2] Add publish/restore contract tests in `tests/contract/publish_restore_api_test.go`
- [X] T034 [P] [US2] Add revision recovery integration tests in `tests/integration/page_revision_recovery_test.go`

### Page Service

- [X] T035 [P] [US2] Extend Page Service schema for draft head, published head, idempotency keys, and restore lineage in `services/page-service/migrations/0002_revision_lifecycle.up.sql`
- [X] T036 [US2] Implement autosave, append-only revision creation, and stale write rejection in `services/page-service/internal/usecase/autosave_draft.go`
- [X] T037 [US2] Implement publish, version history, and restore use cases in `services/page-service/internal/usecase/publish_page.go`, `services/page-service/internal/usecase/list_versions.go`, and `services/page-service/internal/usecase/restore_revision.go`
- [X] T038 [US2] Implement autosave, version history, publish, and restore REST handlers in `services/page-service/api/rest/draft_handler.go` and `services/page-service/api/rest/version_handler.go`
- [X] T039 [US2] Implement draft recovery query and conflict payload builder in `services/page-service/internal/usecase/recover_draft.go` and `services/page-service/internal/domain/conflict_payload.go`
- [X] T040 [US2] Add autosave audit metrics and domain events in `services/page-service/internal/usecase/autosave_draft.go` and `services/page-service/internal/usecase/publish_page.go`

**Checkpoint**: Autosave, recovery, publish, and restore flows are independently runnable and verified.

---

## Phase 5: User Story 3 - Collaborate in Realtime (Priority: P1)

**Goal**: Multiple users can join authenticated page rooms, see presence, send patches,
receive server-authoritative updates, and recover cleanly after reconnect or stale patches.

**Independent Test**: Two clients join the same page room, exchange patches and presence
updates, trigger stale patch rejection, reconnect, and reload the latest accepted snapshot.

### Tests for User Story 3

- [X] T041 [P] [US3] Add WebSocket bootstrap and patch contract tests in `tests/contract/collab_websocket_contract_test.go`
- [X] T042 [P] [US3] Add realtime session sync integration tests in `tests/realtime/collab_patch_sync_test.go`
- [X] T043 [P] [US3] Add reconnect and stale patch recovery tests in `tests/realtime/collab_reconnect_rebase_test.go`

### Collaboration Service

- [X] T044 [P] [US3] Implement Redis-backed session and presence stores in `services/collaboration-service/internal/adapters/redis/session_store.go` and `services/collaboration-service/internal/adapters/redis/presence_store.go`
- [X] T045 [P] [US3] Implement patch validator, session aggregate, and rebase-required logic in `services/collaboration-service/internal/domain/session.go` and `services/collaboration-service/internal/usecase/submit_patch.go`
- [X] T046 [US3] Implement WebSocket join/leave/presence/cursor handlers in `services/collaboration-service/api/websocket/handler.go`
- [X] T047 [US3] Implement heartbeat, session expiration, and reconnect snapshot reload in `services/collaboration-service/internal/usecase/session_lifecycle.go`
- [X] T048 [US3] Implement page revision commit gRPC client and revision refresh consumer in `services/collaboration-service/internal/adapters/page_client.go` and `services/collaboration-service/internal/adapters/nats_consumer.go`

### Gateway and Page Service Integration

- [X] T049 [US3] Implement WebSocket upgrade proxy and correlation propagation in `services/api-gateway/internal/adapters/http/ws_proxy.go`
- [X] T050 [US3] Implement collaboration commit endpoint contract on Page Service in `services/page-service/api/grpc/server.go`

**Checkpoint**: Realtime collaboration, presence, patch validation, and reconnect behavior are testable.

---

## Phase 6: User Story 4 - Discover and Govern Knowledge (Priority: P2)

**Goal**: Users can upload attachments, navigate backlinks, search knowledge, and see only
the content allowed by workspace/page permissions.

**Independent Test**: Upload a file, attach it to a page, create page links, rebuild
backlinks, search by title/content/link/embed reference, and verify role-based filtering.

### Tests for User Story 4

- [X] T051 [P] [US4] Add backlinks and search contract tests in `tests/contract/backlinks_search_api_test.go`
- [X] T052 [P] [US4] Add file upload contract tests in `tests/contract/file_upload_api_test.go`
- [X] T053 [P] [US4] Add backlink/search/RBAC integration tests in `tests/integration/backlink_search_rbac_test.go`

### Knowledge Graph / Search Service

- [X] T054 [P] [US4] Create search and backlink schema in `services/knowledge-graph-search-service/migrations/0001_search_graph.up.sql`
- [X] T055 [US4] Implement backlink and search projection consumers in `services/knowledge-graph-search-service/internal/usecase/project_page_events.go`
- [X] T056 [US4] Implement PostgreSQL full-text search, related pages, and backlink query use cases in `services/knowledge-graph-search-service/internal/usecase/search_pages.go` and `services/knowledge-graph-search-service/internal/usecase/get_backlinks.go`
- [X] T057 [US4] Implement search/backlinks REST handlers with workspace filtering and `updated_at` sorting in `services/knowledge-graph-search-service/api/rest/search_handler.go`

### File Service and Page Integration

- [X] T058 [P] [US4] Create File Service schema and MinIO metadata model in `services/file-service/migrations/0001_files.up.sql` and `services/file-service/internal/domain/file_object.go`
- [X] T059 [US4] Implement upload session, presigned URL generation, finalize, validation, limits, and soft delete in `services/file-service/internal/usecase/start_upload.go`, `services/file-service/internal/usecase/complete_upload.go`, and `services/file-service/api/rest/file_handler.go`
- [X] T060 [US4] Implement attachment linking in Page Service and file metadata lookup in `services/page-service/internal/usecase/attach_file.go` and `services/file-service/api/grpc/server.go`

### RBAC and Link Extraction

- [X] T061 [US4] Implement canonical link extraction and page link projection updates in `services/page-service/internal/usecase/extract_links.go`
- [X] T062 [US4] Implement RBAC enforcement guards for page, search, collaboration, file, and embed actions in `services/api-gateway/internal/adapters/http/middleware.go`, `services/page-service/internal/usecase/authorize_action.go`, `services/collaboration-service/internal/usecase/authorize_session.go`, `services/file-service/internal/usecase/authorize_file_action.go`, and `services/knowledge-graph-search-service/internal/usecase/filter_results.go`

**Checkpoint**: Backlinks, search, attachments, and permission-filtered access are independently testable.

---

## Phase 7: User Story 5 - Power the Editor Experience (Priority: P2)

**Goal**: The editor can fetch slash-menu, hotkey, and capability metadata and recover local
editing state through backend-supported sync flows.

**Independent Test**: The editor fetches metadata, receives MWS embed descriptors, loses
connectivity, reloads snapshot state, and continues editing using the latest accepted revision.

### Tests for User Story 5

- [X] T063 [P] [US5] Add editor metadata and slash-menu contract tests in `tests/contract/editor_metadata_api_test.go`
- [X] T064 [P] [US5] Add sync resume contract tests in `tests/contract/sync_resume_api_test.go`

### Page Service and MWS Integration Service

- [X] T065 [P] [US5] Implement editor catalog use case (block types, slash-menu items, hotkeys, capabilities) in `services/page-service/internal/domain/editor_catalog.go` and `services/page-service/internal/usecase/get_editor_metadata.go`
- [X] T066 [US5] Implement editor metadata REST handlers in `services/page-service/api/rest/editor_handler.go`
- [X] T067 [US5] Implement client resume sync token and replay window in `services/page-service/internal/usecase/resume_editor_sync.go`
- [X] T068 [US5] Implement resume sync REST endpoint in `services/page-service/api/rest/sync_handler.go`
- [X] T069 [US5] Wire editor metadata and sync endpoints through the gateway in `services/api-gateway/internal/adapters/http/router.go`

**Checkpoint**: Editor metadata, capability catalogs, and local sync resume flows are runnable end to end.

---

## Final Phase: Polish & Cross-Cutting Concerns

**Purpose**: Make the backend demo-ready, observable, and verifiable across services.

- [ ] T069 [P] Add service-wide health/readiness/liveness handlers in `services/api-gateway/cmd/server/main.go`, `services/auth-service/cmd/server/main.go`, `services/page-service/cmd/server/main.go`, `services/collaboration-service/cmd/server/main.go`, `services/knowledge-graph-search-service/cmd/server/main.go`, `services/mws-integration-service/cmd/server/main.go`, and `services/file-service/cmd/server/main.go`
- [ ] T070 [P] Add Prometheus registration and structured log enrichment across services in `pkg/observability/metrics.go` and `pkg/observability/logger.go`
- [ ] T071 [P] Add NATS publisher/consumer retry, timeout, and failure logging in `pkg/messaging/outbox.go`, `pkg/messaging/retry.go`, and service consumer bootstraps
- [ ] T072 [P] Add OpenAPI validation/generation automation in `Makefile` and `scripts/validate_openapi.sh`
- [X] T073 Add Docker Compose smoke test flow in `tests/compose/demo_smoke_test.sh` and `tests/compose/demo_smoke_test.ps1`
- [ ] T074 Add seeded demo data for sample page, embedded table, backlinks, and collaborative session walkthrough in `scripts/seed_demo_data.go`
- [ ] T075 Update operator and demo instructions in `README.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1: Setup** must finish before any service implementation.
- **Phase 2: Foundational** must finish before all user story phases because contracts,
  shared packages, auth, and gateway routing are blocking prerequisites.
- **Phase 3 (US1)** enables the first runnable slice: page CRUD, block persistence, and MWS embed.
- **Phase 4 (US2)** depends on US1 because autosave, publish, and restore build on Page Service persistence.
- **Phase 5 (US3)** depends on US1 and US2 because collaboration commits target revision-gated draft flows.
- **Phase 6 (US4)** depends on US1 and foundational auth/events because backlinks, search,
  files, and RBAC use page events and page identities.
- **Phase 7 (US5)** depends on US1, US2, and US3 because editor metadata and sync resume
  rely on canonical page structure, revision semantics, and collaboration behavior.
- **Final Phase** depends on all story phases.

### User Story Dependencies

- **US1**: No dependency on other user stories once Phase 2 is done.
- **US2**: Depends on US1 Page Service foundations.
- **US3**: Depends on US1 page retrieval and US2 revision gating.
- **US4**: Depends on US1 page events and Phase 2 auth contracts.
- **US5**: Depends on US1 document model plus US2/US3 revision and sync flows.

### Within Each User Story

- Tests should be authored before or alongside implementation and must pass before the phase is complete.
- Database migration and contract tasks come before dependent use case or handler tasks.
- Domain and repository work comes before handler wiring.
- Event emission/projection tasks come after the underlying write model exists.

## Parallel Opportunities

- Phase 1: `T003`, `T004`, and `T005` can run in parallel after `T001` and `T002`.
- Phase 2: `T010` and `T011` can run in parallel after `T009`; `T012`, `T013`, and `T014`
  can run in parallel once contracts are stable.
- US1: `T021`, `T022`, `T023`, `T024`, and `T028` can run in parallel after Phase 2.
- US2: test tasks `T032`-`T034` can run in parallel; `T035` and `T036` can run in parallel.
- US3: `T041`-`T043` can run in parallel; `T044` and `T045` can run in parallel.
- US4: `T051`-`T053`, `T054`, and `T058` can run in parallel after prior dependencies.
- US5: `T063` and `T064` can run in parallel; `T065` and `T067` can run in parallel.
- Final Phase: `T069`-`T072` can run in parallel before `T073`-`T075`.

## Parallel Example: User Story 1

```text
T021 Add page CRUD contract tests in tests/contract/page_api_contract_test.go
T022 Add MWS embed resolution integration tests in tests/integration/mws_embed_resolution_test.go
T023 Create Page Service schema in services/page-service/migrations/0001_page_core.up.sql
T028 Implement MWS client adapter in services/mws-integration-service/internal/adapters/mws_client.go
```

## Parallel Example: User Story 3

```text
T041 Add WebSocket bootstrap and patch contract tests in tests/contract/collab_websocket_contract_test.go
T044 Implement Redis-backed session and presence stores in services/collaboration-service/internal/adapters/redis/session_store.go
T045 Implement patch validator and session aggregate in services/collaboration-service/internal/domain/session.go
```

## Implementation Strategy

### Earliest Runnable Slice

1. Complete Phase 1 and Phase 2.
2. Complete Phase 3 (US1).
3. Validate page CRUD plus live MWS embedding through Docker Compose.

### Demo-Ready MVP Scope

1. Complete Phase 1 and Phase 2.
2. Complete US1, US2, and US3 because page creation, autosave, publish/recover, and
   realtime collaboration are all mandatory MVP capabilities.
3. Complete US4 and US5 because backlinks, search, file handling, and editor metadata are
   also explicitly mandatory for the final demo.
4. Finish the Final Phase and run compose smoke tests.

### Incremental Delivery

1. Ship core authoring foundation with US1.
2. Add revision safety and publish lifecycle with US2.
3. Add realtime collaboration with US3.
4. Add discoverability, files, and access filtering with US4.
5. Add editor capability metadata and sync-resume support with US5.

## Notes

- No standalone monolithic backend is allowed; all tasks preserve the planned service split.
- No mandatory feature is deferred to future work.
- `commenter` role is included in Auth/RBAC tasks for compatibility with the requested access model, even though standalone comment capability is not part of this MVP.
- Every task above includes a concrete file path and is intended to be executable incrementally by `/speckit.implement`.
