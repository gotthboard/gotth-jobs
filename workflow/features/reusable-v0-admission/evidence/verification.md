# Reusable v0 admission evidence

## Identity and scope

- Baseline: `874212b762571cd88322867872e458af0d9e0435`.
- Contract: `a1835f320f66107545f58a6462a17ee9d97cf95f`.
- Initial implementation: `64a053d51583854f13d8000c42345a645c993bf4`.
- Bounded retry correction: `4e76bc508b54e66ea16a7418a052f3d3dd14b049`.
- Admitted source: `d800418f2013b8e8e24c61d9baed38e10dffe26e`.
- Branch: `feature/reusable-v0-admission` in an isolated worktree.
- No tag, remote push, live database, deployment, secret, or consumer was
  changed.

## Capacity and toolchain

The preflight checked both bytes and inodes before dependency or container
work. The agent host root had 425 GiB available with 5% bytes and 1% inodes
used. The development host root had 1.5 TiB available with 23% bytes and 60%
inodes used. Neither approached the 90% stop threshold.

- Canonical local compiler: Go 1.26.6-X:nodwarf5, Linux amd64.
- Integration compiler: Go 1.26.5-X:nodwarf5, Linux amd64. The patch-level
  mismatch is disclosed; local admission remains bound to `.go-version`.
- PostgreSQL: 17.10 in
  `postgres:17@sha256:a426e44bac0b759c95894d68e1a0ac03ecc20b619f498a91aae373bf06d8508d`.
- Docker Engine on the disposable validation host: 29.7.1.
- Graphify: 0.9.32.

## Tests-first defect record

Each production slice began with a compiling failure until its public or
private contract existed. Real PostgreSQL then exposed three defects that fake
rows could not: an empty payload encoded as SQL NULL, an ambiguous returned
column in the claim join, and exhausted-lease reaping rolled back when no job
was claimable. Cold review later exposed range-limited `UnixNano`
fingerprinting, silent sub-microsecond truncation, insufficient custom-store
lease validation, and an unbounded zero-initial retry calculation. Each defect
received a regression test before admission.

## Local gates

The admitted source passed:

```text
go test -mod=readonly ./...
go vet -mod=readonly ./...
go build -mod=readonly ./...
go test -mod=readonly -race ./...
go test -mod=readonly -race -count=50 ./...
go test -mod=readonly -coverprofile=/tmp/gotth-jobs-coverage-final.out ./...
```

Canonical local statement coverage is 96.5%. The remaining statements are
defensive failure handling around injected database errors, cryptographic
entropy failure, and the impossible missing embedded subtree. Every claimed
state transition and public sentinel has direct unit or PostgreSQL coverage.

## Fuzz admission

Two fresh five-second admissions at the admitted source passed:

- `FuzzEnvelopeValidationNeverPanics`: 111,741 executions.
- `FuzzBoundedFailureIsValid`: 45,631 executions.
- Total: 157,372 executions without a product failure.

## PostgreSQL integration

The race-instrumented integration suite passed against the pinned disposable
PostgreSQL 17.10 container. It asserts server major version and UTF8 encoding,
atomic consumer rollback, sequential and concurrent idempotency, concurrent
nonduplicating claims, stale-token fencing, lease expiry and replacement,
heartbeat, retry and exhaustion, cancel, dead pagination, redrive, counts,
future scheduling, bounded exhausted reaping, and the worker loop.

Remote compiler coverage was 96.0%; compiler patch differences account for
the distinct instrumentation result. Cached artifacts tied to the admitted
source are:

| Artifact | SHA-256 |
| --- | --- |
| `integration-final.log` | `25e2c55e8ab44f613bac8f89be53538ab9152e40653c532e19fa92a1452daf03` |
| `integration-coverage-final.out` | `95a22d0db7240cfd17e2332be1868b711bde01d2db9266bc3983bc465fab13e4` |
| `performance-final.log` | `151faa22e3e1815f6b26941dca39747a1fba079694b014f669e196014f212a52` |

The bounded cache root is
`~/.cache/openclaw-code-index/gotth-jobs/d800418f2013b8e8e24c61d9baed38e10dffe26e/`.

## Performance admission

The final uninstrumented performance run passed. Exact percentiles and the
single locked-prefix sample are in `docs/performance.md` and the hashed log
above. No optimization or speedup is claimed, so hotspot share, local speedup,
and Amdahl prediction are N/A. The separate race-instrumented integration run
is not used as a timing source.

## Clean clone and external consumer

A no-local clone checked out the exact admitted source and passed readonly
race, vet, and build commands. A separate module used a local `replace` only
for the clean-clone admission, imported `pkg/jobs`, asserted the `Store`
surface, constructed every public value family, traversed every public
sentinel, read `Migrations`, and passed readonly test and build commands.

## Graph review

Graphify extracted the exact admitted source in code-only mode:

- 214 nodes, 491 edges, and 15 communities;
- zero missing or dangling endpoints;
- zero self-loops;
- zero exact duplicate edges;
- zero directed or undirected same-endpoint collision groups.

`transact` reaches every mutating API. `scanJob` reaches every API returning a
database job. `requestFingerprint` reaches enqueue and its direct regression
tests. The graph file SHA-256 is
`3dbb28dcd34cbe2d55b156a2840edb25284ad4234c37770d93c445fa06c79089`.
The code-only extractor skipped documentation and seven non-code files. Its
optional SQL parser was unavailable; installing another dependency was not
justified because the migration and SQL paths were reviewed directly and
executed against PostgreSQL.

## Cold review disposition

- Judge 1: FAIL; all findings corrected.
- Judge 2: FAIL; all findings corrected.
- Judge 3: CLEAN.
- Judge 4: CLEAN, separate fresh confirmation of the admitted source.

The exact findings and rulings are in the sibling `review/` directory. There
are no exceptions, deferred required gates, or unresolved high-risk coverage
items.
