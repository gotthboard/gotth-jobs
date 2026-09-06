# Reusable v0 admission evidence

## Identity and disposition

- Baseline: `874212b762571cd88322867872e458af0d9e0435`.
- Judge-12-rejected candidate:
  `2ef420fdfa90c2f28c793564093e74d6c3e6158b`.
- Independent report: `/tmp/gotth-jobs-independent-judge-12.md`.
- Exact implementation repair source:
  `27bbaa962e1d7d65e2395a5bb212e92ea6e6d667`.
- Source tree: `7c057626752d2d5a41a6f5067dc070830ff5162c`.
- Source bundle SHA-256:
  `2f2fc2acd0dbf557da8cf663efbed3ff4685e2883dd562d186ebb9f706237236`.
- Branch: `feature/reusable-v0-admission` in the assigned isolated worktree.
- State: active. This repair worker does not claim independent final
  admission.
- No tag, push, merge, release, pull request, deployment, remote change, live
  database, or external consumer was changed.

Judge 12 found one Worker error-precedence defect and one evidence-path defect.
Claim checked `ErrNoJob` before `ErrCommitOutcomeUnknown`, while Complete and
Fail checked `ErrCanceled` first. Because the exported Store contract permits
joined errors and the API promises `errors.Is` traversal, those routine
identities could hide an unknown commit result and cause polling or successful
continuation. The Judge 10 evidence also described its artifact subdirectory
as the root while listing a source bundle stored in the parent directory.

## Repair and contract

Worker now checks `ErrCommitOutcomeUnknown` immediately after Claim, Complete,
and Fail return, matching Heartbeat's precedence. A nonzero unknown Claim
result becomes `ClaimReconciliationError`; a zero result returns a wrapped
unknown error without polling. Unknown Complete and Fail results become
`LeaseReconciliationError`, preserving the exact returned Job and known Lease.
Joined routine identities remain traversable but cannot select polling,
cancellation, implicit retry, or continued handling.

The public API, Store signatures, SQL, transaction behavior, and at-least-once
delivery contract are unchanged.

## Expected-red evidence

The custom-Store regressions were added before production changes. Claim cases
return `errors.Join(ErrCommitOutcomeUnknown, ErrNoJob)` with and without a
reconciliation Job. Complete and Fail return produced jobs with
`errors.Join(ErrCommitOutcomeUnknown, ErrCanceled)`. Each test supplies a
second Claim sentinel to prove that Worker does not poll or continue, and the
handle cases assert exact ID, token, job result, lease, error traversal, and
secret-safe typed error text.

Against rejected candidate `2ef420f`, both Claim cases reached a second Claim
and returned its continuation error. Complete and Fail each returned nil from
the attempt, reached a second Claim, and returned its continuation error. The
focused command failed all four paths.

| Artifact | SHA-256 |
| --- | --- |
| `artifacts/expected-red-tests.patch` | `fd1113084c40eb77a6631eaf48f258011517af4258d24acc919e64e84bd7d999` |
| `artifacts/expected-red.log` | `0e5d4f9566d9af1b8b331e73528108dfff3ae74c07d976c9e2168f60c5911c59` |

## Local checks

Agenthost work remained lightweight. The focused reconciliation set passed for
ten repeats, the complete package passed once, integration-tag compilation
passed without running integration, and vet, formatting, and
`git diff --check` passed. Go commands used `GOMAXPROCS=2` and
`go test -p=1`.

## Exact-source development record

The complete source bundle was cloned detached at:

`/home/linus/.cache/openclaw-code-index/gotth-jobs/27bbaa9/source`

The retained runner writes both streams directly to its transcript and enables
`set -x`. It records literal commands, cwd, Go 1.26.6 and host toolchains,
`GOMAXPROCS=4`, source bundle hash, exact HEAD/tree, clean pre/post status,
packages, regexes, options, counts, and an explicit completion sentinel. This
document intentionally does not reconstruct those commands.

Exact source passed format, vet, unit, build, full race, 50 race-instrumented
focused repeats, 100 focused repeats, full unit coverage, and focused coverage.
Unit statement coverage is 97.7%. Every new precedence condition and return is
executed in `coverage.out`; remaining function-level gaps in `Run` and
`runAttempt` are unrelated pre-existing error branches and are retained in
`coverage-gaps.log`.

## Corrected Judge 10 path

The Judge 10 artifact inventory is rooted at:

`/home/linus/.cache/openclaw-code-index/gotth-jobs/389915b`

Its evidence entries have the `artifacts/` prefix. Its source bundle is exactly:

`/home/linus/.cache/openclaw-code-index/gotth-jobs/389915b/source.bundle`

The Judge 12 exact-source runner verifies that path exists with SHA-256
`04a057763025faafc8c4426c343400be5851de25855dcc39815bd140923db709`
and that `389915b/artifacts/source.bundle` does not exist. The prior hash was
correct; only the documented root/path was false.

## Proportional gate scope

No PostgreSQL test or performance matrix was run because the repair changes no
SQL, transaction, scanner, PostgreSQL-backed path, or Store method. The
standalone external-consumer fixture was not rerun because no exported
identifier or signature changed. Existing fuzz targets do not exercise Worker
error precedence, and no parser changed. Graph extraction was not rerun because
production call relationships and SQL did not change. Prior PostgreSQL,
consumer, performance, fuzz, and graph results remain ancestor evidence only
and are not rebound to `27bbaa9`.

## Artifact inventory

Inventory root:

`/home/linus/.cache/openclaw-code-index/gotth-jobs/27bbaa9`

| Artifact | SHA-256 |
| --- | --- |
| `artifacts/artifact-inventory.sha256` | `f6094357be4b5e51a23b8f79c14fa3017605b43c28c42777cc0254fc8e93a27c` |
| `artifacts/coverage-functions.log` | `cd2ce89ae7184259530cbc1bc0451f16a2ed7c83eb8a567e7d3698175833d3ae` |
| `artifacts/coverage-gaps.log` | `8bd2d071f28c6028fc2c1faf4e0c91a11bbb7b5c15e983bbc6f00e9a6457b49a` |
| `artifacts/coverage.out` | `f644f6a59ae093c17f7694a6dd5b8bbe53cfd77d7b2f93e987aa093a81b03a2c` |
| `artifacts/expected-red-tests.patch` | `fd1113084c40eb77a6631eaf48f258011517af4258d24acc919e64e84bd7d999` |
| `artifacts/expected-red.log` | `0e5d4f9566d9af1b8b331e73528108dfff3ae74c07d976c9e2168f60c5911c59` |
| `artifacts/focused-coverage-functions.log` | `f93c0c25204a5a21b07c29b4b199b0a79e738209f3e21a8d39bf50750c9b1634` |
| `artifacts/focused-coverage.out` | `248e6f455aef462c368d3292c399cb1aed3b986c79f0333f98514c216fc9b5e7` |
| `artifacts/run-verification.sh` | `c82346ffa877f6382711e5f269f124a81173dae25d9967c5656d83d3740a6c35` |
| `artifacts/verification-transcript.log` | `631d8c18f94b94c7e27932ef8feeb21ef8c10ea7d590c245e338e947b30547fd` |
| `source.bundle` | `2f2fc2acd0dbf557da8cf663efbed3ff4685e2883dd562d186ebb9f706237236` |

## Remaining gate

Implementation and exact-source evidence gates are complete for this repair.
The final candidate identity and proportional detached clean-clone result are
recorded in `/tmp/gotth-jobs-repair-handoff.md`, because a Git commit cannot
embed its own object ID. Final admission remains active pending two fresh
attributable independent reviews pinned to that final candidate. Those reviews
are orchestrator-owned; this worker neither creates them nor claims a result.
