# Changelog

This repository records user-visible and compatibility-relevant changes here.
Released sections use Semantic Versioning; unreleased work remains under
`Unreleased` and does not imply a tag.

## Unreleased

### 2026-09-03 09:52 CDT — Preserve temporal and worker boundaries

Commit: `d800418f2013b8e8e24c61d9baed38e10dffe26e`

Affected files:

- `pkg/jobs/values.go`
- `pkg/jobs/rows.go`
- `pkg/jobs/lifecycle.go`
- `pkg/jobs/operations.go`
- `pkg/jobs/worker.go`
- corresponding unit tests
- `docs/implementation-spec.md`
- `docs/runtime-boundary.md`

Explanation:

Replace range-limited `UnixNano` request fingerprinting with signed seconds
plus nanoseconds, reject database-bound times and durations finer than
PostgreSQL's microsecond precision, and reject malformed or wrongly owned jobs
returned by a custom worker store.

Verification:

- format, vet, unit, race, and fifty repeated race runs
- PostgreSQL 17.10 race integration and concurrency suite
- two fuzz admissions totaling 157,372 executions
- 96.5% local statement coverage
- final performance, clean-clone, external-consumer, and Graphify gates

Risks / non-goals:

- Sub-microsecond database-bound values fail with `ErrInvalid`; they are not
  silently rounded.
- No tag, consumer pin, remote push, live database, or deployment changes.

### 2026-09-03 09:42 CDT — Close admission defects

Commit: `4e76bc508b54e66ea16a7418a052f3d3dd14b049`

Affected files:

- `pkg/jobs/retry.go`
- `pkg/jobs/rows.go`
- `pkg/jobs/values.go`
- `pkg/jobs/values_test.go`

Explanation:

Make zero-initial-delay retry calculation constant-time even for a hostile
attempt number, and correct two complexity contracts that falsely counted
payload-byte scans performed elsewhere.

Verification:

- format, vet, unit, race, and fifty repeated race runs
- PostgreSQL 17.10 race integration
- two fuzz admissions totaling 135,283 executions
- performance matrix and clean-clone external-consumer compilation

Risks / non-goals:

- Delivery remains at least once.
- No tag, consumer pin, remote push, live database, or deployment changes.

### 2026-09-03 09:25 CDT — Implement the durable PostgreSQL job engine

Commit: `64a053d51583854f13d8000c42345a645c993bf4`

Affected files:

- `go.mod`
- `go.sum`
- `pkg/jobs/**`
- `docs/runtime-boundary.md`
- `docs/performance.md`
- `docs/verification.md`
- `workflow/COVERAGE.md`

Explanation:

Implement the first complete reusable library boundary: immutable PostgreSQL
schema, atomic and idempotent enqueue, bounded nonblocking claim, database-clock
leases, random fencing tokens, heartbeats, completion and failure transitions,
cooperative cancellation, bounded retries, dead-letter pagination and redrive,
queue counts, and a serial worker with panic containment. Real PostgreSQL tests
caught and closed nil-payload encoding, ambiguous SQL, and rolled-back
lease-reaping defects that fake rows could not reveal.

Verification:

- `go test ./pkg/jobs`
- `go vet ./pkg/jobs`
- `go test -race ./pkg/jobs`
- PostgreSQL 17.10 integration and concurrency suite
- two five-second fuzz admissions totaling 147,908 executions
- 96.4% statement coverage with explicit residual gaps
- PostgreSQL performance workload matrix

Risks / non-goals:

- Delivery is at least once; external side effects remain consumer-idempotent.
- No tag, consumer pin, remote push, live database, or deployment changes.

### 2026-09-03 08:57 CDT — Define the durable job library contract

Commit: `a1835f320f66107545f58a6462a17ee9d97cf95f`

Affected files:

- `LICENSE`
- `README.md`
- `CONTRIBUTING.md`
- `SECURITY.md`
- `Makefile`
- `go.mod`
- `docs/**`
- `workflow.toml`
- `workflow/**`

Explanation:

Replace the placeholder claim with explicit requirements, architecture,
runtime boundaries, implementation units, workflow state, and the maintainer's
MIT licensing decision. The admitted mechanism is PostgreSQL 17 with
at-least-once execution and lease fencing; it does not promise exactly-once
external effects or invent a generic backend framework.

Verification:

- PostgreSQL 17 locking and transaction documentation reviewed
- pgx 5.10.0 transaction source contract reviewed
- documentation and workflow consistency inspection

Risks / non-goals:

- Implementation and verification remain pending.
- No tag, consumer pin, remote push, live database, or deployment changes.

### 2026-09-03 00:42 CDT — Establish GitHub public distribution

Commit: `0244f3cfc76058ebc0de0aade689982bdbefa6f5`

Affected files:

- `README.md`
- `CONTRIBUTING.md`
- `SECURITY.md`
- `docs/distribution.md`
- `docs/RELEASING.md`

Explanation:

Declare GitHub as the public distribution endpoint while retaining Forgejo as
canonical development, define maturity and support honestly, and document the
independent release process. The reserved namespace remains a documentation-only placeholder and makes no API or release claim.

Verification:

- exact old-import search
- documentation contract audit

Risks / non-goals:

- No license is selected.
- No existing tag is changed and no new release is created.
- Mirror direction, repository ownership, and account type are unchanged.
