# Verification status

Current Judge 9 repair verification:

- Independent Judge 9 rejected candidate
  `8213b6d8a14cd95545d95d2119d1bd5bd231c9e5` because Worker normalized
  and NUL-redacted complete handler error strings before enforcing the 4 KiB
  persistence limit.
- Exact implementation repair source
  `b54c0fcabb5f7f43e3268749f75a59fbfd27413d`, tree
  `1159e048b57ce06eb23bc55d2d987deadaa94f4d`, invokes `Error()` once,
  caps the returned source before normalization, and reserves the ellipsis
  within `MaxFailureBytes`.
- Expected red measured 8,736,880 bytes allocated for a preallocated 1 MiB
  alternating-invalid string and 2,101,280 bytes for a 1 MiB NUL string. The
  large valid control already used a bounded allocation.
- Focused local repeats, the full package, vet, formatting, integration-tag
  compilation, and `git diff --check` passed with `GOMAXPROCS=2` and
  `go test -p=1`.
- Exact detached source on `development` passed format, vet, unit, build, full
  race, 50 focused race repeats, 20 verbose allocation runs, a 100-repeat timed
  workload, the 10-second bounded-failure fuzz target, and coverage.
- Unit statement coverage is 97.5%; `boundedFailure` is 100% covered. The
  retained 20-run samples allocated 4,096 bytes for valid, 29,952 bytes for
  invalid UTF-8, and 12,288-12,320 bytes for NUL input.
- Exact-source Graphify 0.9.32 extraction and diagnostics passed with 316 nodes,
  835 edges, and no malformed or duplicate endpoint findings. The optional SQL
  parser remains unavailable and is not claimed.
- PostgreSQL, external-consumer, and PostgreSQL performance gates were not
  rerun because no SQL, Store contract, public API, or successful Worker path
  changed. Their prior results remain ancestor evidence only.
- Literal commands, cwd, toolchain, package patterns, regexes, options, exact
  HEAD/tree, and clean pre/post status are retained in the hashed runner and
  transcript. An initial runner stopped before all gates because
  `/usr/bin/time` was absent; that failed evidence is retained and superseded
  by the successful Bash-timed runner.
- Workflow remains active. Two fresh attributable independent reviews of the
  final candidate remain orchestrator-owned; this worker claims no admission.

Complete hashes and scope are recorded in
`workflow/features/reusable-v0-admission/evidence/verification.md`.
