# Reusable v0 admission evidence

## Identity and disposition

- Baseline: `874212b762571cd88322867872e458af0d9e0435`.
- Judge-6-rejected candidate:
  `6126a751cfc70d47b53449927c1d1552f604ba4d`.
- Independent report: `/tmp/gotth-jobs-independent-judge-6.md`.
- Exact implementation repair source:
  `1fc2a7b3db6edd354e7efa4154f87032866cb090`.
- Source bundle SHA-256:
  `805670ac9ce5847f75e8a9919b57939aedbd9f6b75d1a891ad3bd4a010e6e0e7`.
- Branch: `feature/reusable-v0-admission` in the assigned isolated worktree.
- State: active. This repair worker does not claim independent final
  admission.
- No tag, Git remote configuration, push, merge, release, pull request,
  deployment, live database, secret, or external consumer was changed.

Judge 6 found five defects: Worker erased reconciliation handles for unknown
acknowledgement commits; bounded bytea scanning depended on the connection's
pgx query mode; Worker accepted renewal intervals without a sufficient first
heartbeat budget; EnqueueTx accepted isolation levels its snapshot mechanism
does not support; and stored-row validation accepted NULL payloads and invalid
timestamps. Historical reviews do not admit this source. Two fresh
attributable orchestrator-owned reviews remain required.

## Contracts and source inspection

The worker read the complete Judge 6 report, PRD, architecture,
implementation specification, runtime boundary, workflow state/evidence, all
transaction wrappers, every job scan and acknowledgement caller, and pinned
pgx 5.10 source.

pgx consumes leading query options from the actual Query/QueryRow argument
list. DescribeExec obtains result OIDs each execution and uses the extended
protocol with two round trips. Exec and SimpleProtocol use text results, and a
result-format-by-OID map leaves unmapped OIDs in text. Binary ByteaCodec calls
`BytesScanner.ScanBytes` directly with borrowed driver memory; text ByteaCodec
first allocates a decoded byte slice. The final helper therefore passes
DescribeExec plus binary formats for every job-column OID: text, bytea, int4,
and timestamptz. This both protects payload allocation and preserves binary
decoding across PostgreSQL's far-future finite timestamp range.

pgx Tx does not retain inspectable public TxOptions. EnqueueTx therefore reads
`current_setting('transaction_isolation')` inside the supplied transaction and
accepts only `read committed`. It does not commit, roll back, or retry; a
consumer retry must restart its complete domain transaction.

All transaction wrappers were inspected for analogous value loss. The generic
transaction helper already returns a body value with an unknown commit error;
Enqueue, Claim, Complete/Fail, Cancel, and Redrive preserve that value.
Heartbeat has no value result, so Worker preserves its already known claimed
job and exact lease.

## Expected-red evidence

| Artifact | SHA-256 |
| --- | --- |
| `/tmp/gotth-jobs-red-judge6-unit.log` | `7367411e9e833ac5a09ea2154dbf981eac47c07b4aa42efa1632e0419f80db0e` |
| `/tmp/gotth-jobs-red-judge6-tests.patch` | `cceb13cffc36c9eea43ee9b4f664ad8943b3a9f79bb7e06553ea63ef161a1174` |
| `judge6-red-6126/artifacts/expected-red-integration.log` | `8e22dbc51e880d56c3416f485d6f116836cf1b8cfb12e250909a75b4fd7c027e` |

Before production changes, focused unit tests observed all three unknown
acknowledgements without typed reconciliation, accepted a heartbeat interval
one microsecond above half the lease, accepted Repeatable Read EnqueueTx,
accepted NULL payloads and every mandatory/optional timestamp invalid class,
and allowed invalid custom Store rows to reach the handler.

The pre-fix PostgreSQL 17 run accepted Repeatable Read and Serializable
EnqueueTx. Maximum/oversized bytes per operation were:

| Default mode | Maximum | Oversized rejection |
| --- | ---: | ---: |
| CacheStatement | 1,518,781 | 6,832 |
| CacheDescribe | 1,537,053 | 6,094 |
| DescribeExec | 1,521,356 | 11,038 |
| Exec | 3,527,920 | 1,401,950 |
| SimpleProtocol | 3,707,986 | 1,912,568 |

Exec and SimpleProtocol therefore allocated an additional payload-sized text
decode for accepted rows and before oversized rejection.

One intermediate post-repair source, `b56e350`, forced binary bytea but left
other OIDs in text. Its PostgreSQL race gate correctly failed while scanning a
maximum finite timestamp; failed log SHA-256 is
`495a16e35c7067f23af9265fb5418e94d3afbd66357f869a3876698a83f67f33`.
That source and its earlier clean gates are superseded and are not final pass
evidence.

## Repairs and focused checks

`LeaseReconciliationError` wraps the original commit-unknown Heartbeat,
Complete, or Fail error and exposes reconciliation-only Job and Lease values.
Its Error text contains neither job ID nor token. Worker captures Complete and
Fail's returned Job, uses its known job for Heartbeat, and performs exactly one
acknowledgement call.

Worker rejects heartbeat intervals above half the lease while accepting the
exact half boundary. The remaining budget covers post-Claim startup,
scheduling, and the renewal round trip; no hard liveness guarantee is claimed
across arbitrary pauses.

EnqueueTx validates before copying, inspects isolation before insertion, and
rejects Repeatable Read and Serializable with ErrInvalid. Oversized requests
still allocate no proportional clone and perform no transaction query.

The owning payload scanner rejects SQL NULL distinctly, checks source length
before allocation, and makes one library ownership copy of accepted binary
bytes. Mandatory timestamp destinations retain SQL NULL presence. SQL-decoded
times are normalized to UTC, then mandatory and present optional timestamps
are checked for nonzero presence, finite PostgreSQL range, and microsecond
precision. Custom Store jobs must already be UTC. State/attempt and
lease/finished presence shapes remain enforced.

Local post-repair checks on `agenthost` used Go 1.26.6-X:nodwarf5 and only
lightweight constrained commands:

```text
GOMAXPROCS=2 go test -mod=readonly -p=1 ./pkg/jobs -run='<defect tests>' -count=10
GOMAXPROCS=2 go test -mod=readonly -p=1 ./pkg/jobs -count=1
GOMAXPROCS=2 go vet -mod=readonly ./pkg/jobs
GOMAXPROCS=2 go test -mod=readonly -p=1 -tags=integration ./pkg/jobs -run='^$'
```

All passed. Formatting and `git diff --check` also passed.

## Exact-source development gates

The complete Git bundle was cloned detached on `development` at:

```text
/home/linus/.cache/openclaw-code-index/gotth-jobs/1fc2a7b3db6edd354e7efa4154f87032866cb090/source
```

The clone selected Go 1.26.6 and remained clean. Exact source passed:

```text
tracked Go format check
go vet -mod=readonly ./...
go test -mod=readonly -count=1 ./...
go build -mod=readonly ./...
go test -mod=readonly -race -count=1 ./...
go test -mod=readonly -race -count=50 -run='<22 affected/adjacent tests>' ./pkg/jobs
go test -mod=readonly -count=1 -coverprofile=<artifact>/coverage.out ./...
go test -mod=readonly -run='^$' -fuzz=<each of two targets> -fuzztime=10s ./pkg/jobs
```

`commands.txt` records the full unabridged regular expressions and exact
working directories for every command summarized in this document.

Unit statement coverage is 97.3%. EnqueueTx isolation, job query options,
bounded payload scanning, scanJob, scanJobWithFingerprint, scanJobRow,
validateStoredJob, and every LeaseReconciliationError method are 100% covered.
Every new Worker acknowledgement branch is covered. `runAttempt` is 97.8%; its
only uncovered block is the preexisting RetryPolicy.Delay error after a
handler failure, unreachable through a Worker.Run whose configuration and
claimed attempt were validated. Other exact gaps are preexisting entropy,
nil/input, migration panic, and unrelated operation error branches listed in
`coverage-gaps.log`.

## PostgreSQL 17 and allocation

Disposable integration used PostgreSQL 17.10 at exactly:

```text
postgres:17@sha256:a426e44bac0b759c95894d68e1a0ac03ecc20b619f498a91aae373bf06d8508d
```

Exact source passed:

```text
go test -mod=readonly -race -tags=integration -count=1 ./...
go test -mod=readonly -tags=integration -count=1 -coverprofile=<artifact>/integration-coverage.out ./...
go test -mod=readonly -tags=integration -run='^TestPostgreSQLJobPayloadScanningAcrossDefaultQueryModes$' -count=3 -v ./pkg/jobs
go test -mod=readonly -race -tags=integration -run='<isolation and two key-retention interleavings>' -count=10 -v ./pkg/jobs
```

Integration statement coverage is 97.3%. Full integration includes minimum
and maximum availability timestamps and all lifecycle/concurrency behavior.
Every supported pgx default mode passed maximum and oversized payload tests in
three runs. Accepted maximum rows used about 1.50-1.61 MiB/op across all modes;
oversized rejection used 3,369-12,840 bytes/op. No text-configured mode added a
payload-sized allocation. The accepted figure includes pgx/network storage in
addition to the scanner's single library ownership copy.

Ten race-instrumented repeats rejected Repeatable Read and Serializable before
insert and passed both authenticated-row/key-update retaining interleavings.
The disposable container was removed.

## External consumer and performance

A standalone module outside the repository implements the current error-only
Heartbeat Store contract. It drives Worker to a commit-unknown Complete,
asserts errors.Is/errors.As, exact Job and Lease accessors, no ID/token in
Error text, and exactly one Complete call. It passed:

```text
go test -mod=readonly -count=1 ./...
go build -mod=readonly ./...
```

The exact-source complete performance matrix passed. DescribeExec's extra
round trip is intentionally visible and reduced throughput compared with the
prior cached-mode source; exact measurements are in `docs/performance.md`. No
optimization or latency guarantee is claimed.

## Artifact inventory

Artifact root:

```text
/home/linus/.cache/openclaw-code-index/gotth-jobs/1fc2a7b3db6edd354e7efa4154f87032866cb090/artifacts
```

| Artifact | SHA-256 |
| --- | --- |
| `clean-gates.log` | `88858b20f9499d3732b583b8d3829ba00419b05f85d79eb5f8e79817a2b840ba` |
| `commands.txt` | `9040a6b4ae64eb382ea8aca36f301958b77ec84f48527aa3eecce8a9ded08b28` |
| `coverage-functions.log` | `325db50c41df94e4d5293bf4cd116e7fe22a84bf03c5725d33dcf9fc5fceb813` |
| `coverage-gaps.log` | `1d05b8d48c47b147e41802a16a8dcb0c6fb3685d2b12bf3d99296be2bc8d2910` |
| `coverage.out` | `87f9078135f075c4719ec4a4c8e52e558c6b788773a2877c721610f04f5a1c9a` |
| `external-consumer.log` | `a30c9457de037b44d6f0e2a745bd8dbf8b40e6d5115b0f8c18fad0bc37043f25` |
| `fuzz.log` | `d10c09860bbe0c50d49ce33324729cde8fa24145011a34377d83bd9b572a268a` |
| `integration-coverage.out` | `87f9078135f075c4719ec4a4c8e52e558c6b788773a2877c721610f04f5a1c9a` |
| `integration.log` | `f699c9a17fc97ecafec96ab93fa0c270af5e8ef02229774714191ecb28309d36` |
| `performance.log` | `9fc784f5968f088c97a20777516624f694fa527a3b3dc48331fb44d9085f501e` |
| `postgresql-image.json` | `ccf16c283a6d50b470635a882630526b29503e512c745269e6390be9c0c5fdd2` |
| `postgresql-interleavings.log` | `4cb0340cd68d4075b4bf881998731e92258077fbe1c2c403145bf5701e7acfd7` |
| `postgresql-query-modes.log` | `18be4a1fc9924f97366ad24c110b79e37cba695b30110101086387f81ec6bad7` |

## Remaining gate

Implementation and exact-source evidence gates are complete for this repair.
Final admission remains active pending two fresh attributable independent
reviews pinned to the final candidate tree. Those reviews are
orchestrator-owned; this worker neither creates them nor claims a result.
