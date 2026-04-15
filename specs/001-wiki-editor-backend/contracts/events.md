# Events Contract: Domain Events over NATS

## Subjects

- `auth.membership.changed`
- `auth.page_acl.changed`
- `page.created`
- `page.draft.saved`
- `page.published`
- `page.archived`
- `page.links.extracted`
- `page.attachments.changed`
- `collab.session.started`
- `collab.presence.changed`
- `collab.patch.accepted`
- `collab.patch.rejected`
- `collab.session.ended`
- `files.upload.completed`
- `files.deleted`
- `mws.embed.resolved`
- `mws.embed.refreshed`
- `mws.embed.degraded`
- `search.reindex.failed`

## Envelope

All messages share this envelope:

```json
{
  "event_id": "1e1ded75-bfc8-4a1d-8c82-6898f119d3a5",
  "event_type": "page.draft.saved",
  "aggregate_type": "page",
  "aggregate_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
  "occurred_at": "2026-04-13T20:00:00Z",
  "correlation_id": "c4c19440-3ff0-4c95-8b8c-a848b99bdb1e",
  "causation_id": "auto-req-17",
  "producer": "page-service",
  "schema_version": 1,
  "payload": {}
}
```

Rules:

- Page-originated domain events publish through the outbox helper
- Consumers are idempotent by `event_id`
- Delivery is at least once
- Projection lag is acceptable; write paths must not block on read-model consumers

## Payloads

### `auth.membership.changed`

```json
{
  "workspace_id": "fcb4f02d-2444-4ee1-892e-2c73056c7351",
  "user_id": "fefcd815-1c12-4bda-bef8-9f08c4802d1b",
  "role": "editor",
  "status": "active"
}
```

### `auth.page_acl.changed`

```json
{
  "page_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
  "workspace_id": "fcb4f02d-2444-4ee1-892e-2c73056c7351",
  "subject_user_id": "fefcd815-1c12-4bda-bef8-9f08c4802d1b",
  "can_view": true,
  "can_edit": false
}
```

### `page.created`

```json
{
  "page_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
  "workspace_id": "fcb4f02d-2444-4ee1-892e-2c73056c7351",
  "title": "MVP Overview",
  "revision_no": 1,
  "revision_id": "4f5c83f8-578f-4738-93f5-8c0c7c87f15d",
  "status": "draft"
}
```

### `page.draft.saved`

```json
{
  "page_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
  "workspace_id": "fcb4f02d-2444-4ee1-892e-2c73056c7351",
  "revision_no": 18,
  "revision_id": "4078d2db-2b9e-481d-b467-f8051afc5073",
  "title": "MVP Overview",
  "saved_via": "rest_autosave",
  "canonical_links": [
    {
      "target_page_id": "0b8b5373-9e90-45fb-9353-b4099ee54874",
      "block_id": "blk-link-3"
    }
  ],
  "embedded_tables": [
    {
      "mws_table_id": "tbl_123",
      "block_id": "blk-table-1"
    }
  ],
  "attachment_ids": [
    "4adc5df5-0ab8-48a3-ba3e-e38d4137b8ea"
  ]
}
```

### `page.published`

```json
{
  "page_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
  "workspace_id": "fcb4f02d-2444-4ee1-892e-2c73056c7351",
  "published_revision_no": 19,
  "published_revision_id": "524f2126-0b29-43cc-9dbf-855235db0637"
}
```

### `page.links.extracted`

```json
{
  "page_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
  "workspace_id": "fcb4f02d-2444-4ee1-892e-2c73056c7351",
  "revision_id": "4078d2db-2b9e-481d-b467-f8051afc5073",
  "links": [
    {
      "target_page_id": "0b8b5373-9e90-45fb-9353-b4099ee54874",
      "block_id": "blk-link-3",
      "link_kind": "page_ref"
    }
  ]
}
```

### `page.attachments.changed`

```json
{
  "page_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
  "workspace_id": "fcb4f02d-2444-4ee1-892e-2c73056c7351",
  "revision_id": "4078d2db-2b9e-481d-b467-f8051afc5073",
  "attachment_ids": [
    "4adc5df5-0ab8-48a3-ba3e-e38d4137b8ea"
  ]
}
```

### `collab.session.started`

```json
{
  "session_id": "sess-9d5b4250",
  "page_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
  "workspace_id": "fcb4f02d-2444-4ee1-892e-2c73056c7351"
}
```

### `collab.presence.changed`

```json
{
  "session_id": "sess-9d5b4250",
  "page_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
  "user_id": "fefcd815-1c12-4bda-bef8-9f08c4802d1b",
  "event": "joined"
}
```

### `collab.patch.accepted`

```json
{
  "session_id": "sess-9d5b4250",
  "page_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
  "patch_id": "patch-020",
  "accepted_revision_no": 18
}
```

### `collab.patch.rejected`

```json
{
  "session_id": "sess-9d5b4250",
  "page_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
  "patch_id": "patch-020",
  "reason": "validation_failed"
}
```

### `files.upload.completed`

```json
{
  "file_id": "4adc5df5-0ab8-48a3-ba3e-e38d4137b8ea",
  "workspace_id": "fcb4f02d-2444-4ee1-892e-2c73056c7351",
  "page_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
  "filename": "diagram.png",
  "content_type": "image/png",
  "size_bytes": 102400
}
```

### `files.deleted`

```json
{
  "file_id": "4adc5df5-0ab8-48a3-ba3e-e38d4137b8ea",
  "workspace_id": "fcb4f02d-2444-4ee1-892e-2c73056c7351"
}
```

### `mws.embed.resolved`

```json
{
  "page_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
  "mws_table_id": "tbl_123",
  "preview_state": "ready",
  "cache_ttl_seconds": 300
}
```

### `mws.embed.refreshed`

```json
{
  "page_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
  "mws_table_id": "tbl_123",
  "preview_state": "cached"
}
```

### `mws.embed.degraded`

```json
{
  "page_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
  "mws_table_id": "tbl_123",
  "reason": "source_unavailable",
  "cache_hit": true
}
```

### `search.reindex.failed`

```json
{
  "page_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
  "revision_id": "4078d2db-2b9e-481d-b467-f8051afc5073",
  "reason": "database_timeout"
}
```
