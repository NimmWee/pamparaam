# Feature Specification: Wiki Editor Backend

**Feature Branch**: `001-wiki-editor-backend`  
**Created**: 2026-04-13  
**Status**: Draft  
**Input**: User description: "Build a backend for a wiki-editor module that integrates with MWS Tables."

## Clarifications

### Session 2026-04-13

- Q: Which document and revision storage model should the MVP adopt as the explicit backend decision? -> A: Hybrid model: canonical editable draft is stored as a JSON document snapshot, while links, embedded table refs, and search/backlink projections are extracted into dedicated tables; revisions are append-only document snapshots.
- Q: Which autosave and conflict policy should the MVP use? -> A: Revision-gated autosave: each save references a base revision, the server creates a new append-only revision on success, and stale saves are rejected and must rebase from the latest server state.
- Q: Which realtime collaboration model should the MVP adopt? -> A: Patch-based sync with server-authoritative state; clients send patches against a base revision, the server validates and applies them, then broadcasts accepted updates.
- Q: Which service boundary model should the MVP adopt? -> A: Three services: Page Service owns pages, revisions, publishing, attachments, MWS embed references, and canonical link extraction; Collaboration Service owns presence, live sessions, and patch validation/broadcast; Knowledge Graph/Search Service owns backlinks, search indexing, and query read models built from events.
- Q: Which MWS embedding, caching, and permission policy should the MVP adopt? -> A: MWS remains the source of truth for table data; the wiki stores only embed reference metadata, display settings, and a short-lived cache of table schema and preview data; if MWS is unavailable the page still loads with a degraded embed placeholder; only users who can edit the page and are allowed to access the target table may add or modify embeds.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create Connected Pages (Priority: P1)

A workspace member creates a page, writes block-based content, embeds a live MWS table,
and continues editing without leaving the page context.

**Owning Service**: Page Service  
**Why this priority**: Without reliable page creation and live table embedding, the module
does not solve the fragmentation problem the product is meant to address.  
**Independent Test**: A user creates a page, adds multiple content blocks, inserts a live
table, saves naturally through normal editing, reloads the page, and sees the same page
with the live table still connected.

**Acceptance Scenarios**:

1. **Given** a user can access a workspace, **When** they create a new page and add title
   and content blocks, **Then** the page is stored as an editable knowledge unit.
2. **Given** a user is editing a page, **When** they insert an MWS table through the editor,
   **Then** the page shows a live embedded table rather than a static copy.
3. **Given** a page contains mixed content and an embedded table, **When** the user reopens
   the page later, **Then** the structure and table connection are preserved.

---

### User Story 2 - Save, Recover, and Publish Safely (Priority: P1)

An author edits a page with confidence because changes are autosaved, recoverable after
interruptions, and publishable without losing previous versions.

**Owning Service**: Page Service  
**Why this priority**: Knowledge work fails if authors cannot trust the system to preserve
drafts, restore work after failure, and maintain a clear published record.  
**Independent Test**: A user edits a page, interrupts the session during editing, returns,
recovers the latest draft, publishes it, and reviews earlier versions.

**Acceptance Scenarios**:

1. **Given** a user is editing a page, **When** they pause naturally during editing,
   **Then** recent changes are saved automatically without a manual save action.
2. **Given** an editing session fails unexpectedly, **When** the user returns to the page,
   **Then** they are offered the latest recoverable draft with minimal lost work.
3. **Given** a page is ready to share, **When** the user publishes it, **Then** the system
   records a published version while preserving prior versions for review or rollback.

---

### User Story 3 - Collaborate in Realtime (Priority: P1)

Multiple workspace members work on the same page at the same time, see who is present, and
stay aligned as changes are synchronized.

**Owning Service**: Collaboration Service  
**Why this priority**: Realtime collaboration is a mandatory capability and central to the
goal of turning isolated documents into shared knowledge spaces.  
**Independent Test**: Two or more users open the same page, see each other's presence,
apply edits from separate clients, and observe synchronized page state without creating
confusing duplicate drafts.

**Acceptance Scenarios**:

1. **Given** two users open the same page, **When** both enter editing mode, **Then** each
   user can see the others currently present on the page.
2. **Given** one collaborator edits the page, **When** the change is accepted by the
   system, **Then** other collaborators see the update reflected in their current session.
3. **Given** collaborators make overlapping edits, **When** the system synchronizes them,
   **Then** it resolves or surfaces conflicts in a way that preserves a coherent page state.
4. **Given** collaborators submit concurrent patches, **When** the server accepts one patch
   first, **Then** later stale patches are rejected or rebased against the latest accepted
   page revision before they become visible to others.

---

### User Story 4 - Discover and Govern Knowledge (Priority: P2)

A workspace member links pages, finds related knowledge through search and backlinks,
attaches relevant files, and only sees actions allowed by their role.

**Owning Service**: Knowledge Graph/Search Service  
**Why this priority**: Connected discovery and governed access are what turn stored content
into usable corporate knowledge instead of isolated notes.  
**Independent Test**: A user links pages, uploads an attachment, searches for related
content, opens backlink results, and verifies that available actions match their role.

**Acceptance Scenarios**:

1. **Given** a page references another page, **When** the reference is saved, **Then** the
   linked page can show the backlink relationship.
2. **Given** a user searches for a topic, **When** matching pages, links, and related
   references exist, **Then** the system returns relevant results that help the user regain
   context quickly.
3. **Given** two users have different roles, **When** they access the same page or action,
   **Then** each user only sees or performs actions allowed by their permissions.

---

### User Story 5 - Power the Editor Experience (Priority: P2)

The editor frontend receives the metadata it needs to drive slash-menu actions, block tools,
table insertion, and local-to-server synchronization without inventing its own rules.

**Owning Service**: Page Service  
**Why this priority**: The editor must rely on explicit backend behavior for metadata,
sync, and insert actions or the frontend becomes fragile and inconsistent.  
**Independent Test**: The editor requests its metadata and sync support, offers context
actions to the user, and keeps local edits aligned with the stored page state.

**Acceptance Scenarios**:

1. **Given** the editor loads a workspace page, **When** it requests available authoring
   actions, **Then** it receives the metadata needed to show slash-menu and contextual tools.
2. **Given** a user makes local edits while connectivity is unstable, **When** connection
   quality changes, **Then** the editor can reconcile local and stored state without losing
   the user's recent work.

### Edge Cases

- A user edits a stale page revision while another collaborator publishes a newer version.
- A page contains multiple embedded live tables and one table source becomes temporarily
  unavailable.
- A page loads successfully while one or more embedded tables can only return cached schema
  or preview metadata from the wiki backend.
- A reconnecting editor submits local changes after the page state has advanced elsewhere.
- Autosave requests are retried or received out of order after a newer revision has already
  been accepted.
- A user loses access to a workspace or page while the page is already open in the editor.
- Search results include restricted content that the current user should not be allowed to open.
- An attachment upload succeeds but the page update that references it fails, or vice versa.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST allow authorized users to create, rename, edit, and archive
  pages inside a workspace.
- **FR-002**: The system MUST store page content as ordered blocks so the editor can render
  and update rich page structures.
- **FR-002a**: The system MUST treat the editable page draft as a canonical JSON document
  snapshot so autosave and recovery operate on whole-document revisions.
- **FR-003**: The system MUST allow pages to embed live MWS tables that remain connected to
  their source rather than being stored as static snapshots.
- **FR-003a**: The system MUST treat MWS as the source of truth for table data and store only
  embed reference metadata, display settings, and short-lived cached schema or preview data
  inside the wiki platform.
- **FR-004**: The system MUST provide the editor with the metadata needed to support
  slash-menu actions, contextual insertions, and block-level authoring behaviors.
- **FR-005**: The system MUST automatically preserve in-progress page changes without
  requiring users to trigger a manual save action.
- **FR-005a**: Each autosave request MUST include the client's base revision so the system
  can validate whether the save is still current.
- **FR-006**: The system MUST track page revisions so that draft state can be recovered
  after interruption and conflicting edits can be detected.
- **FR-007**: The system MUST allow users to recover the latest valid draft after editor,
  browser, or service failure.
- **FR-008**: The system MUST support publishing a page while retaining accessible version
  history for review, comparison, or rollback decisions.
- **FR-009**: The system MUST allow pages to reference other pages and expose backlink
  relationships for connected navigation.
- **FR-009a**: The system MUST extract links, embedded table references, and search-visible
  metadata from the canonical page snapshot into dedicated queryable records.
- **FR-009b**: The Page Service MUST extract canonical page-to-page links from every accepted
  page revision, and the Knowledge Graph/Search Service MUST persist forward-link and backlink
  indexes from those extracted records.
- **FR-010**: The system MUST support realtime collaboration so active collaborators can
  see presence information and synchronized page updates.
- **FR-010a**: Collaboration clients MUST submit patches against a known base revision, and
  the system MUST treat the server-held page draft as the authoritative state.
- **FR-011**: The system MUST reconcile local editor state with stored page state so users
  can continue editing through short-lived disruptions and resynchronize safely.
- **FR-011a**: When a client saves against a stale revision, the system MUST reject the
  save, return the latest accepted server revision, and allow the client to rebase local
  edits instead of silently overwriting newer content.
- **FR-012**: The system MUST allow authorized users to upload, attach, retrieve, and
  remove files associated with pages.
- **FR-014a**: The system MUST allow table embedding or embed modification only when the user
  has permission to edit the page and permission to access the target MWS table.
- **FR-013**: The system MUST provide search across page content, titles, references, and
  page relationships so users can recover context quickly.
- **FR-014**: The system MUST enforce role-based access for viewing, editing, publishing,
  linking, searching, and file actions at workspace and page scope.
- **FR-014b**: For MVP, the RBAC model MUST use workspace roles `viewer`, `editor`, and
  `admin`, with page-level view or edit overrides for exceptions; publish actions are allowed
  to workspace `editor` and `admin` roles or equivalent page-level edit grants.
- **FR-015**: The system MUST expose clear frontend-facing contracts for page editing,
  metadata, collaboration, search, attachments, and publishing.
- **FR-016**: The system MUST be extensible enough to add new block types, editor actions,
  and knowledge relationships after MVP without rewriting the page model.
- **FR-016a**: The Page Service MUST own page drafts, revisions, publishing, attachments,
  MWS embed references, and canonical link extraction from page content.
- **FR-016b**: The Collaboration Service MUST own page presence, active editing sessions,
  patch validation, and broadcast of accepted live updates.
- **FR-016c**: The Knowledge Graph/Search Service MUST own backlinks, search indexing, and
  query-oriented read models derived from page and link events.

### Mandatory Platform Capabilities

- **MC-001**: Required and in scope. Pages can embed live MWS tables that stay connected to
  source data.
- **MC-002**: Required and in scope. Autosave protects in-progress work with revision-aware
  draft handling.
- **MC-003**: Required and in scope. Local editor state can resynchronize with stored page
  state after disruption.
- **MC-004**: Required and in scope. Pages can link to other pages and expose backlinks.
- **MC-005**: Required and in scope. The editor receives slash-menu and metadata support for
  authoring workflows.
- **MC-006**: Required and in scope. Realtime collaboration includes presence and patch sync.

### Consistency & Reliability Requirements

- **CR-001**: The system MUST reject or reconcile stale edits in a way that prevents silent
  overwrites of newer page revisions.
- **CR-001a**: For MVP, stale draft writes MUST be detected through base-revision mismatch
  and resolved through explicit client rebase rather than server-side silent merge.
- **CR-002**: The system MUST distinguish between draft state and published state for pages
  that are actively authored and later shared.
- **CR-003**: The system MUST treat autosave history as revision-based so the latest safe
  draft can be restored after interruption.
- **CR-003a**: The system MUST store draft and published revisions as append-only document
  snapshots rather than reconstructing them from per-block history.
- **CR-003b**: Every successful autosave MUST create a new append-only draft revision that
  preserves revision ordering for recovery and auditability.
- **CR-004**: The system MUST keep page updates, publication events, backlink changes, and
  search-visible changes consistent enough that users do not see contradictory page state.
- **CR-004a**: Accepted page patches, autosave revisions, publish transitions, and attachment
  references MUST be strongly consistent inside the Page Service before success is returned.
- **CR-004b**: Backlink indexes, search indexes, and MWS schema or preview caches MAY become
  eventually consistent through event-driven updates after the canonical page revision commits.
- **CR-005**: The system MUST ensure collaboration updates are delivered reliably enough for
  active editors to maintain a shared understanding of current page content.
- **CR-005b**: When MWS is unavailable, the system MUST preserve page availability and show a
  degraded embed state instead of blocking the rest of page viewing or editing.
- **CR-005a**: For MVP, concurrent collaboration changes MUST be validated and applied on the
  server before being broadcast, rather than merged independently by clients.

### Key Entities *(include if feature involves data)*

- **Page**: A connected knowledge unit containing title, ownership, visibility, current
  draft state, and current published state.
- **PageBlock**: A single ordered content element within a page, such as text, heading,
  attachment reference, embedded table, or linked page reference.
- **PageRevision**: A recoverable saved state of a page draft or published version, used for
  autosave, recovery, comparison, and rollback decisions.
- **PageProjection**: A query-oriented record derived from the canonical page snapshot and
  used for backlinks, search, and embedded table lookups.
- **EmbeddedTableReference**: A page element that links a page block to a live MWS table and
  carries the display and connection context needed by the editor.
- **PageLink**: A directed relationship from one page to another that enables connected
  navigation and backlink discovery.
- **CollaborationSession**: The current shared editing context for a page, including active
  participants, page presence, and synchronized changes.
- **PageAttachment**: A file associated with a page and governed by the same access rules as
  the page or block that references it.
- **RoleAssignment**: A workspace-level or page-level permission record that defines which
  actions a user can perform, using MVP roles of viewer, editor, and admin plus page-level
  view or edit overrides where needed.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In acceptance testing, at least 90% of target users can create a page, embed a
  live table, and publish the result in under 5 minutes without facilitator help.
- **SC-002**: In at least 95% of failure-recovery trials, users recover a draft that
  contains all but the last 30 seconds of editing activity.
- **SC-003**: During collaborative acceptance tests with 20 simultaneous active editors, at
  least 95% of visible page updates and presence changes appear to collaborators within
  2 seconds.
- **SC-004**: In relevance testing, at least 90% of search tasks for known pages, links, or
  related topics return a useful result on the first search attempt.
- **SC-005**: In access-control validation, zero unauthorized page views, edits, publishes,
  attachment actions, or restricted search result openings are allowed.

## Assumptions

- Workspace membership and user identity already exist or will be provided as part of the
  same product ecosystem.
- Page editing and editor metadata contracts are served by the Page Service rather than a
  separate editor-only backend service.
- MWS tables expose stable live references that can be embedded and reopened from a page.
- MWS exposes enough access-check information for the wiki backend to verify whether a page
  editor is also allowed to embed or modify a referenced table.
- The MVP primarily targets desktop web authoring workflows; mobile-first editing behavior is
  out of scope unless added later.
- Search relevance can begin with workspace-scoped page, link, and attachment discovery
  before more advanced ranking behavior is introduced.
- Basic file handling is in scope for MVP, while advanced compliance workflows such as legal
  hold or custom retention policies are out of scope.
