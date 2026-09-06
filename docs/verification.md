# Verification status

Current Judge 8 repair verification:

- Independent Judge 8 rejected candidate
  `2486b4732076976d4565e235de1d97c36b361951` because scalar lease
  classification allocated untrusted stored state and treated unknown state as
  lease loss, `Counts` omitted malformed states, and the prior evidence did
  not retain exact commands or external-consumer source.
- Exact implementation repair source
  `4ec1970ed632f0306cc772bceeae8e15e17f5ab6`, tree
  `ea2427f5410f3a438ac29dd3df4f8ddef42ac4e1`, scans the classification
  scalar through the bounded borrowed-byte hook, accepts exactly five states,
  and verifies count completeness against the total row count.
- Expected-red unit tests reproduced direct string scanning, ordinary
  lease-loss classification for oversized/unknown state, unsafe NULL handling,
  and incomplete count observations. A real-pgx expected-red run showed about
  1.60-1.68 MiB/op for a rejected 1 MiB state across all five modes.
- Focused local tests, package tests, vet, formatting, integration compilation,
  and `git diff --check` passed with `GOMAXPROCS=2` and `go test -p=1`.
- Exact detached source on `development` passed format, vet, unit, build, full
  race, 50 affected race repeats, two 10-second fuzz targets, and coverage.
  Unit statement coverage is 97.4%; `classifyLease` and `Counts` are 100%.
- PostgreSQL 17.10 full race and coverage passed against the pinned image in a
  disposable, server-verified test database. Integration coverage is 97.4%.
- Three real-pgx runs passed malformed-state and allocation tests under all
  five supported connection defaults. Ten race-instrumented repeats passed.
  Rejected 1 MiB states used 5,963-39,351 bytes/op without race and remained
  below one source-sized allocation with race instrumentation.
- The full performance matrix and a retained standalone external-consumer
  source test/build passed against exact implementation source. The container
  was removed and source pre/post status was clean.
- Literal commands, cwd, toolchain, relevant environment, package patterns,
  regexes, tags, counts, exact HEAD/tree, and cleanliness are retained in the
  hashed `set -x` transcript and its hashed runner. They are not reconstructed
  in this document.
- Workflow remains active. Two fresh attributable independent reviews of the
  final candidate remain orchestrator-owned; this worker claims no admission.

The destructive integration runner still requires the exact opt-in, database
name, and server-side marker documented in the runtime boundary before DDL.
The complete evidence and artifact inventory are in
`workflow/features/reusable-v0-admission/evidence/verification.md`.
