# Reusable v0 admission evidence

## Identity and disposition

- Baseline: `874212b762571cd88322867872e458af0d9e0435`.
- Audit-rejected candidate: `c64368a202f4af62c33a8640ac0d0923df2e333e`.
- Admission audit: `/tmp/gotth-jobs-admission-audit.md`.
- First repair source: `72c62231fa4a0012ceef0a9c5ff61ff05feaf859`.
- First-review-rejected candidate:
  `671a1eac9ddc6d273136d46de6d906730c7182e5`.
- First independent review: `/tmp/gotth-jobs-independent-judge-1.md`.
- Current cursor repair source:
  `9f6acc74f8901a58a3a9929d10ad7a3779241f4d`.
- Branch: `feature/reusable-v0-admission` in the assigned isolated worktree.
- State: active. This repair worker does not claim independent final admission.
- No tag, Git remote configuration, push, merge, release, pull request,
  deployment, live database, secret, or consumer was changed.

The admission audit found unbounded enqueue timestamp encoding and impossible
state/attempt combinations. The first independent review then found the same
PostgreSQL encoding boundary missing from non-nil `DeadCursor.FinishedAt`.
Those implementation defects are repaired. The historical reviews do not
admit the current tree; two fresh attributable orchestrator-owned reviews of
the final candidate remain required.

## Contracts checked

The worker read the full admission audit, first independent review, PRD,
architecture, implementation specification, runtime boundary, workflow plan,
manifest, records, and prior evidence before changing production code.

PostgreSQL 17 accepts finite `timestamptz` values from
`-4713-11-24T00:00:00Z` through `294276-12-31T23:59:59.999999Z` in Go's
proleptic Gregorian calendar. pgx 5.10.0 performs binary timestamp arithmetic
in `int64` and does not reject every out-of-range finite `time.Time`; the exact
reported fixture can wrap to Y2K. One internal predicate now enforces nonzero
UTC, microsecond precision, and the inclusive PostgreSQL endpoints for both
explicit enqueue availability and non-nil dead-letter cursors. The public API
and successful-query semantics are unchanged.

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
GOMAXPROCS=2 go test -mod=readonly -p=1 ./pkg/jobs -run '^TestListDeadCursorPostgreSQLRange$' -count=1
GOMAXPROCS=2 go test -mod=readonly -p=1 ./pkg/jobs -run '^(TestListDeadCursorPostgreSQLRange|TestEnqueueAvailabilityPostgreSQLRange)$' -count=1
GOMAXPROCS=2 go test -mod=readonly -p=1 ./pkg/jobs -count=1
GOMAXPROCS=2 go vet -mod=readonly ./pkg/jobs
```

All post-repair checks passed. The focused coverage run reports
`isPostgreSQLTimestamp` at 100%; the narrower selection leaves unrelated paths
in `validateEnqueue` and `ListDead` uncovered, so exact-source suite coverage
below is the relevant function-level result.

## Exact clean-source development gates

The repair source was transferred without a push in a Git bundle and cloned
detached on `development` at:

```text
/home/linus/.cache/openclaw-code-index/gotth-jobs/9f6acc74f8901a58a3a9929d10ad7a3779241f4d/source
```

The bundle SHA-256 is
`983bb32f4762e3a94458c9d6ad562488e0107734793e53ddf0fc71d5bb92abc7`.
The clone was clean before and after every gate. The module selected Go 1.26.6.

The exact source passed:

```text
gofmt tracked-file check
go vet -mod=readonly ./...
go test -mod=readonly -count=1 ./...
go build -mod=readonly ./...
go test -mod=readonly -race -count=1 ./...
go test -mod=readonly -count=1 -coverprofile=<artifact>/coverage.out ./...
```

Statement coverage is 96.3%. `isPostgreSQLTimestamp` and `validateEnqueue` are
100% covered; `ListDead` is 90% covered. All branches introduced or shared by
this cursor repair have direct tests, with no known relevant coverage gap.

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

Both suites passed; integration statement coverage is 96.5%. The new real-pgx
regression proves the invalid cursor returns `ErrInvalid` without wrapped query
behavior. Existing exact enqueue endpoint round trips, all 16 state/attempt
boundary combinations, transaction, concurrency, fencing, lifecycle,
cancellation, dead-letter, and worker oracles also passed. The container was
removed and the exact source remained clean.

## External consumer and proportional scope

A standalone module outside the repository used a local `replace` to the exact
clean source. It passed:

```text
go test -mod=readonly -count=1 ./...
go build -mod=readonly ./...
```

The earlier 50-repeat race, two fuzz targets, performance matrix, and Graphify
integrity gates were not rerun because this repair adds only synchronous input
validation before the existing query and does not alter successful SQL,
concurrency, retry, allocation, or package dependency behavior. Their results
at ancestor `72c62231fa4a0012ceef0a9c5ff61ff05feaf859` remain historical
evidence only and are not represented as exact current-source gates.

## Current artifact inventory

Artifact root:

```text
/home/linus/.cache/openclaw-code-index/gotth-jobs/9f6acc74f8901a58a3a9929d10ad7a3779241f4d/artifacts
```

| Artifact | SHA-256 |
| --- | --- |
| `bundle-verify.log` | `d44b66d946ddeb6f2577b2a957fcea8fda4928112293f9dacb1a2d9da5b82d1b` |
| `clean-gates.log` | `6dd42e1c47071baefa6c40f6a0b1c33aab5289895b966254b6fb404bb63117ba` |
| `coverage.out` | `be816d10c02d2b2f9993ed254fb06a5bf8797af319f89370bc6f72683afd201a` |
| `integration.log` | `4957dad650b8ca4f88f216ca621619e7186ae291d2c06bc6cba2235c1626b59a` |
| `integration-coverage.out` | `f486feda0c27b4093fbb8b24e2ddf8f9cb703eb787cd65cf2f7a419cc138a4ab` |
| `postgresql-image.json` | `ea1f9a4b971fc46a7ddf85058899556ac0d523ed0bd9bf962bde7d0b12c5fa8b` |
| `external-consumer.log` | `ad897c89e3fb796acd06ce727b090988870e19949c2360672314a0b85a2936c9` |

## Remaining gate

Implementation and proportional exact-source evidence gates are complete for
the cursor repair. Final admission remains blocked on two attributable, fresh
independent clean reviews pinned to the final candidate tree. Those reviews
are orchestrator-owned; this worker neither creates them nor claims a result.
