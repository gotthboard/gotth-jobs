# Verification status

Current Judge 12 repair verification:

- Independent Judge 12 rejected candidate
  `2ef420fdfa90c2f28c793564093e74d6c3e6158b` because Worker allowed joined
  `ErrNoJob`/`ErrCanceled` identities to suppress `ErrCommitOutcomeUnknown`,
  and because Judge 10 evidence stated the wrong root for `source.bundle`.
- Exact implementation repair source
  `27bbaa962e1d7d65e2395a5bb212e92ea6e6d667`, tree
  `7c057626752d2d5a41a6f5067dc070830ff5162c`, classifies unknown outcome
  before routine sentinels for Claim, Complete, and Fail.
- Expected red proved both Claim variants polled into a second Store call and
  both acknowledgement variants returned nil then continued to a second Claim.
- Focused local repeats, the full package, vet, formatting, integration-tag
  compilation, and `git diff --check` passed with `GOMAXPROCS=2` and
  `go test -p=1`.
- Exact detached source on `development` passed format, vet, unit, build, full
  race, 50 focused race repeats, 100 focused repeats, full coverage, and
  focused coverage.
- Unit statement coverage is 97.7%; every changed precedence branch is
  covered. Remaining `Run` and `runAttempt` gaps are unrelated existing paths.
- The exact-source runner verified Judge 10's bundle at
  `/home/linus/.cache/openclaw-code-index/gotth-jobs/389915b/source.bundle`
  and verified that no `389915b/artifacts/source.bundle` exists.
- PostgreSQL, external-consumer, fuzz, graph, and performance gates were not
  invalidated and remain ancestor evidence only.
- Literal commands, cwd, toolchain, `GOMAXPROCS`, package patterns, regexes,
  options, exact HEAD/tree, bundle hashes, and clean pre/post status are
  retained in the hashed runner and transcript.
- Workflow remains active. Two fresh attributable independent reviews of the
  final candidate remain orchestrator-owned; this worker claims no admission.

Complete hashes and scope are recorded in
`workflow/features/reusable-v0-admission/evidence/verification.md`.
