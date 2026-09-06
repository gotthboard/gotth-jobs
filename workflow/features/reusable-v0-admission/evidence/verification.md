# Reusable v0 admission evidence

## Identity and disposition

- Baseline: `874212b762571cd88322867872e458af0d9e0435`.
- Judge-5-rejected candidate:
  `6984227d9e1bc9247a6bc783627dfeabd5c2dc42`.
- Independent report: `/tmp/gotth-jobs-independent-judge-5.md`.
- Exact implementation repair source:
  `53cf140090cb7c1bc2076579437aab8edd3a0229`.
- Branch: `feature/reusable-v0-admission` in the assigned isolated worktree.
- State: active. This repair worker does not claim independent final
  admission.
- No tag, Git remote configuration, push, merge, release, pull request,
  deployment, live database, secret, or consumer was changed.

Judge 5 found three defects: Heartbeat returned and Worker discarded a full
Job payload on every tick; the partial queue/key unique index did not make key
changes conflict with `FOR KEY SHARE`; and `scanJobRow` copied bytea before
checking its size, then copied valid payloads again. Historical clean reviews
do not admit this source. Two fresh attributable orchestrator-owned reviews
remain required.

## Contracts and adjacent audit

The worker read the complete Judge 5 report, PRD, architecture, implementation
specification, runtime boundary, workflow state and evidence, PostgreSQL
integration/performance tests, and every job scan and Heartbeat caller.

Pinned pgx 5.10 source was inspected directly. `pgtype.BytesScanner` receives
driver memory valid only until the next database method call. `ByteaCodec`
prefers binary format; its binary scanner plan calls `ScanBytes(src)` directly,
while the old `*[]byte` plan allocates and copies. The new scanner therefore
checks `len(src)` first and makes one owning copy only for an accepted payload.
All public Job-returning SQL paths share `scanJobRow`; no analogous payload
post-clone remains. The only separate byte slice scan is the fixed 32-byte
idempotency fingerprint.

PostgreSQL's non-partial unique `(queue, idempotency_key)` index makes queue/key
updates key-changing row updates that conflict with `FOR KEY SHARE`. Default
unique-index `NULL` semantics remain distinct, so unkeyed jobs are unaffected.
Heartbeat is the only lifecycle value discarded by Worker; Complete and Fail
return their transitioned Job to public callers and retain their payload cost.

## Repairs and regressions

Heartbeat's unreleased Store and PostgreSQL signatures now return `error`.
The SQL returns one boolean and no `jobColumns`; rejected leases still use the
same same-transaction state classifier. Worker and all fakes/public consumers
use the error-only contract. Unit SQL-shape coverage rejects any heartbeat
payload column. Real PostgreSQL benchmarking compares empty and 1 MiB jobs and
requires the latter to allocate no more than 64 KiB/op above the former; three
repeats passed.

The migration now defines a non-partial unique queue/key index and Enqueue's
conflict target matches it. Migration text coverage rejects a partial
predicate. PostgreSQL coverage enqueues two distinct rows with `NULL` keys.
The deterministic key-update test obtains the idempotent fallback's key-share
lock, starts a concurrent key-changing UPDATE, observes it waiting on a row
lock, commits the fallback transaction, then completes the update and inserts
replacement B. A subsequent request A must conflict with B.

`boundedPayloadScanner` implements `pgtype.BytesScanner`. It rejects
`MaxPayloadBytes+1` before payload-sized allocation, accepts exactly
`MaxPayloadBytes`, owns the returned bytes independently of source mutation,
and removes the post-scan clone. The valid regression measured about 2.10 MiB
before repair and requires one payload-sized allocation after repair; the
oversized regression measured about 2.11 MiB before repair and now requires
less than half a payload of incidental allocation.

Expected-red transcript:

| Artifact | SHA-256 |
| --- | --- |
| `/tmp/gotth-jobs-red-judge5.log` | `2025d182ac8baaf169237ea35c838e195f96bea3ff6e68717b0d113cae7c6ec1` |

The focused red command failed all four defect assertions before production
changes. The PostgreSQL interleaving was added in the same tests-first phase
and executed only on the designated development host after the migration fix;
no local PostgreSQL gate was claimed.

## Local focused checks

The agent host used Go 1.26.6-X:nodwarf5 on Linux amd64. Only constrained,
lightweight checks ran locally:

```text
GOMAXPROCS=2 go test -mod=readonly -p=1 ./pkg/jobs -run '<four defect regressions>' -count=1  # expected red
GOMAXPROCS=2 go test -mod=readonly -p=1 ./pkg/jobs -run '<four defect regressions>' -count=1
GOMAXPROCS=2 go test -mod=readonly -p=1 ./pkg/jobs -run '^(TestHeartbeat|TestScanJob|TestMigrationUsesNonPartialIdempotencyKeyIndex)' -count=10
GOMAXPROCS=2 go test -mod=readonly -p=1 ./pkg/jobs -count=1
GOMAXPROCS=2 go test -mod=readonly -p=1 -tags=integration ./pkg/jobs -run '^$' -count=1
GOMAXPROCS=2 go vet -mod=readonly ./pkg/jobs/...
```

All post-repair checks passed. Formatting and `git diff --check` passed.

## Exact clean-source development gates

The source was transferred without a push in a complete Git bundle and cloned
detached on `development` at:

```text
/home/linus/.cache/openclaw-code-index/gotth-jobs/53cf140090cb7c1bc2076579437aab8edd3a0229/source
```

Bundle SHA-256:
`f8005454b5a687bf5a43d3887cf41b0f89c2c1300d72cdd8237d585354d722c3`.
The clone was clean before and after every gate and selected Go 1.26.6.

Exact source passed:

```text
tracked Go format check
go vet -mod=readonly ./...
go test -mod=readonly -count=1 ./...
go build -mod=readonly ./...
go test -mod=readonly -race -count=1 ./...
go test -mod=readonly -race -count=50 -run=<seven affected/adjacent tests> ./pkg/jobs
go test -mod=readonly -count=1 -coverprofile=<artifact>/coverage.out ./...
go test -mod=readonly -fuzz=<each target> -fuzztime=10s ./pkg/jobs
```

Statement coverage is 97.0%. Heartbeat, Complete, Fail, `leaseMutation`,
`boundedPayloadScanner.ScanBytes`, `scanJob`, `scanJobWithFingerprint`, and
`scanJobRow` are all 100% covered. The exact lower-coverage functions are in
`affected-coverage-gaps.log`; they are preexisting entropy/error, migration
panic, operation, stored terminal timestamp, Claim, and Worker branches. No
changed function or defect-specific statement remains uncovered.

## PostgreSQL 17, external consumer, and performance

Integration used disposable PostgreSQL 17.10 at exactly:

```text
postgres:17@sha256:a426e44bac0b759c95894d68e1a0ac03ecc20b619f498a91aae373bf06d8508d
```

Exact commands:

```text
go test -mod=readonly -race -tags=integration -count=1 ./...
go test -mod=readonly -tags=integration -count=1 -coverprofile=<artifact>/integration-coverage.out ./...
go test -mod=readonly -race -tags=integration -run '^TestPostgreSQLEnqueueIdempotentFallbackRetainsKeyUpdate$' -count=10 -v ./pkg/jobs
go test -mod=readonly -tags=integration -run '^TestPostgreSQLHeartbeatAllocationIndependentOfPayload$' -count=3 -v ./pkg/jobs
go test -mod=readonly -tags='integration performance' -run '^TestPostgreSQLPerformanceAdmission$' -count=1 -v ./pkg/jobs
```

All passed; integration statement coverage is 97.0%. Every key-changing UPDATE
was observed waiting until transaction end. Heartbeat response shape is one
boolean, and all three empty-versus-maximum allocation comparisons met the
64 KiB independence threshold. The complete performance matrix is recorded in
`docs/performance.md`. The disposable containers were removed.

A standalone external module outside the repository was updated for
`Heartbeat(...) error`, asserted `jobs.Store` compatibility, retained the Claim
reconciliation test, and passed:

```text
go test -mod=readonly -count=1 ./...
go build -mod=readonly ./...
```

Two PostgreSQL launcher attempts failed before test execution because direct
Docker socket access was denied and then because an exported URL was scoped to
a piped subshell. The valid run used the host's noninteractive `sudo docker`
path and exported the URL in the parent shell. Failed setup logs were
overwritten and are not represented as test passes.

## Artifact inventory

Artifact root:

```text
/home/linus/.cache/openclaw-code-index/gotth-jobs/53cf140090cb7c1bc2076579437aab8edd3a0229/artifacts
```

| Artifact | SHA-256 |
| --- | --- |
| `bundle-verify.log` | `3b80beb75fe0293692321181c611e2c59233823be7fd9d6341650d8ef18bdd9c` |
| `clean-gates.log` | `ea7945c31ba1237acb17463c63caa6a91cce8370cb338e6189a967cb62ff134c` |
| `coverage.out` | `2f3caffdb22cf93d02492132e92a3de54ecce15365d6125832e6657a05397aa7` |
| `affected-coverage-gaps.log` | `f90bcbd19102002eeec504f0aa37e70fc8002f01ddf5f45d21535fe404846f28` |
| `integration.log` | `6736b2ab82f74b79d2e466ca17f54e825f2004e232570ac60cd0ff1c5604402c` |
| `integration-coverage.out` | `2f3caffdb22cf93d02492132e92a3de54ecce15365d6125832e6657a05397aa7` |
| `postgresql-image.json` | `ea1f9a4b971fc46a7ddf85058899556ac0d523ed0bd9bf962bde7d0b12c5fa8b` |
| `postgresql-repeat.log` | `e7d6b93eb8c32f2044027b861e8d7b89745ad19e0d06cf37b21e468feeb6dfe8` |
| `performance.log` | `0e61cfcc03ff0afe63ef25679e0a87ff60f0c924c80c152bbe3d0fe2b61a6c9f` |
| `external-consumer.log` | `a74e9aaf058b4e4f9baca424416d81eff08cc41ba499a5d382841216886c0a19` |
| `fuzz.log` | `fc5af7d4b8ceb34816c34b38e81c7949e9f152e78ebae0848a06e25a7e0c0b38` |

## Remaining gate

Implementation and exact-source evidence gates are complete for this repair.
Final admission remains active pending two fresh attributable independent
reviews pinned to the final candidate tree. Those reviews are
orchestrator-owned; this worker neither creates them nor claims a result.
