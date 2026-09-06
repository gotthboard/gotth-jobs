# Reusable v0 admission evidence

## Identity and disposition

- Baseline: `874212b762571cd88322867872e458af0d9e0435`.
- Judge-10-rejected candidate:
  `a6900a151c2fecf8eae21d845f82880643f81d28`.
- Independent report: `/tmp/gotth-jobs-independent-judge-10.md`.
- Exact implementation repair source:
  `389915b6c4f27b1a2d5912de369a80b918c394fb`.
- Source tree: `81511f58ac262b3adcfb5ce3a77d381f84288116`.
- Source bundle SHA-256:
  `04a057763025faafc8c4426c343400be5851de25855dcc39815bd140923db709`.
- Branch: `feature/reusable-v0-admission` in the assigned isolated worktree.
- State: active. This repair worker does not claim independent final
  admission.
- No tag, push, merge, release, pull request, deployment, remote change, live
  database, or external consumer was changed.

Judge 10 found two Worker-boundary defects. Under
`GODEBUG=panicnil=1`, `recover()` returns nil for `panic(nil)`, so the prior
non-nil check incorrectly acknowledged the attempt through Complete. A custom
Store could also return a preallocated large unknown `Job.State`; `%q` and the
outer validation wrapper copied that complete untrusted value into errors.
The report found no additional material defect. Historical reviews do not
admit this repair; two fresh orchestrator-owned reviews remain required.

## Repair and contract

`callHandler` now sets an explicit completion flag only after the handler
returns normally. Its deferred function recovers every unwind without
inspecting or formatting the panic value, then returns the existing bounded
`errHandlerPanicked`. This includes `panic(nil)` in legacy runtime mode and
preserves the retryable Fail path; Complete is never called for an unwind.

`validateStoredJob` now returns a package-owned constant error for unknown
state. `validateClaimedAttempt` retains the public `ErrInvalid`
classification, but neither layer interpolates the state. The audit confirmed
that the only other state formatting sites receive scanner-bounded values, so
they were not changed. Public API and at-least-once behavior are unchanged.

## Expected-red evidence

Both regressions were added before production changes. The controlled test
sets `GODEBUG=panicnil=1`, invokes `panic(nil)`, and asserts one Fail plus zero
Complete calls. The exported `Worker.Run` test uses a custom Store returning a
preallocated 1 MiB unknown state, then asserts `ErrInvalid`, no handler call,
bounded error text, and no source-sized library allocation.

Against rejected candidate `a6900a1` with only the retained test patch,
`panic(nil)` made one Complete and zero Fail calls. The large-state path
allocated 5,284,888 bytes and returned a 1,048,665-byte error. The focused
command failed exactly those two checks.

| Artifact | SHA-256 |
| --- | --- |
| `expected-red-tests.patch` | `c182739bd7921ea2f742fdaaef182951466aa95e45f698d5fa78374f3603e263` |
| `expected-red.log` | `c7dd19b77bd9edadbb05ad623473d649b7c1af2749998c720d9840aea83f75f5` |

## Local checks

Agenthost work remained lightweight. The two focused tests passed for ten
repeats, the complete package passed once, integration-tag compilation passed
without running integration, and vet, formatting, and `git diff --check`
passed. Go commands used `GOMAXPROCS=2` and `go test -p=1`.

## Exact-source development record

The complete source bundle was cloned detached at:

`/home/linus/.cache/openclaw-code-index/gotth-jobs/389915b/source`

The retained successful runner writes both streams directly to its transcript
and enables `set -x`. It records literal commands, cwd, Go 1.26.6 and host
toolchains, `GOMAXPROCS=4`, source bundle hash, exact HEAD/tree, clean pre/post
status, packages, regexes, options, counts, timing, and an explicit completion
sentinel. This document intentionally does not reconstruct those commands.

Exact source passed format, vet, unit, build, full race, 50 race-instrumented
repeats of the two repairs plus adjacent Worker paths, 20 verbose allocation
runs, 100 focused repeats, and unit coverage. The 100-repeat test process took
0.67s real, 0.80s user, and 0.35s system; it includes compilation, fixture
construction, forced garbage collection, and test overhead, so it is not
claimed as function latency.

Across the retained 20 runs, rejection of a preallocated 1,048,576-byte state
allocated 648-1,088 bytes and returned an 86-byte error. Unit statement
coverage is 97.5%; `callHandler` and `validateStoredJob` are 100% covered.
Exact unrelated residual blocks are retained in `coverage-gaps.log`; there is
no changed-path gap.

The first runner invocation failed before cloning or running any gate because
`git bundle verify` lacked a repository context. Its runner and transcript are
retained. The corrected runner cloned first, verified the same bundle from the
detached repository, and completed every stated gate.

## Proportional gate scope

No existing fuzz target reaches handler invocation or stored jobs returned by
a custom Store. `FuzzEnvelopeValidationNeverPanics` covers enqueue input and
`FuzzBoundedFailureIsValid` covers handler error normalization; neither repair
changed those paths, so fuzz was not rerun.

No PostgreSQL test or performance matrix was run because the repair changes no
SQL, transaction, scanner, PostgreSQL-backed successful path, or Store method.
The standalone external-consumer fixture was not rerun because no exported
identifier or signature changed. Graph extraction was not rerun because no
call edge or SQL statement changed. Prior PostgreSQL, consumer, performance,
fuzz, and graph results remain ancestor evidence only and are not rebound to
`389915b`.

## Artifact inventory

Artifact root:

`/home/linus/.cache/openclaw-code-index/gotth-jobs/389915b/artifacts`

| Artifact | SHA-256 |
| --- | --- |
| `artifact-inventory.sha256` | `b535df026a6d98d9c4806137dad6cbbf67fd74c99971e7ab8d811c47e20e4308` |
| `coverage-functions.log` | `4071304ae580184d89d7bcfa0319b167c05f8c770f894d0dbbfaa7e560966afa` |
| `coverage-gaps.log` | `5da31787a31ba378672bbc875bb402771f1e369891d20243eeb38d68334bcd05` |
| `coverage.out` | `08fccd3e0ffb4df21441f209e1a6b08f9dec12beef6b7439c94a86c237145160` |
| `expected-red-tests.patch` | `c182739bd7921ea2f742fdaaef182951466aa95e45f698d5fa78374f3603e263` |
| `expected-red.log` | `c7dd19b77bd9edadbb05ad623473d649b7c1af2749998c720d9840aea83f75f5` |
| `run-verification.failed.sh` | `9f8af217c3dba1482f477139520b41986e2e8f0f0ee6b7fc7e0404180ea10c58` |
| `verification-transcript.failed.log` | `3dce45e72916b9c38aaa4880da95367c0114b8e7c8b882f85830de1ab478c8db` |
| `run-verification.sh` | `0448d46b9c988f1a0cd91376df4412614e5caa7a60c0d9dd8b7205c8ec3648c1` |
| `verification-transcript.log` | `9eae244433eaeb42a0777c02fa3ca1374b154d20b2de3d9abdbebbffd0ba7932` |
| `source.bundle` | `04a057763025faafc8c4426c343400be5851de25855dcc39815bd140923db709` |

## Remaining gate

Implementation and exact-source evidence gates are complete for this repair.
The final candidate identity and proportional detached clean-clone result are
recorded in `/tmp/gotth-jobs-repair-handoff.md`, because a Git commit cannot
embed its own object ID. Final admission remains active pending two fresh
attributable independent reviews pinned to that final candidate. Those reviews
are orchestrator-owned; this worker neither creates them nor claims a result.
