# Changelog

This repository records user-visible and compatibility-relevant changes here.
Released sections use Semantic Versioning; unreleased work remains under
`Unreleased` and does not imply a tag.

## Unreleased

### 2026-09-06 02:29 CDT — Preserve Claim reconciliation and stop heartbeats

Commit: `6655331ae4e3b7509b826a03db11c36cee9a6ca2`

Affected files:

- `pkg/jobs/lifecycle.go`
- `pkg/jobs/worker.go`
- focused lifecycle, transaction-wrapper, and worker tests
- public contract and workflow evidence documents

Explanation:

Preserve the exact job ID and lease token when Claim produced a candidate but
transaction commit returned `ErrCommitOutcomeUnknown`. That value is
reconciliation-only while the error is non-nil. After a handler returns, stop
and cancel its heartbeat context before joining the heartbeat goroutine;
distinguish that local teardown from parent cancellation and genuine heartbeat
failure so successful work can still be acknowledged.

Verification:

- expected-red tests for a discarded commit-unknown fencing handle and a
  context-aware heartbeat that blocked teardown
- focused local package tests and vet with `GOMAXPROCS=2` and `-p=1`
- exact clean-source format, vet, unit, build, full race, 50 focused race
  repeats, and 96.3% statement coverage on `development`
- PostgreSQL 17.10 race and coverage integration at 96.5% against the pinned
  image digest
- every changed production statement covered; exact preexisting residual
  blocks recorded in verification evidence

Risks / non-goals:

- A Job returned with `ErrCommitOutcomeUnknown` does not authorize handling;
  durable state and the exact token must be confirmed first.
- Delivery remains at least once. No automatic retry was added.
- External-consumer, fuzz, performance, and graph gates were not rerun because
  public signatures, steady-state SQL, and dependency shape are unchanged.
- Two fresh independent clean reviews remain orchestrator-owned. This repair
  does not claim final admission.
- No push, merge, tag, release, pull request, deployment, or remote change.

### 2026-09-06 01:51 CDT — Correct Claim lock-cardinality contract

Commit: `b54cd4f3c385cbe0df1158c2a866efe7fbc216d1`

Affected file:

- `docs/runtime-boundary.md`

Explanation:

Correct the runtime contract to match the existing bounded Claim statement.
One call can lock and update up to 100 expired, exhausted rows plus at most one
disjoint eligible candidate, holding at most 101 row locks until the short
transaction ends. It may perform 100 cleanup writes while returning no job;
the eligible result remains limited to one job.

Verification:

- inspected both disjoint `FOR UPDATE SKIP LOCKED` CTE predicates and limits
- searched canonical documentation and evidence for the rejected false claim
- passed whitespace, workflow-format, and exact-head cleanliness checks

Risks / non-goals:

- No code, SQL, public API, or runtime behavior changed.
- Existing implementation gates remain bound to exact source `9f6acc74`; no
  unrelated heavy gate was rerun for this documentation-only correction.
- Two fresh independent clean reviews remain orchestrator-owned. This repair
  does not claim final admission.
- No push, merge, tag, release, pull request, deployment, or remote change.

### 2026-09-06 01:28 CDT — Validate dead-letter cursor timestamps

Commit: `9f6acc74f8901a58a3a9929d10ad7a3779241f4d`

Affected files:

- `pkg/jobs/values.go`
- `pkg/jobs/operations.go`
- focused unit and PostgreSQL integration tests
- runtime, verification, and workflow records

Explanation:

Use one PostgreSQL timestamp predicate for explicit enqueue availability and
non-nil dead-letter cursors. `ListDead` now rejects finite UTC microsecond
timestamps outside PostgreSQL 17's range before pgx can wrap their binary
encoding, including the independently reported far-future value that encoded
as Y2K.

Verification:

- expected-red cursor regressions for both endpoints, adjacent out-of-range
  values, and the exact wrap-to-Y2K fixture
- focused local tests and vet with `GOMAXPROCS=2` and `-p=1`
- exact clean-source format, vet, unit, build, race, and coverage gates on
  `development`
- PostgreSQL 17.10 race and coverage integration proving an invalid cursor
  returns `ErrInvalid` before wrapped query behavior
- standalone external-consumer test and build against the exact clean source

Risks / non-goals:

- Successful cursor query behavior and at-least-once delivery are unchanged.
- Repeat, fuzz, performance, and graph gates were not rerun for this bounded
  validation repair; prior results remain ancestor evidence only.
- Two fresh independent reviews remain pending and orchestrator-owned. This
  repair does not claim final admission.
- No tag, consumer pin, remote push, merge, release, pull request, live
  database, or deployment changes.

### 2026-09-06 00:51 CDT — Enforce PostgreSQL time and attempt boundaries

Commit: `72c62231fa4a0012ceef0a9c5ff61ff05feaf859`

Affected files:

- `pkg/jobs/values.go`
- `pkg/jobs/rows.go`
- `pkg/jobs/migrations/000001_jobs.sql`
- focused unit, migration, and PostgreSQL integration tests
- runtime, verification, performance, and workflow records

Explanation:

Reject explicit availability outside PostgreSQL 17's finite timestamp range
before pgx can wrap the binary microsecond value. Reject impossible
state/attempt pairs at both the untrusted row boundary and in the unreleased
initial schema: pending attempts remain below the maximum; running, succeeded,
and dead rows require an admitted attempt; canceled rows may remain at zero.

Verification:

- expected-red regressions for the exact pgx wrap fixture, row validation, and
  migration constraint
- focused local tests and vet under Go 1.26.6 with `GOMAXPROCS=2` and `-p=1`
- exact clean-revision unit, race, 50-repeat race, coverage, and fuzz gates on
  `development`
- PostgreSQL 17.10 integration against the pinned image, including exact time
  endpoint round trips and all 16 state/attempt boundary combinations
- performance, external-consumer, clean-clone, and Graphify gates

Risks / non-goals:

- Delivery remains at least once; handler idempotency remains consumer-owned.
- Fresh independent admission reviews remain pending and orchestrator-owned.
- No tag, consumer pin, remote push, live database, or deployment changes.

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
