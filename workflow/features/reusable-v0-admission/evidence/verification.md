# Reusable v0 admission evidence

## Identity and disposition

- Baseline: `874212b762571cd88322867872e458af0d9e0435`.
- Judge-9-rejected candidate:
  `8213b6d8a14cd95545d95d2119d1bd5bd231c9e5`.
- Independent report: `/tmp/gotth-jobs-independent-judge-9.md`.
- Exact implementation repair source:
  `b54c0fcabb5f7f43e3268749f75a59fbfd27413d`.
- Source tree: `1159e048b57ce06eb23bc55d2d987deadaa94f4d`.
- Source bundle SHA-256:
  `e93e80b32f4a22e56b8e281d205f2cdf798278c849a899805299e86e0fee173d`.
- Branch: `feature/reusable-v0-admission` in the assigned isolated worktree.
- State: active. This repair worker does not claim independent final
  admission.
- No tag, push, merge, release, pull request, deployment, remote change, live
  database, or external consumer was changed.

Judge 9 found one implementation defect. `boundedFailure` called
`strings.ToValidUTF8` and `strings.ReplaceAll` over the complete unbounded
handler error string before truncation. Invalid UTF-8 and NUL input therefore
caused source-sized or larger library allocation and complete-source scanning.
The report found no additional material defect. Historical reviews do not
admit this repair; two fresh orchestrator-owned reviews remain required.

## Repair and contract

`boundedFailure` calls `err.Error()` exactly once. After that delegated call
returns, it selects at most a `MaxFailureBytes`-scale byte prefix before any
normalization. A source known to exceed the final limit is capped with the
three-byte ellipsis already reserved. Only that bounded prefix reaches
`strings.ToValidUTF8` and NUL replacement.

If invalid UTF-8 or NUL replacement expands a within-limit source, output is
UTF-8-boundary-truncated with the same reserved ellipsis. The result remains no
larger than `MaxFailureBytes`, valid UTF-8, NUL-free, and uses U+FFFD for both
invalid input and NUL redaction. Library time after `Error()` is
Theta(1+min(n, MaxFailureBytes)); worst-case auxiliary allocation is
O(MaxFailureBytes), independent of the full source length. Work performed
inside a custom consumer `Error()` method is outside the library bound.

The public API, Store contract, PostgreSQL behavior, fencing, retries, and
at-least-once delivery contract are unchanged.

## Expected-red evidence

The allocation test was added before production changes and used preallocated
1 MiB valid, alternating-invalid, and NUL-containing strings. It also asserts
one `Error()` call, final byte limit, valid UTF-8, no NUL, replacement-rune
redaction, and ellipsis behavior.

Against rejected candidate `8213b6d` with only the retained test patch, the
valid control passed while invalid UTF-8 allocated 8,736,880 bytes and NUL
input allocated 2,101,280 bytes. The focused test failed exactly those two
allocation checks.

| Artifact | SHA-256 |
| --- | --- |
| `expected-red-tests.patch` | `28351cdf9189859a7db7c41593c8784695fd5e2a6130eb62d7a833f23550fbf5` |
| `expected-red-allocation.log` | `e37b2fa858eff27f85ec4b3d93e78868575d5dda01a532c7bb9eb39826f26f34` |

## Local checks

Agenthost work remained lightweight. Focused output/allocation tests passed for
ten repeats, the complete package passed once, integration-tag compilation
passed without running integration, and vet, formatting, and
`git diff --check` passed. Go commands used `GOMAXPROCS=2` and
`go test -p=1`.

## Exact-source development record

The complete source bundle was cloned detached at:

`/home/linus/.cache/openclaw-code-index/gotth-jobs/b54c0fc/source`

The retained successful runner writes both streams directly to its transcript
and enables `set -x`. It records literal commands, cwd, Go 1.26.6 and host
toolchains, source bundle hash, exact HEAD/tree, clean pre/post status,
packages, regexes, options, counts, timing, and an explicit completion
sentinel. This document intentionally does not reconstruct those commands.

Exact source passed format, vet, unit, build, full race, 50 focused race
repeats, 20 verbose allocation runs, a 100-repeat Bash-timed focused workload,
unit coverage, and the 10-second bounded-failure fuzz target. The 100-repeat
test process took 1.94s real, 3.48s user, and 1.45s system; it includes
compilation, fixture construction, forced garbage collection, and test
overhead, so it is not claimed as function latency.

The 20-run allocation samples were stable:

| Preallocated source | Source bytes | Library bytes allocated | Result bytes |
| --- | ---: | ---: | ---: |
| valid ASCII | 1,048,576 | 4,096 | 4,096 |
| alternating invalid UTF-8 / ASCII | 1,048,576 | 29,952 | 4,095 |
| ASCII / NUL | 1,048,576 | 12,288-12,320 | 4,096 |

Unit statement coverage is 97.5%, and `boundedFailure` is 100% covered. Exact
unrelated residual blocks are retained in `coverage-gaps.log`; there is no
changed-path gap.

The first runner invocation failed before any verification gate because the
host does not provide `/usr/bin/time`. Its runner and transcript are retained.
The corrected runner used Bash `time -p` and completed every stated gate.

## Proportional gate scope

No PostgreSQL test was run. The repair modifies one private, local helper after
a handler failure and changes no SQL, database transaction, Store method,
public API, or successful Worker path. The canonical PostgreSQL correctness and
performance workloads do not exercise handler failure normalization; rerunning
them would not validate this defect.

The standalone external-consumer fixture was not rerun because no public
contract changed. Prior PostgreSQL and consumer results remain ancestor
evidence only and are not rebound to `b54c0fc`.

Exact-source Graphify 0.9.32 code-only extraction reported 316 nodes, 835
edges, and 17 communities. Diagnostics reported zero unverified nodes,
non-object edges, missing/dangling endpoints, self-loops, exact duplicates, or
same-endpoint groups. The optional SQL parser was unavailable; no SQL graph
coverage is claimed.

## Artifact inventory

Artifact root:

`/home/linus/.cache/openclaw-code-index/gotth-jobs/b54c0fc/artifacts`

| Artifact | SHA-256 |
| --- | --- |
| `artifact-inventory.sha256` | `24f10937efae782af13f2c0473bb74dd9743446a18a01f1edf0605df9b084cc6` |
| `expected-red-tests.patch` | `28351cdf9189859a7db7c41593c8784695fd5e2a6130eb62d7a833f23550fbf5` |
| `expected-red-allocation.log` | `e37b2fa858eff27f85ec4b3d93e78868575d5dda01a532c7bb9eb39826f26f34` |
| `run-verification.sh` | `a1cc08f116e75f0df63cb36ecf48f48355dd99db5e6326eb54d6e80df9a20c2a` |
| `verification-transcript.log` | `2bf429cfd652172c8f5d1c44f88294af3cc1e4f8430060823050718d0c950056` |
| `coverage.out` | `4d0361bb9a1a84f6e3bec6073ed77c0441dbeca21c97a4832265a6b657ae62bb` |
| `coverage-functions.log` | `98d5b2568a589730eb060d3040b92053de2ffb9ff4e2a33d04912986b06dd978` |
| `coverage-gaps.log` | `5da31787a31ba378672bbc875bb402771f1e369891d20243eeb38d68334bcd05` |
| `run-verification.failed.sh` | `4b27d0d8e15cfebe9e6d9c6bbeb2965312cfc9bd9be60a8f2c907f0ec335f850` |
| `verification-transcript.failed.log` | `7daeeed1dd4a9615f7f4b3d4df75befd2b78edab34752ccf23fbde69a69ccd65` |
| `run-graph-verification.sh` | `bc639bcacc31127a82581635d69bafef51f6fe71c160234d703e715ed82c2dbb` |
| `graph-verification-transcript.log` | `95f9905cbb44dbcf4c1a1336a61a832d04f5e508e12ae0177a7fa88725190588` |
| `graph/graphify-out/graph.json` | `8dc8f55e0645dbae1afbf97ed09e54723f5a7464be1e1afee23f3431d633b465` |
| `graph-diagnose.json` | `b06b596182bb2a635b438ff83e5dabba5b4daea6aa7dc50c2aa0196acedade5f` |
| `source.bundle` | `e93e80b32f4a22e56b8e281d205f2cdf798278c849a899805299e86e0fee173d` |

## Remaining gate

Implementation and exact-source evidence gates are complete for this repair.
The final candidate identity and proportional detached clean-clone result are
recorded in `/tmp/gotth-jobs-repair-handoff.md`, because a Git commit cannot
embed its own object ID. Final admission remains active pending two fresh
attributable independent reviews pinned to that final candidate. Those reviews
are orchestrator-owned; this worker neither creates them nor claims a result.
