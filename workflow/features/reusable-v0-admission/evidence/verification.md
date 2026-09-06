# Reusable v0 admission evidence

## Identity and disposition

- Baseline: `874212b762571cd88322867872e458af0d9e0435`.
- Audit-rejected candidate: `c64368a202f4af62c33a8640ac0d0923df2e333e`.
- Admission audit: `/tmp/gotth-jobs-admission-audit.md`.
- First repair source: `72c62231fa4a0012ceef0a9c5ff61ff05feaf859`.
- First-review-rejected candidate:
  `671a1eac9ddc6d273136d46de6d906730c7182e5`.
- First independent review: `/tmp/gotth-jobs-independent-judge-1.md`.
- Cursor repair source:
  `9f6acc74f8901a58a3a9929d10ad7a3779241f4d`.
- Second-review-rejected candidate:
  `68e2f24b2f20b0c3905d46e9a228d28f044ca9a4`.
- Second independent review: `/tmp/gotth-jobs-independent-judge-2.md`.
- Runtime-contract repair source:
  `b54cd4f3c385cbe0df1158c2a866efe7fbc216d1`.
- Third-review-rejected candidate:
  `b195469add429001a35b3c9658ecfd52e07c08e3`.
- Third independent review: `/tmp/gotth-jobs-independent-judge-3.md`.
- Current implementation repair source:
  `6655331ae4e3b7509b826a03db11c36cee9a6ca2`.
- Branch: `feature/reusable-v0-admission` in the assigned isolated worktree.
- State: active. This repair worker does not claim independent final admission.
- No tag, Git remote configuration, push, merge, release, pull request,
  deployment, live database, secret, or consumer was changed.

The admission audit found unbounded enqueue timestamp encoding and impossible
state/attempt combinations. The first independent review then found the same
PostgreSQL encoding boundary missing from non-nil `DeadCursor.FinishedAt`.
Those implementation defects are repaired. The second independent review
found that the runtime contract falsely described Claim as locking exactly one
row. That documentation defect is corrected. The third independent review
found that Claim discarded its commit-unknown reconciliation handle and that
handler completion could hang joining a context-aware heartbeat. Both defects
are repaired. The historical and rejected reviews do not admit the current
tree; two fresh attributable
orchestrator-owned reviews of the final candidate remain required.

## Contracts checked

The worker read the full admission audit, all three independent review reports, PRD,
architecture, implementation specification, runtime boundary, workflow plan,
manifest, records, and prior evidence before each bounded repair.

PostgreSQL 17 accepts finite `timestamptz` values from
`-4713-11-24T00:00:00Z` through `294276-12-31T23:59:59.999999Z` in Go's
proleptic Gregorian calendar. pgx 5.10.0 performs binary timestamp arithmetic
in `int64` and does not reject every out-of-range finite `time.Time`; the exact
reported fixture can wrap to Y2K. One internal predicate now enforces nonzero
UTC, microsecond precision, and the inclusive PostgreSQL endpoints for both
explicit enqueue availability and non-nil dead-letter cursors. The public API
and successful-query semantics are unchanged.

## Runtime-contract correction

The `expired_exhausted` CTE uses `FOR UPDATE SKIP LOCKED LIMIT 100` for running
rows with `attempts >= max_attempts`. The disjoint `candidate` CTE uses
`FOR UPDATE SKIP LOCKED LIMIT 1` for pending rows or expired running rows with
`attempts < max_attempts`. One Claim statement can therefore lock and update up
to 100 exhausted rows plus at most one eligible candidate, holding at most 101
distinct row locks until transaction end. It can perform 100 cleanup writes
while returning no job; the public result remains limited to one eligible job.

The false "locks exactly one row" sentence occurred only in the canonical
runtime boundary. The PRD, architecture, implementation specification, and
performance document accurately distinguish one eligible result from cleanup
work. Historical review records were inspected but not rewritten.

The correction at `b54cd4f3c385cbe0df1158c2a866efe7fbc216d1` changes only
`docs/runtime-boundary.md`. Focused checks were:

```text
git diff --check
repository-wide search for the rejected lock-cardinality wording
direct inspection of claimSQL predicates, FOR UPDATE clauses, and LIMIT 100/1
```

No Go, SQL, migration, API, or runtime behavior changed, so the existing
race, coverage, PostgreSQL, external-consumer, repeat, fuzz, performance, and
graph gates were not rerun. Their exact source attribution below is unchanged.

## Claim and heartbeat repair

`transact` already returns a populated operation value with
`ErrCommitOutcomeUnknown`. Claim alone discarded its `claimResult` on every
transaction error; it now returns the produced Job with the non-nil error. Its
ID and exact lease token are reconciliation-only until the caller confirms the
durable row. Enqueue unwraps and returns its result fields, while Cancel,
Redrive, and lease mutations directly return the generic transaction value and
error; no analogous production loss was found.

After a handler returns, `runAttempt` now closes the heartbeat stop channel and
cancels the attempt context with a private teardown cause before joining. A
heartbeat error matching context cancellation is ignored only when that local
cause won. Parent cancellation is checked independently, and a heartbeat that
returns `context.Canceled`, `ErrLeaseLost`, or another error before local
teardown remains a genuine failure. The result channel remains buffered and
single-producer/single-consumer; stop is closed exactly once by `runAttempt`.

Expected-red logs:

| Regression | SHA-256 |
| --- | --- |
| `/tmp/gotth-jobs-red-claim-unknown-commit.log` | `15870db84c1f0dae5e24b69319f15660a0f2996a1e3408f61b1d85bff2013eb8` |
| `/tmp/gotth-jobs-red-heartbeat-teardown.log` | `6d454d946fb7c7964d6a4654434fac7fb4b7e126b4b177f23ad3c57925101c6d` |

The first regression failed with a zero Job after the body returned the exact
fixture ID/token. The second timed out after 250 ms and required parent
cancellation to release Heartbeat. Both pass after their respective production
changes. The final worker tests also distinguish local teardown, parent
cancellation, domain `ErrCanceled`, `ErrLeaseLost`, and an independently
returned `context.Canceled` heartbeat error.

## Expected-red cursor regressions

Before the production repair,
`TestListDeadCursorPostgreSQLRange` accepted the exact minimum minus one
microsecond, maximum plus one microsecond, and
`time.Unix(18_447_690_758_509, 551_616_000).UTC()`. The expected-red log is:

```text
/tmp/gotth-jobs-red-dead-cursor.log
SHA-256 1c37f8a5d4de1510ad29b7966fdfc0c209f79cb86637c745fa50462d2c999397
```

The final unit table accepts both exact endpoints and rejects both adjacent
out-of-range values plus the exact wrap-to-Y2K fixture. Its stub database
proves rejected cursors issue zero queries. The existing enqueue range table
exercises the same endpoint and wrap boundaries through the shared predicate.

`TestPostgreSQLListDeadRejectsWrappedCursor` creates a current dead row, then
passes the exact wrap fixture to `ListDead`. It requires a nil result and
`ErrInvalid`; the old behavior encoded the cursor as Y2K, ran the query, and
returned that modern row.

## Local focused checks

The agent host used Go 1.26.6-X:nodwarf5 on Linux amd64. Only lightweight
checks ran locally:

```text
GOMAXPROCS=2 go test -mod=readonly -p=1 ./pkg/jobs -run '^TestClaimPreservesFencingHandleOnUnknownCommit$' -count=1
GOMAXPROCS=2 go test -mod=readonly -p=1 ./pkg/jobs -run '^TestWorkerHandlerCompletionCancelsBlockedHeartbeat$' -count=1
GOMAXPROCS=2 go test -mod=readonly -p=1 ./pkg/jobs -run '^(TestClaimPreservesFencingHandleOnUnknownCommit|TestWorkerHandlerCompletionCancelsBlockedHeartbeat|TestWorkerHeartbeatCancellationCancelsCooperativeHandler|TestWorkerRunAndAttemptFailurePaths|TestWorkerContextAndConfigurationEdges)$' -count=1
GOMAXPROCS=2 go test -mod=readonly -p=1 ./pkg/jobs -count=1
GOMAXPROCS=2 go vet -mod=readonly ./pkg/jobs
```

The first two commands were captured failing before their corresponding
production change, then all listed commands passed. Formatting and
`git diff --check` also passed.

## Exact clean-source development gates

The repair source was transferred without a push in a Git bundle and cloned
detached on `development` at:

```text
/home/linus/.cache/openclaw-code-index/gotth-jobs/6655331ae4e3b7509b826a03db11c36cee9a6ca2/source
```

The bundle SHA-256 is
`0ec64bb7a80a12c44b7d6278f5f36d9d27f60fcc92c9f5ba911b5c1d700e8cc3`.
The clone was clean before and after every gate. The module selected Go 1.26.6.

The exact source passed:

```text
gofmt tracked-file check
go vet -mod=readonly ./...
go test -mod=readonly -count=1 ./...
go build -mod=readonly ./...
go test -mod=readonly -race -count=1 ./...
go test -mod=readonly -race -count=50 -run='^(TestClaimPreservesFencingHandleOnUnknownCommit|TestClaimReturnsOneFencedAttempt|TestWorkerHandlerCompletionCancelsBlockedHeartbeat|TestWorkerHeartbeatCancellationCancelsCooperativeHandler|TestWorkerRunAndAttemptFailurePaths|TestWorkerContextAndConfigurationEdges)$' ./pkg/jobs
go test -mod=readonly -count=1 -coverprofile=<artifact>/coverage.out ./...
```

Statement coverage is 96.3%. `claimWithToken` is 88.9% and `runAttempt` is
97.4%; every changed production statement has a nonzero count. Their uncovered
blocks are preexisting repository/context/request validation, public Claim
entropy failure, Worker.Run edges, and retry-delay-error propagation. The exact
zero-count ranges are in `affected-coverage-gaps.log`; neither repaired path has
a coverage gap.

## PostgreSQL 17 integration

Integration ran against disposable PostgreSQL 17.10 using exactly:

```text
postgres:17@sha256:a426e44bac0b759c95894d68e1a0ac03ecc20b619f498a91aae373bf06d8508d
```

Exact commands:

```text
go test -mod=readonly -race -tags=integration -count=1 ./...
go test -mod=readonly -tags=integration -count=1 -coverprofile=<artifact>/integration-coverage.out ./...
```

Both suites passed at source `6655331`; integration statement coverage is
96.5%. Existing transaction, commit classification, concurrency, fencing,
lifecycle, worker, exact timestamp endpoint, state/attempt, cancellation, and
dead-letter oracles all passed. Commit response loss remains a deterministic
unit fault because a real server cannot safely report whether the commit took
effect. The disposable container was removed and the exact source remained
clean.

## External consumer and proportional scope

A standalone module outside the repository previously passed readonly test and
build against cursor-repair source `9f6acc74`:

```text
go test -mod=readonly -count=1 ./...
go build -mod=readonly ./...
```

Public signatures and the module dependency surface did not change, so that
external-consumer gate was not rerun. The two fuzz targets, performance matrix,
and Graphify integrity gates remain historical evidence at ancestor
`72c62231fa4a0012ceef0a9c5ff61ff05feaf859`; they were not rerun or
represented as current-source results. The affected race repeat did run 50
times at current source as recorded above.

## Current artifact inventory

Artifact root:

```text
/home/linus/.cache/openclaw-code-index/gotth-jobs/6655331ae4e3b7509b826a03db11c36cee9a6ca2/artifacts
```

| Artifact | SHA-256 |
| --- | --- |
| `bundle-verify.log` | `5a8fb580cb61be3f37a71a895f7d2163bc4645cb870288e968f1ed8fb4f13d74` |
| `clean-gates.log` | `b4967c89e869a46cb865daf8861570dc9c9a4ced2940c08c3b954ab488771792` |
| `coverage.out` | `dc27fd7fdf7d57c568b52840ab59536640873b041229f7499d88bd8dc99fa90e` |
| `affected-coverage-gaps.log` | `21169a6443840b9102a2e80684b8ff22305843db97d54369849d23ed9e18ac50` |
| `integration.log` | `3ccc6d87075f389316a584c3ce06fe27a0151c68dc0dce0155f347adcbbad5f4` |
| `integration-coverage.out` | `c64d8ebcfb6d5bcaa20e8d85795a9a5982da5f2f00c807b058b15791f3177572` |
| `postgresql-image.json` | `557203fa8ddb39ed2b8ad084c0ec9a040498828e9f7cb1b344ba43b98844d374` |

## Remaining gate

Affected implementation and exact-source evidence gates are complete for the
Claim and heartbeat repair. Final admission remains blocked on two
attributable, fresh independent clean reviews pinned to the final candidate
tree. Those reviews are orchestrator-owned; this worker neither creates them
nor claims a result.
