# Implementation Plan: [FEATURE]

**Branch**: `[###-feature-name]` | **Date**: [DATE] | **Spec**: [link]
**Input**: Feature specification from `/specs/[###-feature-name]/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See
`.specify/templates/plan-template.md` for the execution workflow.

## Summary

[Extract from feature spec: primary requirement, owning service, and technical approach]

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the concrete feature
  context. Plans are expected to default to the constitution's backend platform
  unless a justified exception is documented.
-->

**Language/Version**: Go [version or NEEDS CLARIFICATION]  
**Primary Dependencies**: PostgreSQL, Redis, NATS, MinIO, Docker Compose,
[service-specific libraries]  
**Storage**: PostgreSQL as system of record; Redis for cache/presence/locks/pub-sub;
MinIO for object storage if needed  
**Testing**: [unit, integration, contract, realtime, compose smoke]  
**Target Platform**: Containerized Linux services orchestrated by Docker Compose  
**Project Type**: Microservice backend with REST, WebSocket, and selective internal gRPC  
**Performance Goals**: [feature-specific p95 latency, sync freshness, throughput, or NEEDS CLARIFICATION]  
**Constraints**: Mandatory MVP collaboration features, optimistic concurrency, draft/published
model, revision autosave, outbox reliability, no business logic in handlers  
**Scale/Scope**: [owning service, bounded context, expected users/workspaces/pages, or NEEDS CLARIFICATION]

## Service & Contract Context

**Owning Service**: [service name]  
**Bounded Context**: [domain boundary]  
**Data Ownership**: [tables/buckets/streams owned by this service only]  
**Inbound Interfaces**: [REST routes, WebSocket channels, gRPC methods, N/A]  
**Outbound Integrations**: [NATS subjects, gRPC calls, MinIO buckets, Redis usage, N/A]  
**Realtime Impact**: [presence, patch sync, autosave, conflict handling, local sync impact, or N/A]

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [ ] Service ownership is explicit; no shared-database access across services is proposed.
- [ ] The design keeps business logic out of HTTP/gRPC handlers and SQL out of the domain layer.
- [ ] The plan uses the mandated stack or documents a justified exception.
- [ ] REST, WebSocket, gRPC, and NATS contracts are identified where relevant.
- [ ] Optimistic concurrency, draft/published state, revision autosave, and outbox behavior
      are covered where relevant.
- [ ] Observability is planned: structured logs, metrics, health/readiness, correlation IDs.
- [ ] Security is planned: JWT auth, workspace/page RBAC, input validation, secret-safe logging.
- [ ] Docker Compose demo readiness and end-to-end validation are included.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
|-- plan.md              # This file (/speckit.plan command output)
|-- research.md          # Phase 0 output (/speckit.plan command)
|-- data-model.md        # Phase 1 output (/speckit.plan command)
|-- quickstart.md        # Phase 1 output (/speckit.plan command)
|-- contracts/           # Phase 1 output (/speckit.plan command)
`-- tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

<!--
  ACTION REQUIRED: Replace the placeholder tree below with the concrete layout
  for this feature. Delete unused paths and expand the selected structure with
  real service names.
-->

```text
services/
|-- [service-name]/
|   |-- cmd/
|   |-- internal/
|   |   |-- domain/
|   |   |-- usecase/
|   |   |-- ports/
|   |   `-- adapters/
|   |-- api/
|   |   |-- rest/
|   |   |-- websocket/
|   |   `-- grpc/
|   `-- migrations/
|-- [another-service]/
|   `-- ...
pkg/
|-- observability/
|-- auth/
`-- messaging/
deploy/
`-- docker-compose.yml
tests/
|-- contract/
|-- integration/
|-- realtime/
`-- compose/
```

**Structure Decision**: [Document the selected structure and reference the real
directories captured above]

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., direct cross-service DB read] | [current need] | [why API/event integration could not satisfy it] |
| [e.g., business logic in transport layer] | [specific problem] | [why use-case layer could not own the rule] |
