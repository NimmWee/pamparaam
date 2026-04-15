<!--
Sync Impact Report
- Version change: template -> 1.0.0
- Modified principles:
  - Principle slot 1 -> I. Service Boundaries Own Business Capabilities
  - Principle slot 2 -> II. Clean Architecture Is Enforced Per Service
  - Principle slot 3 -> III. Realtime Collaboration Is a Core Product Capability
  - Principle slot 4 -> IV. Data Integrity and Event Delivery Are Non-Negotiable
  - Principle slot 5 -> V. Operability, Security, and Demo Readiness Are Required
- Added sections:
  - Mandatory Technical Standards
  - Delivery Workflow & Quality Gates
- Removed sections: none
- Templates requiring updates:
  - updated: .specify/templates/plan-template.md
  - updated: .specify/templates/spec-template.md
  - updated: .specify/templates/tasks-template.md
  - pending: .specify/templates/commands/*.md (directory not present; no update applied)
- Follow-up TODOs: none
-->
# MTC Backend Constitution

## Core Principles

### I. Service Boundaries Own Business Capabilities
The system MUST use microservice architecture at the system level. Each service MUST own
its bounded context, data model, and persistence. Cross-service data access is forbidden;
integration MUST occur through explicit REST, WebSocket, gRPC, or NATS contracts. Shared
libraries MAY provide infrastructure helpers, but they MUST NOT centralize domain behavior
that belongs to a service.

Rationale: ownership boundaries keep the MVP evolvable without hidden coupling and make
demo failures easier to isolate.

### II. Clean Architecture Is Enforced Per Service
Every service MUST be structured around domain, use case, ports, and adapters. Business
rules MUST live in domain and use case layers; HTTP and gRPC handlers MUST remain thin
transport adapters. SQL and persistence details MUST stay out of the domain layer and flow
through repository or port abstractions. Dependency direction MUST point inward toward the
domain.

Rationale: this keeps business logic testable, transport-agnostic, and maintainable under
hackathon time pressure.

### III. Realtime Collaboration Is a Core Product Capability
The product MUST implement all mandatory collaboration features: live embedded MWS Tables,
inline autosave, local sync with backend, backlinks, slash-menu metadata support, and
realtime collaboration with presence, patch sync, and conflict handling. Public client
integrations MUST use REST for request-response APIs and WebSocket for collaborative
realtime flows. Internal synchronous service communication SHOULD use gRPC when it reduces
latency or clarifies service contracts.

Rationale: these capabilities define the MVP itself; they are not optional enhancements.

### IV. Data Integrity and Event Delivery Are Non-Negotiable
Document workflows MUST use optimistic concurrency control, a draft-plus-published model,
and revision-based autosave. Services that publish domain events MUST use the outbox
pattern so persistence and event emission remain reliable. State changes that affect
collaboration or projections MUST be versioned or revisioned so conflicts can be detected,
replayed, and resolved deterministically.

Rationale: collaboration features fail visibly without concurrency control and reliable
event delivery.

### V. Operability, Security, and Demo Readiness Are Required
All services MUST emit structured logs, Prometheus metrics, correlation IDs, and health and
readiness endpoints. Authentication MUST use JWT, authorization MUST support RBAC at
workspace and page scope, and all external inputs MUST be validated. Sensitive data MUST
NOT appear in logs. The full MVP MUST be runnable and demonstrable through Docker Compose,
with all mandatory dependencies wired for an end-to-end demo.

Rationale: a production-grade backend is not credible without observability, access
control, and a reproducible demo environment.

## Mandatory Technical Standards

- Implementation language MUST be Golang.
- Primary relational storage MUST be PostgreSQL.
- Redis MUST be used for cache, presence, locking, and pub/sub concerns where those
  concerns are required.
- NATS MUST be the event bus for asynchronous service communication.
- MinIO MUST back S3-compatible object storage concerns.
- Docker and Docker Compose MUST define the default local and demo runtime.
- Public APIs MUST be RESTful unless a stronger protocol-specific reason is documented.
- Realtime client collaboration channels MUST use WebSocket.
- Internal APIs MAY use gRPC where latency, schema control, or streaming justify it.

## Delivery Workflow & Quality Gates

- Every feature spec and plan MUST identify the owning service, bounded context, storage
  owner, external contracts, and required mandatory features.
- Implementation plans MUST fail constitution review if they place business logic in
  handlers, couple services through database reads, omit observability or security work, or
  bypass the required platform stack without written justification.
- Tasks MUST include work for contract surfaces, concurrency behavior, autosave and sync
  semantics, outbox-backed event propagation, RBAC, observability, and Docker Compose demo
  validation when those concerns are affected.
- Pull requests and reviews MUST verify end-to-end behavior for the mandatory MVP features
  before merge readiness is claimed.
- Prefer the simplest design that satisfies the mandatory feature set; speculative
  abstractions are prohibited unless they directly reduce MVP delivery risk.

## Governance

This constitution supersedes local team habits and ad hoc implementation shortcuts for this
repository. Amendments MUST be made through documented updates to this file and any
impacted templates in `.specify/templates/`. Versioning follows semantic rules: MAJOR for
backward-incompatible governance changes or principle removals, MINOR for new principles or
materially expanded mandates, and PATCH for clarifications that do not change engineering
obligations. Compliance MUST be checked during planning, task generation, code review, and
pre-demo verification. Any exception MUST document the violated rule, business reason, and
why a simpler compliant alternative was rejected.

**Version**: 1.0.0 | **Ratified**: 2026-04-13 | **Last Amended**: 2026-04-13
