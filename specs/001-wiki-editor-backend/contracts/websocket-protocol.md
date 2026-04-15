# WebSocket Protocol: Realtime Collaboration

## Endpoint

- `GET /ws/collab?page_id={pageId}&workspace_id={workspaceId}`
- Auth during upgrade: `Authorization: Bearer <jwt>`
- Correlation header: `X-Request-Id`
- Gateway terminates upgrade and proxies the socket to Collaboration Service

## Session Rules

- Room scope is `workspace_id + page_id`
- Server-authoritative ordering: accepted patches define the next revision
- Every patch references `base_revision_no`
- Presence is ephemeral and Redis-backed
- Reconnect never trusts local state automatically; the client must bootstrap again

## Message Envelope

```json
{
  "type": "submit_patch",
  "request_id": "req-123",
  "sent_at": "2026-04-13T20:00:00Z",
  "payload": {}
}
```

Fields:

- `type`: event name
- `request_id`: per-message correlation ID
- `sent_at`: producer timestamp
- `payload`: event-specific body

## Bootstrap Flow

1. Client opens the socket with JWT and page/workspace query params
2. Client sends `join_session`
3. Server authorizes `page.view` and loads the current draft head
4. Server replies with:
   - `session_joined` and `presence_state`, or
   - `rebase_required`, or
   - `error`

## Client -> Server Events

### `join_session`

```json
{
  "type": "join_session",
  "request_id": "req-join",
  "sent_at": "2026-04-13T20:00:00Z",
  "payload": {
    "page_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
    "workspace_id": "fcb4f02d-2444-4ee1-892e-2c73056c7351",
    "last_known_revision_no": 17,
    "last_known_patch_id": "patch-019"
  }
}
```

### `heartbeat`

Sent every 15 seconds.

```json
{
  "type": "heartbeat",
  "request_id": "req-heartbeat",
  "sent_at": "2026-04-13T20:00:15Z",
  "payload": {
    "session_id": "sess-9d5b4250",
    "cursor": {
      "block_id": "blk-14",
      "offset": 28
    }
  }
}
```

### `update_presence`

```json
{
  "type": "update_presence",
  "request_id": "req-presence",
  "sent_at": "2026-04-13T20:00:18Z",
  "payload": {
    "session_id": "sess-9d5b4250",
    "cursor": {
      "block_id": "blk-14",
      "offset": 28
    },
    "selection": {
      "from_block_id": "blk-14",
      "to_block_id": "blk-14"
    }
  }
}
```

### `submit_patch`

Allowed MVP ops:

- `insert_block`
- `replace_block_text`
- `replace_block_attrs`
- `move_block`
- `delete_block`
- `replace_embed_config`

```json
{
  "type": "submit_patch",
  "request_id": "req-patch",
  "sent_at": "2026-04-13T20:00:20Z",
  "payload": {
    "session_id": "sess-9d5b4250",
    "page_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
    "base_revision_no": 17,
    "patch_id": "patch-020",
    "ops": [
      {
        "op": "replace_block_text",
        "block_id": "blk-14",
        "value": "Updated paragraph text"
      }
    ]
  }
}
```

### `leave_session`

```json
{
  "type": "leave_session",
  "request_id": "req-leave",
  "sent_at": "2026-04-13T20:05:00Z",
  "payload": {
    "session_id": "sess-9d5b4250"
  }
}
```

## Server -> Client Events

### `session_joined`

```json
{
  "type": "session_joined",
  "request_id": "req-join",
  "sent_at": "2026-04-13T20:00:00Z",
  "payload": {
    "session_id": "sess-9d5b4250",
    "page_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
    "workspace_id": "fcb4f02d-2444-4ee1-892e-2c73056c7351",
    "current_revision_no": 17,
    "current_revision_id": "4f5c83f8-578f-4738-93f5-8c0c7c87f15d",
    "document": {
      "blocks": []
    },
    "heartbeat_interval_seconds": 15,
    "presence_ttl_seconds": 45
  }
}
```

### `presence_state`

```json
{
  "type": "presence_state",
  "request_id": "req-join",
  "sent_at": "2026-04-13T20:00:01Z",
  "payload": {
    "session_id": "sess-9d5b4250",
    "members": [
      {
        "user_id": "fefcd815-1c12-4bda-bef8-9f08c4802d1b",
        "display_name": "Demo User",
        "cursor": {
          "block_id": "blk-14",
          "offset": 28
        }
      }
    ]
  }
}
```

### `presence_changed`

```json
{
  "type": "presence_changed",
  "request_id": "req-presence-2",
  "sent_at": "2026-04-13T20:00:19Z",
  "payload": {
    "session_id": "sess-9d5b4250",
    "event": "joined",
    "member": {
      "user_id": "2d4ca2bb-8c16-4810-bcd1-cdcb82b3104f",
      "display_name": "Editor Two"
    }
  }
}
```

### `patch_accepted`

```json
{
  "type": "patch_accepted",
  "request_id": "req-patch",
  "sent_at": "2026-04-13T20:00:20Z",
  "payload": {
    "session_id": "sess-9d5b4250",
    "page_id": "26d8b09f-c4c1-4715-bd6d-fdbff945f290",
    "accepted_revision_no": 18,
    "accepted_revision_id": "4078d2db-2b9e-481d-b467-f8051afc5073",
    "patch_id": "patch-020",
    "ops": [
      {
        "op": "replace_block_text",
        "block_id": "blk-14",
        "value": "Updated paragraph text"
      }
    ]
  }
}
```

### `patch_rejected`

```json
{
  "type": "patch_rejected",
  "request_id": "req-patch",
  "sent_at": "2026-04-13T20:00:20Z",
  "payload": {
    "session_id": "sess-9d5b4250",
    "patch_id": "patch-020",
    "reason": "validation_failed",
    "details": {
      "block_id": "blk-14",
      "message": "target block does not exist"
    }
  }
}
```

### `rebase_required`

```json
{
  "type": "rebase_required",
  "request_id": "req-patch",
  "sent_at": "2026-04-13T20:00:21Z",
  "payload": {
    "session_id": "sess-9d5b4250",
    "reason": "stale_patch",
    "latest_revision_no": 18,
    "latest_revision_id": "4078d2db-2b9e-481d-b467-f8051afc5073",
    "server_document": {
      "blocks": []
    },
    "conflicting_patch_id": "patch-020"
  }
}
```

### `pong`

```json
{
  "type": "pong",
  "request_id": "req-heartbeat",
  "sent_at": "2026-04-13T20:00:15Z",
  "payload": {
    "session_id": "sess-9d5b4250",
    "received_at": "2026-04-13T20:00:15Z"
  }
}
```

### `error`

```json
{
  "type": "error",
  "request_id": "req-patch",
  "sent_at": "2026-04-13T20:00:21Z",
  "payload": {
    "code": "forbidden",
    "message": "page edit permission is required",
    "retryable": false
  }
}
```

## Error Codes

| Code | Meaning | Retryable |
|------|---------|-----------|
| `invalid_payload` | Envelope or payload shape is invalid | false |
| `unauthenticated` | JWT missing or invalid | false |
| `forbidden` | Caller cannot join or edit the page | false |
| `session_not_found` | Session expired or unknown | true |
| `validation_failed` | Patch is structurally invalid | false |
| `stale_patch` | Client base revision is older than session head | true |
| `page_unavailable` | Page Service or session head fetch failed | true |

## Heartbeat and Reconnect

- Clients send `heartbeat` every 15 seconds
- Presence expires after 45 seconds without heartbeat
- Reconnecting clients send `join_session` again
- If the last known revision is stale, the server returns `rebase_required`
