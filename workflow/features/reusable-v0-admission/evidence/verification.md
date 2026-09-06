# Reusable v0 admission evidence

## Identity and disposition

- Baseline: `874212b762571cd88322867872e458af0d9e0435`.
- Rejected candidate: `c64368a202f4af62c33a8640ac0d0923df2e333e`.
- Admission audit: `/tmp/gotth-jobs-admission-audit.md`.
- Repair source: `72c62231fa4a0012ceef0a9c5ff61ff05feaf859`.
- Branch: `feature/reusable-v0-admission` in the assigned isolated worktree.
- State: active. The repair worker does not claim independent final admission.
- No tag, Git remote configuration, push, merge, release, deployment, live
  database, secret, or consumer was changed.

The audit rejected unbounded pgx timestamp encoding, incomplete state/attempt
validation, and stale unattributed reviews. The first two defects are repaired
and verified below. The prior review files remain historical; two fresh,
attributable, orchestrator-owned reviews of the final candidate remain required.

## Contracts checked

Before repair, the worker read the full audit, PRD, architecture,
implementation specification, runtime boundary, feature plan, workflow
manifest, workflow records, prior evidence, and prior reviews.

PostgreSQL 17 documents finite `timestamptz` support from 4713 BC through
294276 AD at one-microsecond resolution. PostgreSQL 17 source defines
`MIN_TIMESTAMP` as `-211813488000000000` microseconds from Y2K and
`END_TIMESTAMP` as the exclusive `9223371331200000000`; in Go's proleptic
Gregorian calendar these are `-4713-11-24T00:00:00Z` inclusive and
`294277-01-01T00:00:00Z` exclusive. pgx 5.10.0's binary codec performs its
microsecond arithmetic in `int64` without rejecting out-of-range finite
values, so public validation must enforce this range before encoding.

## Expected-red regressions

Each production unit was changed only after its focused regression failed:

1. `TestEnqueueAvailabilityPostgreSQLRange` first failed with the manual's
   year-level lower fixture, one microsecond above the maximum, and the audit's
   exact pgx fixture that wrapped to Y2K. After PostgreSQL source refined the
   lower endpoint, the fixture was corrected before production. A controlled
   replay with only the pre-fix range condition restored proves the final test
   fails at exact minimum minus one microsecond, maximum plus one microsecond,
   and the wrap fixture. Logs: `/tmp/gotth-jobs-red-time.log` and
   `/tmp/gotth-jobs-red-time-exact-replay.log`.
2. `TestStoredJobValidationStateAttemptBoundaries` failed only for pending at
   `max_attempts`, running at zero, succeeded at zero, and dead at zero; all
   other state boundaries were accepted. Log:
   `/tmp/gotth-jobs-red-row.log`.
3. `TestMigrationConstrainsStateAttemptCombinations` failed for the absent
   schema constraint and each required relation. Log:
   `/tmp/gotth-jobs-red-migration.log`.

The repair then made each focused test green before moving to the next
production unit. Existing cost comments now state that timestamp comparisons
and state/attempt relation checks are constant-time.

## Local focused checks

The agent host used Go 1.26.6-X:nodwarf5, Linux amd64. Repair work was limited
to lightweight checks with `GOMAXPROCS=2` and `-p=1`:

```text
GOMAXPROCS=2 go test -mod=readonly -p=1 ./pkg/jobs -run '^TestEnqueueAvailabilityPostgreSQLRange$' -count=1
GOMAXPROCS=2 go test -mod=readonly -p=1 ./pkg/jobs -run '^TestStoredJobValidation(RejectsEveryImpossibleShape|StateAttemptBoundaries)$' -count=1
GOMAXPROCS=2 go test -mod=readonly -p=1 ./pkg/jobs -run '^(TestMigrationConstrainsStateAttemptCombinations|TestEnqueueAvailabilityPostgreSQLRange|TestStoredJobValidationStateAttemptBoundaries|TestStoredJobValidationRejectsEveryImpossibleShape)$' -count=1
GOMAXPROCS=2 go test -mod=readonly -p=1 ./pkg/jobs -count=1
GOMAXPROCS=2 go vet -mod=readonly ./pkg/jobs
```

All post-repair local checks passed. `git diff --check` and the tracked Go
format check passed before the repair commit.

## Exact clean-revision gates

The repair source was transferred without a push in a Git bundle and cloned
detached on `development` at:

```text
~/.cache/openclaw-code-index/gotth-jobs/72c62231fa4a0012ceef0a9c5ff61ff05feaf859/source
```

The bundle SHA-256 is
`359f51284b4c989fab93c450d8f10016b129598c435379a66b5181cd07549931`.
The clone was clean before and after every gate. The host launcher is Go
1.26.5-X:nodwarf5; the module selected Go 1.26.6 for execution.

The exact clone passed:

```text
gofmt tracked-file check
go vet -mod=readonly ./...
go test -mod=readonly -count=1 ./...
go build -mod=readonly ./...
go test -mod=readonly -race -count=1 ./...
go test -mod=readonly -race -count=50 ./...
go test -mod=readonly -count=1 -coverprofile=<artifact>/coverage.out ./...
go test -mod=readonly -run='^$' -fuzz='^FuzzEnvelopeValidationNeverPanics$' -fuzztime=5s -v ./pkg/jobs
go test -mod=readonly -run='^$' -fuzz='^FuzzBoundedFailureIsValid$' -fuzztime=5s -v ./pkg/jobs
```

Clean-clone statement coverage is 96.3%. `validateEnqueue` is 100% covered.
`validateStoredJob` is 95.5% covered; its uncovered statement is the preexisting
terminal-timestamp rejection, which one older malformed fixture now reaches the
new state/attempt rejection before. Every newly added timestamp and
state/attempt branch is covered, so the confirmed defects have no known
coverage gap.

Fuzz results:

- envelope validation: 66,722 executions;
- bounded failure text: 342,657 executions;
- total: 409,379 executions without a product failure.

## PostgreSQL 17 integration

Integration ran against disposable PostgreSQL 17.10 using exactly:

```text
postgres:17@sha256:a426e44bac0b759c95894d68e1a0ac03ecc20b619f498a91aae373bf06d8508d
```

The race integration suite and non-race integration coverage suite passed.
Coverage with integration is 96.5%. New integration cases round-trip the exact
minimum, minimum plus one microsecond, maximum minus one microsecond, and exact
maximum availability. They reject the adjacent out-of-range values and the pgx
wrap fixture. The schema test inserts all 16 relevant state/attempt boundary
combinations: pending `0,1,max-1,max`; running, succeeded, and dead
`0,1,max`; canceled `0,1,max`. Only the four impossible pairs fail.

Exact commands:

```text
go test -mod=readonly -race -tags=integration -count=1 ./...
go test -mod=readonly -tags=integration -count=1 -coverprofile=<artifact>/integration-coverage.out ./...
go test -mod=readonly -tags='integration performance' -run='^TestPostgreSQLPerformanceAdmission$' -count=1 -v ./pkg/jobs
```

Existing PostgreSQL version/UTF8, transaction rollback, idempotency,
concurrency, claim, fencing, lease expiry, heartbeat, retry, exhaustion,
cancellation, dead listing, redrive, counts, future scheduling, and worker
oracles also passed. The disposable container was removed, and the exact clone
remained clean.

## Performance and consumer gates

The uninstrumented performance admission passed at the repair source. Exact
percentiles and limitations are in `docs/performance.md`; no speedup is
claimed. The race-instrumented integration run is not used as a timing source.

A standalone module outside the repository used a local `replace` only to the
clean clone. It compiled every public operation and value family, traversed all
public sentinels, read `Migrations`, asserted the `Store` implementation,
and passed `go test -mod=readonly -count=1 ./...` and
`go build -mod=readonly ./...`.

Graphify 0.9.32 code-only extraction reported 219 nodes, 506 valid edges, and
15 communities. Diagnostics found zero missing or dangling endpoints,
self-loops, exact duplicate edges, or directed/undirected same-endpoint
collision groups. The optional SQL parser remains unavailable; PostgreSQL
executed the migration and all material SQL paths directly.

## Artifact inventory

Artifact root:

```text
~/.cache/openclaw-code-index/gotth-jobs/72c62231fa4a0012ceef0a9c5ff61ff05feaf859/artifacts/
```

| Artifact | SHA-256 |
| --- | --- |
| `full-gates.log` | `23af52945d2d8a9a2e4ce5573e1b27b4d8823430c267ccacd330cfa7550436b2` |
| `coverage.out` | `9b77d4f6cf9f85dd0a53d36a53770fd1e2ffdf0c678a1da7aefe1f30bbc8b552` |
| `integration.log` | `03de2bd147008eddc3e64f7b9ee2a1a936768949e66cb47ae619a67911c7dd49` |
| `integration-coverage.out` | `6c12a64c4bf137d5e97f0f723e7a13d3e7e617973b519a73b6cd822adf0a3d7e` |
| `performance.log` | `32a96cbbf3c4402e89e38d39ead9e286238f275c5afc83168bdf45f29e13c19f` |
| `external-consumer.log` | `412f76a48d9b0f050424bfdfd1c590d908f420d7f4caaab888f254cad7246a7c` |
| `graph.json` | `cdc39341c68f477cce0b18fc5e18fd6fc450b1bde416304dddac2e5f54b3aab0` |
| `graph-diagnose.json` | `a0663bb195a2d123094df6d571ea869281133e8d41e0d7406c65d4a41c7a5d82` |
| `graphify.log` | `0a23b47f991c93da3eafd006ad0f6ede250f519f681e1a93d52df9c648efe5fa` |
| `postgresql-image.json` | `ea1f9a4b971fc46a7ddf85058899556ac0d523ed0bd9bf962bde7d0b12c5fa8b` |

## Remaining gate

Implementation and evidence gates are complete for the repair source. Final
admission is still blocked on two attributable, fresh independent clean
reviews pinned to the final candidate tree. Those reviews are explicitly
orchestrator-owned; this worker neither creates them nor claims their result.
