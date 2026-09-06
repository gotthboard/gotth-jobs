# Reusable v0 admission evidence

## Identity and disposition

- Baseline: `874212b762571cd88322867872e458af0d9e0435`.
- Judge-7-rejected candidate:
  `f7afde19020efdb201591e71156a8e35816cd43f`.
- Independent report: `/tmp/gotth-jobs-independent-judge-7.md`.
- Exact implementation repair source:
  `9711e2b00dc95ae3070090745d611b65eca686f3`.
- Source tree: `cd122de97bfa6c1c227b906f801cb8cdeace3503`.
- Source bundle SHA-256:
  `4ea17eace89f0d93037beee2b86785facafa9d34b4a51b930f916d64d95ff4d8`.
- Branch: `feature/reusable-v0-admission` in the assigned isolated worktree.
- State: active. This repair worker does not claim independent final
  admission.
- No tag, Git remote configuration, push, merge, release, pull request,
  deployment, live database, secret, or external consumer was changed.

Judge 7 found five defects: cancellation could erase unknown Heartbeat commit
outcomes; only payload was length-bounded before scan ownership allocation;
the integration helper could drop tables from any PostgreSQL 17 database;
nullable key/token/owner presence was collapsed; and the append-only workflow
ledger omitted Judge 6. Historical reviews do not admit this source. Two fresh
attributable orchestrator-owned reviews remain required.

## Contract and source inspection

The worker read the complete Judge 7 report, PRD, architecture,
implementation specification, runtime boundary, workflow state/evidence,
Worker cancellation/channel lifecycle, every shared job scanner and caller,
every integration DDL entry point, and pinned pgx 5.10 codec/query source.

pgx `TextCodec` and binary `ByteaCodec` both dispatch `pgtype.BytesScanner`
with borrowed source bytes. Direct `*string` converts the complete source to a
string, and direct `*[]byte` copies the complete bytea before caller validation.
The final row scanner therefore uses `BytesScanner` for ID, queue, kind,
payload, idempotency key, state, lease token, lease owner, failure text, and
request fingerprint. It enforces exact schema minima/maxima before one bounded
ownership conversion. Fingerprints are exactly 32 bytes. Nullable text keeps
a presence bit so SQL NULL differs from present-empty, and every non-running
job requires a completely zero `Lease`.

After heartbeat join, `ErrCommitOutcomeUnknown` is classified before parent
context state and before the local teardown-cancellation exception. The result
is always a secret-safe `LeaseReconciliationError` with the known exact Job and
Lease, and Worker performs no subsequent Complete or Fail call.

Every destructive integration test enters through one reset helper. The helper
requires `GOTTH_JOBS_ALLOW_DESTRUCTIVE_TEST_DATABASE_RESET=true`, then queries
PostgreSQL for `current_database()` and the database comment. Only exact name
`gotth_jobs_test` plus exact marker
`gotth-jobs:dedicated-destructive-integration-test-v1` authorizes DDL. URL and
environment names are never inferred as authorization.

## Expected-red evidence

Before production changes, the focused local command used Go 1.26.6 with
`GOMAXPROCS=2` and `-p=1`. It failed for the intended reasons:

- parent cancellation returned only `context.Canceled` instead of
  `ErrCommitOutcomeUnknown` plus `LeaseReconciliationError`;
- local handler teardown suppressed the joined unknown outcome and called
  Complete;
- the scanner-destination assertion found direct variable-width destinations;
- present-empty key, token, and owner all succeeded;
- a non-running `Lease.JobID` succeeded;
- 31-byte and 33-byte fingerprints succeeded.

The integration-tag reset regression initially failed to compile with
`undefined: resetIntegrationSchema`, proving the executable guard did not
exist. No unsafe pre-fix database run was attempted.

One intermediate PostgreSQL race run used a threshold below pgx/race network
framing cost and failed at roughly 0.56-0.71 MiB/op for a rejected 1 MiB
source. That was a test-oracle failure, not claimed evidence. The final oracle
rejects a full source-sized destination allocation while separately comparing
all modes; non-race exact-source results were only 4-46 KiB/op. A container
startup attempt also stopped before marker DDL when `pg_isready` observed the
image's initialization restart; the successful runner waits for an actual SQL
query against the target database.

## Repairs and local checks

The two heartbeat tests deterministically synchronize an in-flight Heartbeat
with parent cancellation and local handler teardown. Both assert
`errors.Is(ErrCommitOutcomeUnknown)`, `errors.As(*LeaseReconciliationError)`,
exact reconciliation Job/Lease values, secret-free error text, and zero later
acknowledgements.

Row tests cover every scanner destination, exact maxima, adjacent minima and
maxima, ownership isolation, SQL NULL for every required variable-width
column, all three nullable present-empty cases, exact/adjacent fingerprint
lengths, oversized rejection without proportional ownership allocation, full
non-running Lease zero, and a bounded but semantically invalid state.

Reset tests reject missing/wrong opt-in, wrong database, missing marker, and
wrong marker with zero destructive executions; only the exact three-part
authorization reaches one reset execution.

Local post-repair checks on `agenthost` were limited to lightweight commands:

```text
GOMAXPROCS=2 go test -p=1 ./pkg/jobs -count=1
GOMAXPROCS=2 go test -p=1 -tags=integration ./pkg/jobs -run='^TestDestructiveIntegrationResetRequiresExplicitDedicatedIdentity$' -count=1
GOMAXPROCS=2 go test -p=1 -tags=integration ./pkg/jobs -run='^$' -count=1
GOMAXPROCS=2 go vet -mod=readonly ./pkg/jobs
```

All passed. Formatting and `git diff --check` also passed.

## Exact-source development gates

The complete Git bundle was cloned detached on `development` at:

```text
/home/linus/.cache/openclaw-code-index/gotth-jobs/9711e2b/source
```

Bundle hash, HEAD, tree, pre-status, and post-status were checked. Go 1.26.6
was selected. Exact clean source passed:

```text
tracked Go format check
go vet -mod=readonly ./...
go test -mod=readonly -count=1 ./...
go build -mod=readonly ./...
go test -mod=readonly -race -count=1 ./...
go test -mod=readonly -race -count=50 -run='<14 affected/adjacent tests>' ./pkg/jobs
go test -mod=readonly -count=1 -coverprofile=<artifact>/coverage.out ./...
go test -mod=readonly -run='^$' -fuzz=<each of two targets> -fuzztime=10s ./pkg/jobs
```

Unit statement coverage is 97.4%. Both scanner methods, `scanJob`,
`scanJobWithFingerprint`, `scanJobRow`, `validateStoredJob`, query option
forcing, and typed reconciliation methods are 100% covered. Both new
`runAttempt` precedence paths are covered. `runAttempt` remains 97.8%; its only
gap is the preexisting RetryPolicy.Delay error after handler failure,
unreachable through a validated `Worker.Run`. Other exact residual blocks are
preexisting entropy, nil/input, migration panic, and unrelated operation error
branches listed in `coverage-gaps.log`.

## PostgreSQL 17 and allocation

Disposable integration used PostgreSQL 17.10 at exactly:

```text
postgres:17@sha256:a426e44bac0b759c95894d68e1a0ac03ecc20b619f498a91aae373bf06d8508d
```

The runner created database `gotth_jobs_test`, set its PostgreSQL database
comment to the exact dedicated marker, exported the separate destructive
opt-in, and only then ran the tests. Exact source passed:

```text
go test -mod=readonly -race -tags=integration -count=1 ./...
go test -mod=readonly -tags=integration -count=1 -coverprofile=<artifact>/integration-coverage.out ./...
go test -mod=readonly -tags=integration -run='<payload and text/fingerprint mode tests>' -count=3 -v ./pkg/jobs
go test -mod=readonly -race -tags=integration -run='<reset rejection, isolation, two key interleavings>' -count=10 -v ./pkg/jobs
go test -mod=readonly -tags='integration performance' -run='^TestPostgreSQLPerformanceAdmission$' -count=1 -v ./pkg/jobs
```

Integration statement coverage is 97.4%. All five pgx connection defaults
passed because each job-returning query still forces described binary results.
Across three non-race runs, 1 MiB oversized text rejection measured about
4-19 KiB/op and oversized fingerprint rejection about 9-46 KiB/op. No mode
made a source-sized destination ownership allocation. Existing maximum payload
rows used about 1.47-1.61 MiB/op including pgx/network storage and the one
accepted library payload copy; oversized payload rejection used about
4-18 KiB/op.

Ten race-instrumented repeats passed every destructive-target rejection and
exact authorization case, Read Committed isolation checks, and both
authenticated-row/key-update retention interleavings. The disposable container
was removed.

## External consumer and performance

A standalone module outside the repository compiled against exact source and
exercised the public Store and secret-safe reconciliation contract. It passed:

```text
go test -mod=readonly -count=1 ./...
go build -mod=readonly ./...
```

The exact-source complete performance matrix passed. Results are recorded in
`docs/performance.md`; no optimization or latency guarantee is claimed.

## Artifact inventory

Artifact root:

```text
/home/linus/.cache/openclaw-code-index/gotth-jobs/9711e2b/artifacts
```

| Artifact | SHA-256 |
| --- | --- |
| `clean-gates.log` | `606ac8befede3b44df1c32b470285adef3ec7f15bc0dfc1978a3906ac0cd0198` |
| `coverage-functions.log` | `603112bc10e8d12f1c8b0084f7aeb472659321e816cc852bebdd337d20272e14` |
| `coverage-gaps.log` | `1d05b8d48c47b147e41802a16a8dcb0c6fb3685d2b12bf3d99296be2bc8d2910` |
| `coverage.out` | `f30b2d243f4f8402ed5cc562d4e11b980b01d7e9e5ffa76499de33aae612ae20` |
| `external-consumer.log` | `b59ef76c038a6af6ac881547d78b26167f4c2994029a43e8959a366934023170` |
| `integration-coverage.out` | `f30b2d243f4f8402ed5cc562d4e11b980b01d7e9e5ffa76499de33aae612ae20` |
| `integration.log` | `373876cec52209dc45904c70edc374ed5354a081ae1dff5149d9c2a0b7d4820c` |
| `performance.log` | `24ef6f7975363a52cb61d738ac571a0e58db9870c39a40c7e7c5cd0e7220b991` |
| `postgresql-image.json` | `557203fa8ddb39ed2b8ad084c0ec9a040498828e9f7cb1b344ba43b98844d374` |
| `postgresql-interleavings.log` | `161143e99b8a30f7773152b5cc5ed65cdf7a2e40d1347f2082dbbb9df0d8e6a2` |
| `postgresql-query-modes.log` | `3e6ac5bf3bda4a059d996442fa0c35a78bd3f54d96d4ec8a22925a86e37b9e16` |

## Remaining gate

Implementation and exact-source evidence gates are complete for this repair.
Final admission remains active pending two fresh attributable independent
reviews pinned to the final candidate tree. Those reviews are
orchestrator-owned; this worker neither creates them nor claims a result.
