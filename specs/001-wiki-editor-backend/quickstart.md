# Quickstart: Wiki Editor Backend MVP

## Prerequisites

- Docker Desktop or compatible Docker Engine
- Docker Compose v2
- 8 GB RAM recommended for full local stack

## Services Started by Compose

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
- `mws-mock`
- `prometheus`

## Start the Stack

```bash
docker compose -f deploy/docker-compose.yml up --build
```

## Initialize Demo Data

1. Run service migrations for all Postgres-backed services.
2. Seed demo users, workspaces, sample wiki pages, and sample MWS tables.
3. Verify MinIO bucket creation for attachments.

Suggested bootstrap sequence:

```bash
docker compose -f deploy/docker-compose.yml run --rm auth-service migrate
docker compose -f deploy/docker-compose.yml run --rm page-service migrate
docker compose -f deploy/docker-compose.yml run --rm knowledge-graph-search-service migrate
docker compose -f deploy/docker-compose.yml run --rm file-service migrate
docker compose -f deploy/docker-compose.yml run --rm gateway seed-demo
```

## Smoke Test

### 1. Authenticate

- Call `POST /api/v1/auth/login` with a seeded demo user.
- Save the returned JWT.

### 2. Create a Page

- Call `POST /api/v1/pages`.
- Confirm a new page is returned with `draft_revision_no = 1`.

### 3. Autosave

- Call `PATCH /api/v1/pages/{pageId}/draft` with the returned `base_revision_no`.
- Confirm the response contains the next accepted revision.

### 4. Embed an MWS Table

- Add a table embed block referencing a seeded `mws_table_id`.
- Confirm the response includes embed metadata and preview cache fields.

### 5. Realtime Collaboration

- Open two WebSocket clients to `/ws/collab`.
- Join the same page from both clients.
- Submit a patch from client A and verify client B receives `patch_accepted`.
- Submit a stale patch from client B and verify it receives `rebase_required`.

### 6. Publish and Search

- Call `POST /api/v1/pages/{pageId}/publish`.
- Query `GET /api/v1/search?q=...` and verify the page is discoverable.
- Query `GET /api/v1/pages/{pageId}/backlinks` after linking another page.

### 7. File Upload

- Request an upload session from `POST /api/v1/files/uploads`.
- Upload directly to MinIO using the returned presigned URL.
- Complete the upload and attach the returned `file_id` to a page draft.

## Observability Checks

- Gateway metrics: `http://localhost:8080/metrics`
- Service health endpoints: `http://localhost:<service-port>/health/ready`
- Prometheus UI: `http://localhost:9090`
- MinIO console: `http://localhost:9001`

## Demo Notes

- If `mws-mock` is stopped, existing pages still open and embedded tables render degraded
  placeholders using cached preview/schema data when available.
- Search/backlinks may lag briefly behind the latest save because they are event-driven.
- New table embed creation fails closed when MWS validation cannot be completed.
