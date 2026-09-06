# Reusable v0 admission evidence

## Identity and disposition

- Baseline: `874212b762571cd88322867872e458af0d9e0435`.
- Judge-8-rejected candidate:
  `2486b4732076976d4565e235de1d97c36b361951`.
- Independent report: `/tmp/gotth-jobs-independent-judge-8.md`.
- Exact implementation repair source:
  `4ec1970ed632f0306cc772bceeae8e15e17f5ab6`.
- Source tree: `ea2427f5410f3a438ac29dd3df4f8ddef42ac4e1`.
- Source bundle SHA-256:
  `5617fb4970e26702829fbf058527db0fbe0d06cc47af4781558da0ab081c7379`.
- Branch: `feature/reusable-v0-admission` in the assigned isolated worktree.
- State: active. This repair worker does not claim independent final
  admission.
- No tag, push, merge, release, pull request, deployment, remote change, live
  database, or external consumer was changed.

Judge 8 found two implementation defects and one evidence defect. The scalar
state read after a rejected lease mutation scanned directly into `*string`,
allowing a source-sized allocation and treating unknown state as
`ErrLeaseLost`. `Counts` summed five filtered buckets without proving they
covered every row. Prior evidence prose used placeholders and did not retain a
literal command transcript or the external-consumer source. Historical reviews
do not admit this source; two fresh orchestrator-owned reviews remain required.

## Repair and contract

`classifyLease` now scans non-NULL state through `boundedTextScanner` with a
maximum equal to the longest allowed state before one bounded ownership
conversion. It explicitly recognizes `pending`, `running`, `succeeded`,
`dead`, and `canceled`. NULL, oversized, and unknown states return a
stored-data corruption/storage error and never an ordinary lease outcome.

`Counts` now selects the five known-state conditional counts and `count(*)` in
one query. It returns no observation when the total differs from the sum of the
known buckets. This catches both unknown and NULL state even if database
constraints have been damaged or bypassed.

The public API, sentinel set, fencing, transaction ownership, and at-least-once
delivery contract are unchanged. Existing stored-row corruption is reported as
a storage error, so this repair does not introduce an inconsistent new public
error identity.

The adjacent scan audit found no other job-state scalar read. Job-returning
queries already route all state fields through the bounded row scanner.

## Expected-red evidence

The tests were written before production changes. Focused local tests against
the rejected source failed because the destination was direct `*string`,
oversized and unknown state became `ErrLeaseLost`, NULL did not produce the
required classified corruption result, and the old five-column count fixture
could not prove completeness.

Retained real-pgx expected-red source was the exact rejected candidate with
only the hashed test patch applied:

```text
/home/linus/.cache/openclaw-code-index/gotth-jobs/judge8-red3-2486b47
```

| Artifact | SHA-256 |
| --- | --- |
| `artifacts/tests.patch` | `3842b8160e32519f566d445ea5ccfa92f05fec5f4b2a51da9cf60bba3fa5ae52` |
| `artifacts/expected-red-allocation.log` | `6acc99e9c22c55d4c126164051726de283cc5276d9e4a69bbcacb6decaad652c` |

The real driver returned `ErrLeaseLost` for a rejected 1 MiB state while
allocating 1,596,030-1,680,034 bytes/op across CacheStatement,
CacheDescribe, DescribeExec, Exec, and SimpleProtocol. That is the defect the
green allocation test distinguishes.

## Local checks

Agenthost work was limited to focused package checks with `GOMAXPROCS=2` and
`go test -p=1`. The new and adjacent tests passed for ten repeats, the whole
package passed once, integration-tag compilation passed, vet and formatting
passed, and `git diff --check` passed. CPU-heavy suites ran only on
`development`.

## Exact-source development record

The complete source bundle was cloned detached at:

```text
/home/linus/.cache/openclaw-code-index/gotth-jobs/4ec1970/source
```

The retained runner enables `set -x` after redirecting both output streams to
the transcript. Together they record the literal commands, cwd transitions,
Go and host toolchains, relevant integration environment, source bundle hash,
exact HEAD/tree, pre/post status, packages, regexes, tags, options, counts,
container identity, database identity, and external-consumer invocation. This
document intentionally does not reconstruct those commands from memory.

The transcript records successful format, vet, unit, build, full race, 50
focused race repeats, unit coverage, both 10-second fuzz targets, PostgreSQL
full race and coverage, three five-mode malformed/allocation runs, ten
race-instrumented focused repeats, the complete performance matrix, and the
external-consumer test/build. The disposable container was removed. Source
status was empty before and after all gates.

Unit and integration statement coverage are both 97.4%. `classifyLease`,
`Counts`, both borrowed-byte scanner methods, every shared job scanner and row
validator are 100% covered. Exact residual blocks are in
`coverage-gaps.log`; they are unrelated preexisting entropy/input/error paths.
No changed production path is uncovered.

## PostgreSQL and allocation

PostgreSQL 17.10 used the pinned image:

```text
postgres:17@sha256:a426e44bac0b759c95894d68e1a0ac03ecc20b619f498a91aae373bf06d8508d
```

Before any DDL, the runner recorded the explicit destructive-test opt-in and
queried the server for exact database name `gotth_jobs_test` plus exact comment
`gotth-jobs:dedicated-destructive-integration-test-v1`.

Malformed unknown and NULL state failed both lease classification and counts.
Across three uninstrumented runs, a rejected 1 MiB state used 5,963-39,351
bytes/op across all five supported modes. Ten race-instrumented repeats used
408,174-747,438 bytes/op, below the source size. These measurements include
driver, protocol, network, transaction, and race-runtime storage; the supported
claim is only that the bounded scanner does not make a library-owned
source-sized state string.

The exact-source performance matrix is recorded in `docs/performance.md`. No
optimization or latency guarantee is claimed.

## External consumer

The standalone module is retained outside the repository at:

```text
/home/linus/.cache/openclaw-code-index/gotth-jobs/4ec1970/external-consumer
```

Its `go.mod` SHA-256 is
`2690f8fa37347d5e3e3b126dc160b6f938613f1e99add6f538998daffc22db7b` and
its `go.sum` SHA-256 is
`10f85f016b4bb4919ddc6453cc8c09962bc186a29d39ed064332a23ed5a58115`,
and its `main_test.go` SHA-256 is
`a8e24085aaab58f0454b8405ce0ba013f7aa3c448a750170dff82eb69c932e45`.
The exact replacement path and literal test/build invocation are in the
retained transcript; both passed.

## Artifact inventory

Artifact root:

```text
/home/linus/.cache/openclaw-code-index/gotth-jobs/4ec1970/artifacts
```

| Artifact | SHA-256 |
| --- | --- |
| `artifact-inventory.sha256` | `a0374719ee2b9163ca807a6366506590a413012d2ebfc1775a3965f4a0f198a0` |
| `run-verification.sh` | `4394a90a43486bcb64f459d02cf53fc290fec472e7409a503f53ecc1645e87fe` |
| `verification-transcript.log` | `23b15454b76a90e88e42952ea04f628619c50d2efa309585bd1527926e3e397d` |
| `coverage.out` | `79a11e92729d3261dc9fa2832e1c6f09721586fe6e8227e61d4ea52de4789d56` |
| `coverage-functions.log` | `dbac3e68bd9a0a36b9bcdb452100fe2d036141400245c6712b84d553c0db402d` |
| `coverage-gaps.log` | `69014389db8da5fda8342bea7e9541a1b5fea694ab2e65b7facf372f8281129c` |
| `integration-coverage.out` | `79a11e92729d3261dc9fa2832e1c6f09721586fe6e8227e61d4ea52de4789d56` |
| `integration-coverage-functions.log` | `dbac3e68bd9a0a36b9bcdb452100fe2d036141400245c6712b84d553c0db402d` |
| `postgresql-image.json` | `557203fa8ddb39ed2b8ad084c0ec9a040498828e9f7cb1b344ba43b98844d374` |
| `external-consumer-source.sha256` | `bdbc1d49cbd87ac3d15b770bec67d4cfde7a40abe98c37dbd38e1248bc15de2f` |
| `source.bundle` | `5617fb4970e26702829fbf058527db0fbe0d06cc47af4781558da0ab081c7379` |

## Remaining gate

Implementation and exact-source evidence gates are complete for this repair.
The final candidate identity and proportional detached clean-clone result are
recorded in `/tmp/gotth-jobs-repair-handoff.md`, because a Git commit cannot
embed its own object ID. Final admission remains active pending two fresh
attributable independent reviews pinned to that final candidate. Those reviews
are orchestrator-owned; this worker neither creates them nor claims a result.
