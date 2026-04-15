# Data Model: Wiki Editor Backend

## Bounded Context Overview

- **Auth Service** owns identity, memberships, JWT sessions, and page-level ACL overrides.
- **Page Service** owns canonical page content, revisions, embeds, attachment references,
  and extracted canonical links.
- **Collaboration Service** owns ephemeral collaboration session state and presence.
- **Knowledge Graph/Search Service** owns search documents, link edges, backlinks, and ACL
  filter projections for query-time filtering.
- **MWS Integration Service** owns short-lived MWS schema/preview cache and refresh state.
- **File Service** owns object metadata and upload session records.

## Auth Service Entities

### User

| Field | Type | Notes |
|-------|------|-------|
| id | UUID | Primary key |
| email | text | Unique |
| display_name | text | Required |
| status | enum | `active`, `disabled` |
| created_at | timestamptz | Required |

### WorkspaceMembership

| Field | Type | Notes |
|-------|------|-------|
| workspace_id | UUID | Scoped workspace |
| user_id | UUID | FK to User |
| role | enum | `viewer`, `editor`, `admin` |
| created_at | timestamptz | Required |

**Uniqueness**: `(workspace_id, user_id)` unique.

### PageGrant

| Field | Type | Notes |
|-------|------|-------|
| page_id | UUID | Scoped page |
| subject_user_id | UUID | User receiving override |
| permission | enum | `view`, `edit` |
| created_at | timestamptz | Required |

**Uniqueness**: `(page_id, subject_user_id, permission)` unique.

## Page Service Entities

### Page

| Field | Type | Notes |
|-------|------|-------|
| id | UUID | Primary key |
| workspace_id | UUID | External auth scope |
| slug | text | Unique per workspace |
| title | text | Required |
| status | enum | `draft`, `published`, `archived` |
| created_by | UUID | User ID reference |
| updated_by | UUID | User ID reference |
| current_draft_revision_id | UUID | FK to PageRevision |
| current_published_revision_id | UUID | FK to PageRevision nullable |
| created_at | timestamptz | Required |
| updated_at | timestamptz | Required |

**Validation**
- `slug` unique inside a workspace.
- Archived pages are immutable except for restore/archive operations.

### PageRevision

| Field | Type | Notes |
|-------|------|-------|
| id | UUID | Primary key |
| page_id | UUID | FK to Page |
| revision_no | bigint | Monotonic per page |
| revision_kind | enum | `draft`, `published` |
| base_revision_id | UUID | Previous accepted revision |
| document_snapshot | jsonb | Canonical block document |
| extracted_title | text | Denormalized title for quick access |
| created_by | UUID | Actor |
| created_via | enum | `rest_autosave`, `collab_patch`, `publish`, `restore` |
| created_at | timestamptz | Required |

**Validation**
- `(page_id, revision_no)` unique.
- `base_revision_id` must match the last accepted draft revision for mutable flows.

### PageLinkRecord

| Field | Type | Notes |
|-------|------|-------|
| id | UUID | Primary key |
| page_revision_id | UUID | Source revision |
| source_page_id | UUID | FK to Page |
| target_page_id | UUID | FK-like external page ref |
| block_id | text | Originating block identifier |
| link_kind | enum | `page_ref`, `backlink_ref` |
| created_at | timestamptz | Required |

### EmbeddedTableReference

| Field | Type | Notes |
|-------|------|-------|
| id | UUID | Primary key |
| page_revision_id | UUID | Revision containing embed |
| page_id | UUID | FK to Page |
| block_id | text | Originating block identifier |
| mws_table_id | text | External table ID |
| display_config | jsonb | View mode, columns, filters |
| preview_cache_key | text | Redis cache pointer |
| created_at | timestamptz | Required |

### AttachmentReference

| Field | Type | Notes |
|-------|------|-------|
| id | UUID | Primary key |
| page_revision_id | UUID | Revision containing attachment |
| page_id | UUID | FK to Page |
| block_id | text | Optional |
| file_id | UUID | File Service metadata reference |
| created_at | timestamptz | Required |

### PageOutbox

| Field | Type | Notes |
|-------|------|-------|
| id | UUID | Primary key |
| aggregate_type | text | `page` |
| aggregate_id | UUID | Page ID |
| event_type | text | Domain event name |
| payload | jsonb | Serialized event |
| status | enum | `pending`, `published`, `failed` |
| created_at | timestamptz | Required |
| published_at | timestamptz | Nullable |

## Collaboration Service Entities

### CollaborationSession

| Field | Type | Notes |
|-------|------|-------|
| session_id | text | Redis key / session identifier |
| page_id | UUID | Scoped page |
| workspace_id | UUID | Scoped workspace |
| current_revision_id | UUID | Latest accepted revision |
| current_revision_no | bigint | Latest accepted number |
| active_members | integer | Derived |
| last_seen_at | timestamptz | Required |

**Storage**: Redis hash; persisted DB record optional and deferred.

### PresenceEntry

| Field | Type | Notes |
|-------|------|-------|
| session_id | text | Session scope |
| user_id | UUID | Present actor |
| connection_id | text | Socket connection |
| cursor_state | jsonb | Optional presence metadata |
| expires_at | timestamptz | TTL |

**Storage**: Redis set/hash with TTL.

## Knowledge Graph / Search Service Entities

### SearchDocument

| Field | Type | Notes |
|-------|------|-------|
| page_id | UUID | Primary key |
| workspace_id | UUID | Scope |
| current_revision_id | UUID | Indexed revision |
| title | text | Indexed |
| searchable_text | text | Flattened document text |
| status | enum | `draft_visible`, `published_visible`, `archived` |
| updated_at | timestamptz | Required |

### LinkEdge

| Field | Type | Notes |
|-------|------|-------|
| source_page_id | UUID | Edge source |
| target_page_id | UUID | Edge target |
| source_revision_id | UUID | Revision that produced edge |
| workspace_id | UUID | Scope |
| created_at | timestamptz | Required |

**Uniqueness**: `(source_page_id, target_page_id, source_revision_id)` unique.

### AccessProjection

| Field | Type | Notes |
|-------|------|-------|
| page_id | UUID | Scoped page |
| workspace_id | UUID | Scope |
| subject_user_id | UUID | User |
| can_view | boolean | Filter bit |
| can_edit | boolean | Filter bit |
| updated_at | timestamptz | Required |

## MWS Integration Service Entities

### TablePreviewCache

| Field | Type | Notes |
|-------|------|-------|
| cache_key | text | Primary key |
| mws_table_id | text | External table |
| schema_json | jsonb | Short-lived cached schema |
| preview_json | jsonb | Short-lived sample/preview |
| fetched_at | timestamptz | Required |
| expires_at | timestamptz | TTL |

**Storage**: Redis primary; optional DB audit deferred.

## File Service Entities

### FileObject

| Field | Type | Notes |
|-------|------|-------|
| id | UUID | Primary key |
| workspace_id | UUID | Scope |
| object_key | text | MinIO object key |
| filename | text | Required |
| content_type | text | Required |
| size_bytes | bigint | Required |
| checksum | text | Optional |
| uploaded_by | UUID | User |
| status | enum | `uploading`, `ready`, `deleted` |
| created_at | timestamptz | Required |

### UploadSession

| Field | Type | Notes |
|-------|------|-------|
| id | UUID | Primary key |
| file_id | UUID | FK to FileObject |
| page_id | UUID | Intended target page |
| expires_at | timestamptz | Required |
| completed_at | timestamptz | Nullable |

## State Transitions

### Page

```text
draft -> published -> archived
  ^         |
  |         v
  +------ restore published revision into new draft head
```

Rules:
- Draft saves only advance the current draft head.
- Publish creates a new immutable published revision and updates the published pointer.
- Restore creates a new draft revision from a prior published revision; it does not mutate history.

### Collaboration Session

```text
idle -> active -> draining -> closed
```

Rules:
- Session becomes `active` on first successful join.
- Session enters `draining` when last active connection disconnects.
- TTL expiry closes the session and clears Redis state.

## Cross-Service Relationship Notes

- `workspace_id`, `user_id`, and permission checks are external references owned by Auth Service.
- `file_id` references File Service metadata; Page Service stores only attachment references.
- `mws_table_id` references MWS; the wiki never owns table rows.
- Search and backlink records are derived projections and may lag behind canonical page state.
