# Testing Strategy

This project uses layered testing to balance speed and confidence.

## Unit Tests
- **Scope**: core services, validation, error mapping.
- **Location**: `internal/core/services/*_test.go`, `pkg/*`.
- **Goal**: fast feedback for business logic changes.

## Integration Tests
- **Scope**: repository + database, Docker adapters, CLI commands.
- **Location**: `internal/repositories/*_test.go`, `tests/*`.
- **Goal**: verify real dependencies and I/O boundaries.

## End-to-End Tests
- **Scope**: user flows (create -> list -> logs -> delete).
- **Location**: `tests/*` scripts and Go tests.
- **Goal**: ensure full-stack behavior and API contracts.

## Conventions
- Use table-driven tests for input matrices.
- Assert on typed errors and status codes.
- Add negative tests for authorization and validation failures.
