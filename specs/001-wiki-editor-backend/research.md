# Research: Wiki Editor Backend

## Decision 1: MVP service topology

- **Decision**: Use seven core backend services plus infrastructure: API Gateway, Auth
  Service, Page Service, Collaboration Service, Knowledge Graph/Search Service, MWS
  Integration Service, and File Service. Do not create a standalone Comment service in MVP.
- **Rationale**: This split preserves bounded contexts where the product is genuinely
  different while avoiding service proliferation that would slow a hackathon team.
  Comments/activity are not mandatory and can be added later as event-driven projections.
- **Alternatives considered**:
  - Fewer services: simpler runtime, but page/collaboration/search concerns become coupled.
  - More services: cleaner theoretical boundaries, but too much operational overhead for MVP.

## Decision 2: Canonical draft persistence

- **Decision**: Store the current editable draft as a canonical JSON document snapshot in
  Page Service, with append-only draft and published revisions. Extract links, embed refs,
  attachments, and search-visible metadata into normalized projection tables.
- **Rationale**: Snapshot persistence matches autosave and recovery needs, while extracted
  projections keep backlink and search queries fast without forcing normalized editing state.
- **Alternatives considered**:
  - Fully normalized block storage: too complex for fast autosave and replay.
  - Snapshot only with no projections: too slow and awkward for backlinks/search.

## Decision 3: Autosave and conflict policy

- **Decision**: Use revision-gated autosave. Every REST autosave or collaboration patch
  references a base revision; Page Service only accepts the next valid revision and rejects
  stale writes for explicit rebase.
- **Rationale**: This gives deterministic conflict detection and aligns with optimistic
  locking, append-only revisions, and server-authoritative collaboration.
- **Alternatives considered**:
  - Last-write-wins: simpler, but unsafe for collaborative editing.
  - Server-side field merge: too error-prone for MVP semantics.

## Decision 4: Realtime collaboration model

- **Decision**: Use patch-based realtime sync with server-authoritative state in
  Collaboration Service, backed by Page Service revision commits.
- **Rationale**: Patch sequencing is simpler than OT/CRDT, and server authority keeps patch
  ordering, stale rejection, and rebasing consistent with revision-gated autosave.
- **Alternatives considered**:
  - Client-side merge: too easy to diverge.
  - OT/CRDT: powerful but too much implementation risk for the MVP window.

## Decision 5: Search and backlink implementation

- **Decision**: Build backlinks and search in Knowledge Graph/Search Service from events,
  using PostgreSQL FTS and dedicated edge/index tables instead of an external search engine.
- **Rationale**: PostgreSQL FTS is sufficient for demo-scale data and keeps the topology
  smaller while still separating read models from Page Service.
- **Alternatives considered**:
  - Search inside Page Service: violates separation of write and read concerns.
  - Elasticsearch/OpenSearch: credible, but unnecessary operational weight for MVP.

## Decision 6: MWS integration strategy

- **Decision**: Treat MWS as the source of truth for table data. Store only embed metadata,
  display configuration, and short-lived schema/preview cache in wiki services. Existing
  embeds degrade gracefully when MWS is unavailable; new embed mutations require live access
  validation.
- **Rationale**: This preserves ownership boundaries and keeps the wiki from becoming a
  shadow table store while still allowing resilient page loads.
- **Alternatives considered**:
  - Full local table replication: too much sync complexity.
  - No cache at all: poor UX during transient MWS failures.

## Decision 7: RBAC model

- **Decision**: Use MVP roles `viewer`, `editor`, and `admin` at workspace level, with
  page-level view/edit overrides and explicit publish rights on edit-capable users.
- **Rationale**: This is the smallest useful permission model that still supports page-level
  governance and table embedding safety.
- **Alternatives considered**:
  - Workspace-only roles: too coarse for sensitive pages.
  - Fine-grained action matrix: overkill for hackathon delivery.

## Decision 8: Strong vs eventual consistency

- **Decision**: Keep page writes, revision advancement, publish transitions, and attachment
  references strongly consistent inside Page Service. Handle backlinks, search indexing,
  activity projections, and MWS cache refresh as eventually consistent event-driven read
  models.
- **Rationale**: Users must trust saved content immediately, but discovery and enrichment can
  lag briefly without breaking authoring flows.
- **Alternatives considered**:
  - Make everything strongly consistent: over-couples services.
  - Make even core page writes eventual: unacceptable for editing trust.

## Decision 9: Observability baseline

- **Decision**: Use shared observability packages for JSON structured logs, Prometheus
  metrics, health/readiness/liveness endpoints, and correlation propagation across REST,
  gRPC, NATS, and WebSocket flows.
- **Rationale**: This is the minimum credible operational baseline for a production-style
  backend demo.
- **Alternatives considered**:
  - Per-service custom instrumentation: too inconsistent.
  - Full tracing stack mandatory: useful later, but not required to deliver MVP.

## Decision 10: Docker Compose runtime

- **Decision**: Run every service plus Redis, NATS, MinIO, service-owned PostgreSQL
  instances, Prometheus, and an `mws-mock` dependency under Docker Compose for the demo.
- **Rationale**: The system must be demo-ready locally with realistic dependencies and seeded
  sample data.
- **Alternatives considered**:
  - Shared single Postgres for all services: simpler, but weakens service ownership.
  - No observability containers: less credible for demo operations.
