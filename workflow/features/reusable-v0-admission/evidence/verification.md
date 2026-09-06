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
- Claim/heartbeat repair source:
  `6655331ae4e3b7509b826a03db11c36cee9a6ca2`.
- Fourth-review-rejected candidate:
  `258aeba1bc43ec39cb21ec93bf6698948854e6dc`.
- Fourth independent review: `/tmp/gotth-jobs-independent-judge-4.md`.
- Current implementation repair source:
  `62d565aa1d3f4ebf19cc4d39bf87d2764c676c8b`.
- Branch: `feature/reusable-v0-admission` in the assigned isolated worktree.
- State: active. This repair worker does not claim independent final admission.
- No tag, Git remote configuration, push, merge, release, pull request,
  deployment, live database, secret, or consumer was changed.

The fourth review found three remaining defects: two Read Committed snapshots
could authenticate old idempotency row A and return replacement B; enqueue
copied an arbitrarily oversized payload and opened a transaction before
rejecting it; and `Worker.Run` erased a nonzero commit-unknown Claim result.
All three are repaired. Historical reviews do not admit the current tree; two
fresh attributable orchestrator-owned reviews of the final candidate remain
required.

## Contracts checked

The worker read the full admission audit, all four independent review reports,
PRD, architecture, implementation specification, runtime boundary, workflow
manifest, records, prior evidence, transaction wrappers, enqueue scanners,
Worker error flow, PostgreSQL 17 Read Committed and row-lock contracts, and pgx
transaction behavior before repair.

At Read Committed, separate commands receive separate snapshots. The fixed
idempotency fallback therefore selects `request_fingerprint` and every public
job column from one row in one command. `FOR KEY SHARE` retains that row's key
identity until transaction end: delete and key-changing updates wait, while
ordinary state-only updates remain compatible. The returned Job is exactly the
one scanned beside the compared fingerprint.

Every transaction wrapper was inspected for analogous value loss. Enqueue
returns its transaction result fields; Cancel, Redrive, and lease mutations
return `transact`'s value/error pair directly; Claim already preserves its
produced value. Only Worker had an additional value-erasing layer.

## Repairs and regressions

`prepareEnqueue` validates caller-controlled sizes before cloning. `Enqueue`
checks context, prepares one bounded copy, and only then calls `BeginTx` and a
private prepared helper. `EnqueueTx` prepares once and calls the same helper.
For a `MaxPayloadBytes+1` fixture, the regression requires `ErrInvalid`, zero
library-owned `BeginTx` calls, zero caller-transaction queries, and fewer than
half the input bytes allocated during each measured call. It therefore detects
the old proportional clone without requiring an unbounded fixture.

The deterministic unit interleaving models the old first fingerprint read of
A, replacement, and second job read of B. It failed by returning B. The fixed
path makes only one fallback statement, returns A from the same scan as its
fingerprint, and asserts `FOR KEY SHARE`. A malformed combined row is also
rejected through the shared untrusted-row validation boundary.

`ClaimReconciliationError` is exported but keeps its Job private. Its
`ReconciliationJob` accessor exposes the exact unconfirmed ID/token only for
durable reconciliation, `Unwrap` returns the original Claim error, and
`Error()` is fixed text containing no job fields. `Worker.Run` returns this
type for a nonzero commit-unknown Claim and does not invoke the handler.

Expected-red transcripts:

| Regression transcript | SHA-256 |
| --- | --- |
| `/tmp/gotth-jobs-red-judge4-enqueue.log` | `4e1eeac31200646d0d2b0cd05a9795206d561ec2d441d3e850c16b40b8408a14` |
| `/tmp/gotth-jobs-red-judge4-worker.log` | `e449b0ecd1e82bc1c44927f49e61211623fd88fc2e616384161158976da12647` |

## Local focused checks

The agent host used Go 1.26.6-X:nodwarf5 on Linux amd64. Only constrained,
lightweight checks ran locally:

```text
GOMAXPROCS=2 go test -p=1 ./pkg/jobs -run 'TestEnqueue(IdempotentFallbackCannotAuthenticateThenReturnReplacement|RejectsOversizedPayloadBeforeCopyOrBegin)$' -count=1
GOMAXPROCS=2 go test -p=1 ./pkg/jobs -run '^TestWorkerRunPreservesUnknownClaimOutcomeForReconciliation$' -count=1
GOMAXPROCS=2 go test -mod=readonly -p=1 ./pkg/jobs -run '^(TestEnqueueIdempotentFallbackCannotAuthenticateThenReturnReplacement|TestEnqueueRejectsOversizedPayloadBeforeCopyOrBegin|TestScanJobWithFingerprintRejectsMalformedCombinedRow|TestWorkerRunPreservesUnknownClaimOutcomeForReconciliation)$' -count=10
GOMAXPROCS=2 go test -mod=readonly -p=1 ./pkg/jobs -count=1
GOMAXPROCS=2 go vet -mod=readonly ./pkg/jobs/...
GOMAXPROCS=2 go test -mod=readonly -p=1 -tags=integration ./pkg/jobs -run '^$' -count=1
```

The first two commands were captured failing before their corresponding
production repairs. Post-repair focused, repeated, package, vet, compile,
format, and `git diff --check` checks passed.

## Exact clean-source development gates

The repair source was transferred without a push in a Git bundle and cloned
detached on `development` at:

```text
/home/linus/.cache/openclaw-code-index/gotth-jobs/62d565aa1d3f4ebf19cc4d39bf87d2764c676c8b/source
```

The bundle SHA-256 is
`f839d6abde5b773787203597d22e88a16e2ba5111c5eecaab1339717d04476e7`.
The clone was clean before and after every gate and selected Go 1.26.6.

Exact source passed:

```text
gofmt tracked-file check
go vet -mod=readonly ./...
go test -mod=readonly -count=1 ./...
go build -mod=readonly ./...
go test -mod=readonly -race -count=1 ./...
go test -mod=readonly -race -count=50 -run=<eight affected/adjacent tests> ./pkg/jobs
go test -mod=readonly -count=1 -coverprofile=<artifact>/coverage.out ./...
```

Statement coverage is 96.9%. `Enqueue`, `EnqueueTx`, `prepareEnqueue`,
`scanJob`, `scanJobWithFingerprint`, `scanJobRow`, and all three
`ClaimReconciliationError` methods are 100% covered. The new Worker branch has
nonzero counts. Exact zero-count ranges are in
`affected-coverage-gaps.log`: enqueue entropy failure, preexisting terminal
timestamp rejection, Worker nil/configuration/attempt error propagation, and
retry-delay failure. No defect-specific statement remains uncovered.

## PostgreSQL 17 and performance

Integration used disposable PostgreSQL 17.10 at exactly:

```text
postgres:17@sha256:a426e44bac0b759c95894d68e1a0ac03ecc20b619f498a91aae373bf06d8508d
```

Exact commands:

```text
go test -mod=readonly -race -tags=integration -count=1 ./...
go test -mod=readonly -tags=integration -count=1 -coverprofile=<artifact>/integration-coverage.out ./...
go test -mod=readonly -race -tags=integration -run '^TestPostgreSQLEnqueueIdempotentFallbackRetainsAuthenticatedRow$' -count=10 -v ./pkg/jobs
go test -mod=readonly -tags='integration performance' -run '^TestPostgreSQLPerformanceAdmission$' -count=1 -v ./pkg/jobs
```

All passed. The retaining-row test observed the replacement `DELETE` waiting
in `pg_stat_activity` with `wait_event_type='Lock'` until the caller-owned
transaction committed, then inserted B and required request A to return
`ErrIdempotencyConflict`. All 10 race-instrumented repeats passed. Integration
coverage is 96.9%. The complete performance matrix passed; exact results are in
`docs/performance.md` and no optimization claim is made. The disposable
container was removed.

## External consumer and proportional scope

A standalone module outside the repository compiled and ran against exact
repair source. It used `errors.As` for `*jobs.ClaimReconciliationError`, checked
`errors.Is` against the original joined error, recovered the exact ID/token,
verified token-free error text, and proved the handler was not called:

```text
go test -mod=readonly -count=1 ./...
go build -mod=readonly ./...
```

The two fuzz targets and Graphify integrity gate were not invalidated by the
SQL snapshot, allocation ordering, or additive error type and remain ancestor
evidence at `72c62231fa4a0012ceef0a9c5ff61ff05feaf859`. They were not
represented as current-source results.

## Current artifact inventory

Artifact root:

```text
/home/linus/.cache/openclaw-code-index/gotth-jobs/62d565aa1d3f4ebf19cc4d39bf87d2764c676c8b/artifacts
```

| Artifact | SHA-256 |
| --- | --- |
| `bundle-verify.log` | `305a146bcfb5c47f05cf36bd97442b3c7bc63e86d2f68c8999a53cd3d2f5b764` |
| `clean-gates.log` | `2d185c12e401131e0aca63977c46a7a25a4550cb69e8f3bf9a2fbf6be5d0e6a0` |
| `coverage.out` | `84f21c827b4f0e8757b41d19b8112a2e78c0a3e58baadf47135d4373faa162ab` |
| `affected-coverage-gaps.log` | `124d5bafba72c525110fb57758747b36d19ad30411909a1008cb314c42a3a8cf` |
| `integration.log` | `d221930d6e714d454cbac2cfdb01d10dd15e3f2eeffd3acdd38517b7c075f0d1` |
| `integration-coverage.out` | `84f21c827b4f0e8757b41d19b8112a2e78c0a3e58baadf47135d4373faa162ab` |
| `postgresql-image.json` | `557203fa8ddb39ed2b8ad084c0ec9a040498828e9f7cb1b344ba43b98844d374` |
| `postgresql-repeat.log` | `ffb827171b351e904aacd4dfc213e0fba608c08f5bfa51a227f6d7c4162969fa` |
| `performance.log` | `dbe24670baff242efc2440e2f187629d16b0aa6929799a0a71cdac33f5aa0ff9` |
| `external-consumer.log` | `da8228ef3155f6ed5d893b27fe6a2dd30ae9bdd2e29f6e9d7fb1603edd558c51` |

## Remaining gate

Affected implementation and exact-source evidence gates are complete for this
repair. Final admission remains blocked on two attributable, fresh independent
clean reviews pinned to the final candidate tree. Those reviews are
orchestrator-owned; this worker neither creates them nor claims a result.
