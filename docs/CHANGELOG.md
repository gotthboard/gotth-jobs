# Changelog

This repository records user-visible and compatibility-relevant changes here.
Released sections use Semantic Versioning; unreleased work remains under
`Unreleased` and does not imply a tag.

## Unreleased

### 2026-09-06 08:25 CDT - Bound Worker failure normalization

Implementation source: `b54c0fcabb5f7f43e3268749f75a59fbfd27413d`

Affected files:

- Worker failure-message preparation and complexity comments
- focused output/allocation regression
- runtime, performance, verification, coverage, and workflow evidence

Explanation:

Worker now calls a handler error's `Error()` method once and caps its returned
source before UTF-8 normalization or NUL redaction. Normalization operates on
only a `MaxFailureBytes`-scale prefix, truncation reserves the ellipsis within
the final limit, and invalid UTF-8 expansion cannot make library work or
allocation depend on the complete source length. Final failure text remains at
most 4 KiB, valid UTF-8, NUL-free, redacted, and explicitly truncated.

Verification:

- expected-red 1 MiB valid, alternating-invalid, and NUL allocation regression
- constrained local focused repeats, full package, vet, format, and integration
  compilation
- exact clean-source format, vet, unit, build, full race, 50 focused race
  repeats, 20 allocation runs, 100-repeat focused timing, bounded-failure fuzz,
  and 97.5% statement coverage on `development`
- exact-source Graphify extraction/diagnosis; `boundedFailure` is 100% covered

Risks / non-goals:

- Work performed inside a consumer-defined `Error()` method remains
  consumer-owned; the library bound begins after that single call returns.
- PostgreSQL, external-consumer, and PostgreSQL performance gates were not
  rerun because no SQL, Store contract, public API, or successful Worker path
  changed. Existing evidence remains ancestor evidence only.
- Two fresh independent reviews remain orchestrator-owned. This repair does
  not claim final admission.
- No push, merge, tag, release, pull request, deployment, or remote change.

### 2026-09-06 07:23 CDT - Reject malformed scalar job states

Implementation source: `4ec1970ed632f0306cc772bceeae8e15e17f5ab6`

Affected files:

- lease-state classification and bounded scanner tests
- count completeness query and malformed-state tests
- PostgreSQL query-mode allocation regression
- public contracts, verification evidence, and workflow history

Explanation:

Lease failure classification now scans stored state through pgx's borrowed-byte
hook with the longest allowed state as its bound before making one bounded
string conversion. Only the five state-machine values are classified; NULL,
oversized, and unknown values are returned as stored-data errors rather than
ordinary lease loss. `Counts` now reads the total row count with the five known
buckets and fails if their sum differs, preventing malformed rows from being
silently omitted.

Verification:

- expected-red unit and real-pgx allocation evidence against rejected
  candidate `2486b4732076976d4565e235de1d97c36b361951`
- constrained local focused repeats, package, vet, formatting, and integration
  compilation
- retained exact-source `set -x` transcript for format, vet, unit, build, full
  race, 50 affected race repeats, fuzz, and 97.4% statement coverage
- PostgreSQL 17.10 race/coverage, three five-mode allocation runs, ten focused
  race repeats, full performance matrix, and retained external-consumer source
  plus exact invocation
- `classifyLease` and `Counts` are 100% covered

Risks / non-goals:

- Corrupt stored data remains a storage error rather than a new public error
  identity; existing public API and at-least-once behavior are unchanged.
- pgx/network buffers remain runtime storage; the bound prevents a second
  source-sized ownership conversion by the library scanner.
- Two fresh independent reviews remain orchestrator-owned. This repair does
  not claim final admission.
- No push, merge, tag, release, pull request, deployment, or remote change.

### 2026-09-06 06:27 CDT - Bound stored rows and harden integration reset

Implementation source: `9711e2b00dc95ae3070090745d611b65eca686f3`

Affected files:

- Worker heartbeat reconciliation ordering and deterministic race tests
- bounded pgx row scanners and malformed-row tests
- destructive PostgreSQL integration target guard and runner contract
- PostgreSQL mode/allocation tests, documentation, and workflow evidence

Explanation:

Worker now gives an unknown Heartbeat commit outcome immediate precedence
after heartbeat join over both parent and local teardown cancellation, always
returning the secret-safe `LeaseReconciliationError`. Every untrusted stored
text, nullable-text, payload, and fingerprint field is bounded through pgx's
borrowed-byte scanner hook before one ownership conversion. Nullable presence
is retained, present-empty key/token/owner fields are rejected, fingerprints
must be exactly 32 bytes, and non-running jobs require a wholly zero Lease.
Destructive integration reset now requires an explicit opt-in plus the exact
database name and an exact PostgreSQL database-comment marker before DDL.

Verification:

- expected-red heartbeat, scanner, nullable-presence, fingerprint, and reset
  authorization regressions
- constrained local package, focused repeat, vet, and integration compile
- exact clean-source format, vet, unit, build, full race, 50 affected race
  repeats, fuzz, and 97.4% statement coverage on `development`
- PostgreSQL 17.10 race/coverage, three five-mode payload/text/fingerprint
  allocation runs, ten safety/isolation/key-lock repeats, and performance
- standalone external-consumer test/build; every changed production path is
  100% covered

Risks / non-goals:

- pgx/network buffers remain runtime storage; the one-conversion claim is the
  library's ownership conversion after source-length validation.
- Integration authorization is intentionally exact and requires disposable
  database setup before `make verify-integration`.
- Two fresh independent reviews remain orchestrator-owned. This repair does
  not claim final admission.
- No push, merge, tag, release, pull request, deployment, or remote change.

### 2026-09-06 05:22 CDT - Repair acknowledgement and PostgreSQL boundaries

Implementation source: `1fc2a7b3db6edd354e7efa4154f87032866cb090`

Affected files:

- Worker acknowledgement reconciliation and renewal interval validation
- EnqueueTx caller isolation inspection
- pgx job query result-mode enforcement and stored-row validation
- focused unit, PostgreSQL, public API, and external-consumer tests
- architecture, runtime, performance, verification, and workflow records

Explanation:

Worker now returns secret-safe `LeaseReconciliationError` values for unknown
Heartbeat, Complete, and Fail commit outcomes, retaining the exact affected
job and lease without retrying. Heartbeat intervals may not exceed half the
lease. EnqueueTx accepts only Read Committed and never retries a caller-owned
transaction. Every job-returning query forces DescribeExec and binary formats
for all job-column OIDs, so payload size checks run before ownership allocation
under every pgx default mode and full-range timestamps stay on binary decoding.
Stored SQL NULL payloads, missing mandatory timestamps, and invalid mandatory
or optional timestamps are rejected.

Verification:

- expected-red unit and real-pgx query-mode/isolation evidence
- constrained local focused repeats, package, vet, and integration compile
- exact clean-source format, vet, unit, build, full race, 50 affected race
  repeats, fuzz, and 97.3% statement coverage on `development`
- PostgreSQL 17.10 race/coverage, three five-mode allocation runs, and ten
  race-instrumented isolation/key-retention repeats
- exact-source standalone external-consumer test/build and full performance
  matrix
- every defect-specific path is covered; exact unrelated/preexisting gaps are
  recorded in verification evidence

Risks / non-goals:

- DescribeExec adds one round trip relative to cached extended execution; the
  measured cost is disclosed and no speedup is claimed.
- The half-lease budget is not a hard guarantee across arbitrary pauses.
- Delivery remains at least once and handler idempotency remains
  consumer-owned.
- Two fresh independent reviews remain orchestrator-owned. This repair does
  not claim final admission.
- No push, merge, tag, release, pull request, deployment, or remote change.

### 2026-09-06 04:08 CDT — Bound heartbeat and row payload costs

Commit: `53cf140090cb7c1bc2076579437aab8edd3a0229`

Affected files:

- heartbeat SQL, public Store/PostgreSQL contract, Worker, and fakes
- idempotency index and conflict insert
- shared PostgreSQL row scanner
- migration, unit, PostgreSQL, public API, and external-consumer tests
- architecture, runtime, performance, verification, and workflow records

Explanation:

Heartbeat now returns only an error and its SQL returns one boolean instead of
the complete job, keeping response traffic and allocation independent of
payload size. The idempotency index is non-partial so `FOR KEY SHARE` blocks
queue/key changes; PostgreSQL's default distinct-`NULL` uniqueness continues
to allow unkeyed jobs. Job payloads now scan through a bounded owning
`pgtype.BytesScanner` that checks source length before allocation and copies an
accepted bytea exactly once.

Verification:

- expected-red scalar-heartbeat, partial-index, and row-allocation regressions
- focused local package tests, 10 repeats, vet, and integration-tag compile
- exact clean-source format, vet, unit, build, full race, 50 affected race
  repeats, fuzz, and 97.0% statement coverage on `development`
- PostgreSQL 17.10 race and coverage integration, 10 race-instrumented
  key-update lock repeats, and three heartbeat allocation repeats
- exact-source standalone external-consumer test/build and performance matrix
- every materially changed production function is 100% covered

Risks / non-goals:

- Heartbeat's unreleased return signature changes from `(Job, error)` to
  `error`; Complete and Fail still return the transitioned Job.
- The migration is unreleased and immutable; no live schema was upgraded.
- Delivery remains at least once and handler idempotency remains
  consumer-owned.
- Fresh independent reviews remain orchestrator-owned. This repair does not
  claim final admission.
- No push, merge, tag, release, pull request, deployment, or remote change.

### 2026-09-06 03:18 CDT — Bind idempotency snapshots and Worker reconciliation

Commit: `62d565aa1d3f4ebf19cc4d39bf87d2764c676c8b`

Affected files:

- enqueue preparation, conflict fallback, and shared row scanning
- `ClaimReconciliationError` and `Worker.Run`
- focused unit, PostgreSQL, public API, and external-consumer tests
- public contract, performance, verification, and workflow records

Explanation:

Read an idempotent duplicate's fingerprint and complete job from one row and
Read Committed statement snapshot, retaining that row identity with
`FOR KEY SHARE` through transaction end. Validate all enqueue bounds before
copying payload bytes; library-owned Enqueue now prepares one bounded copy
before `BeginTx` and executes a private prepared helper without recopying.
When Worker receives a nonzero Claim result with `ErrCommitOutcomeUnknown`,
return `ClaimReconciliationError` with the reconciliation-only Job while
preserving error traversal and omitting the lease token from error text.

Verification:

- expected-red replacement-interleaving, oversized allocation/transaction,
  and Worker value-loss regressions
- focused local package tests, 10 repeats, vet, and integration-tag compile
  with `GOMAXPROCS=2` and `-p=1`
- exact clean-source format, vet, unit, build, full race, 50 affected race
  repeats, and 96.9% statement coverage on `development`
- PostgreSQL 17.10 race and coverage integration plus 10 repeated row-lock
  interleavings against the pinned image digest
- exact-source standalone external-consumer test/build and performance matrix
- every defect-specific path is covered; exact unrelated/preexisting gaps are
  recorded in verification evidence

Risks / non-goals:

- The retaining read can wait behind a writer of the conflicting row; it does
  not promise exactly-once work or external side effects.
- A reconciliation Job does not authorize handling until durable ID/token
  state is confirmed.
- Fuzz and graph gates were not invalidated and remain ancestor evidence.
- Two fresh independent reviews remain orchestrator-owned. This repair does
  not claim final admission.
- No push, merge, tag, release, pull request, deployment, or remote change.

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
