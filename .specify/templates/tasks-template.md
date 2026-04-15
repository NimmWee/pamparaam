---

description: "Task list template for feature implementation"
---

# Tasks: [FEATURE NAME]

**Input**: Design documents from `/specs/[###-feature-name]/`
**Prerequisites**: plan.md (required), spec.md (required for user stories),
research.md, data-model.md, contracts/

**Tests**: Include verification tasks for contracts, realtime behavior, concurrency,
security, and Docker Compose smoke checks whenever the feature touches those concerns.

**Organization**: Tasks are grouped by user story to enable independent
implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- Services live under `services/[service-name]/`
- Shared packages live under `pkg/`
- Deploy/runtime assets live under `deploy/`
- Cross-service and end-to-end verification lives under `tests/`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [ ] T001 Create or update service directories per implementation plan
- [ ] T002 Initialize or update Go module dependencies required by the feature
- [ ] T003 [P] Wire local dependencies in `deploy/docker-compose.yml`
- [ ] T004 [P] Define or update service configuration, env handling, and secrets loading

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story work

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T005 Define PostgreSQL schema changes and migrations for the owning service
- [ ] T006 [P] Implement repository/port changes so domain logic stays free of SQL
- [ ] T007 [P] Define or update REST, WebSocket, gRPC, and NATS contracts impacted by the feature
- [ ] T008 [P] Implement JWT auth, workspace/page RBAC, and input validation updates
- [ ] T009 [P] Add structured logging, Prometheus metrics, health/readiness endpoints, and correlation ID propagation
- [ ] T010 [P] Implement Redis, NATS, and MinIO integration changes required by the feature
- [ ] T011 Implement optimistic concurrency, revision tracking, and draft/published state handling where applicable
- [ ] T012 Implement outbox-backed event publishing where the feature emits domain events

**Checkpoint**: Foundation ready; user story implementation can now begin

---

## Phase 3: User Story 1 - [Title] (Priority: P1) MVP

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Verification for User Story 1

- [ ] T013 [P] [US1] Add or update contract tests in `tests/contract/`
- [ ] T014 [P] [US1] Add or update integration tests in `tests/integration/`
- [ ] T015 [P] [US1] Add or update realtime/conflict tests in `tests/realtime/` when applicable

### Implementation for User Story 1

- [ ] T016 [P] [US1] Implement domain changes in `services/[service-name]/internal/domain/`
- [ ] T017 [P] [US1] Implement use-case orchestration in `services/[service-name]/internal/usecase/`
- [ ] T018 [P] [US1] Implement adapter/repository changes in `services/[service-name]/internal/adapters/`
- [ ] T019 [US1] Implement transport handlers in `services/[service-name]/api/`
- [ ] T020 [US1] Add observability and authorization coverage for the story flow
- [ ] T021 [US1] Validate story behavior through Docker Compose

**Checkpoint**: User Story 1 should be fully functional and independently testable

---

## Phase 4: User Story 2 - [Title] (Priority: P2)

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Verification for User Story 2

- [ ] T022 [P] [US2] Add or update contract tests in `tests/contract/`
- [ ] T023 [P] [US2] Add or update integration tests in `tests/integration/`
- [ ] T024 [P] [US2] Add or update realtime/conflict tests in `tests/realtime/` when applicable

### Implementation for User Story 2

- [ ] T025 [P] [US2] Implement domain and use-case changes in the owning service
- [ ] T026 [P] [US2] Implement transport and messaging contract changes
- [ ] T027 [US2] Integrate with prior stories without violating service ownership boundaries
- [ ] T028 [US2] Validate story behavior through Docker Compose

**Checkpoint**: User Stories 1 and 2 should both work independently

---

## Phase 5: User Story 3 - [Title] (Priority: P3)

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Verification for User Story 3

- [ ] T029 [P] [US3] Add or update contract tests in `tests/contract/`
- [ ] T030 [P] [US3] Add or update integration tests in `tests/integration/`
- [ ] T031 [P] [US3] Add or update realtime/conflict tests in `tests/realtime/` when applicable

### Implementation for User Story 3

- [ ] T032 [P] [US3] Implement domain and use-case changes in the owning service
- [ ] T033 [P] [US3] Implement transport and messaging contract changes
- [ ] T034 [US3] Add observability, security, and compose validation for the story flow

**Checkpoint**: All user stories should now be independently functional

---

## Phase N: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] TXXX [P] Update docs and quickstart files impacted by the feature
- [ ] TXXX Validate dashboards, metrics, logs, and health/readiness endpoints
- [ ] TXXX [P] Harden authorization, validation, and sensitive-log handling
- [ ] TXXX Validate Docker Compose demo flow end to end
- [ ] TXXX Run regression checks for mandatory collaboration capabilities affected by the feature

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion; blocks all user stories
- **User Stories (Phase 3+)**: Depend on Foundational completion
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- User stories should remain independently testable even when they share infrastructure
- Cross-story integration MUST happen through contracts, not direct database access
- Realtime and autosave features MUST include conflict and recovery verification where affected

### Within Each User Story

- Domain and use-case changes before transport wiring
- Contracts before cross-service integration
- Observability and authorization checks before story sign-off
- Compose-based validation before completion claims

### Parallel Opportunities

- Setup tasks marked [P] can run in parallel
- Foundational tasks marked [P] can run in parallel when they touch different files
- Once Foundational is complete, independent user stories can proceed in parallel
- Verification tasks for a story marked [P] can run in parallel

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. Validate User Story 1 independently in Docker Compose
5. Stop and demo if ready

### Incremental Delivery

1. Complete Setup and Foundational work
2. Deliver User Story 1 and validate it independently
3. Add User Story 2 and re-run affected contract, integration, and realtime checks
4. Add User Story 3 and re-run affected contract, integration, and realtime checks
5. End with an end-to-end Docker Compose demo pass

## Notes

- [P] tasks = different files, no dependencies
- [Story] labels map tasks to user stories for traceability
- Keep business logic out of handlers and SQL out of the domain layer
- Prefer the simplest implementation that still satisfies mandatory MVP capabilities
- Do not mark work complete without compose-based verification for impacted flows
