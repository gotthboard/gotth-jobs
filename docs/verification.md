# Verification status

Current repair verification:

- Independent Judge 5 rejected candidate
  `6984227d9e1bc9247a6bc783627dfeabd5c2dc42` for payload-returning
  heartbeats, a partial idempotency index that did not retain key-changing
  updates, and payload allocation before stored-row size validation.
- Exact repair source `53cf140090cb7c1bc2076579437aab8edd3a0229`
  changes Heartbeat to error-only with one returned PostgreSQL boolean, makes
  the unique queue/key index non-partial, and scans bytea through a bounded
  owning `pgtype.BytesScanner`.
- Expected-red tests observed a scalar heartbeat scan-width failure, about
  2.10 MiB allocated for a valid 1 MiB payload, about 2.11 MiB allocated before
  oversized rejection, and the partial migration shape. All pass after repair.
- Focused local tests, 10 repeats, the complete package test, vet, and
  integration-tag compilation passed with `GOMAXPROCS=2` and `-p=1`.
- Exact detached clean source on `development` passed format, vet, unit,
  build, full race, 50 affected race repeats, two 10-second fuzz targets, and
  coverage. Overall statement coverage is 97.0%; Heartbeat, `ScanBytes`, and
  all shared job scanners are 100% covered.
- PostgreSQL 17.10 race and coverage integration passed at the pinned image
  digest. Ten race-instrumented UPDATE-key interleavings observed the update
  blocked until the key-share transaction ended. Three real-pgx allocation
  repeats proved Heartbeat allocation does not scale with a 1 MiB payload.
  Two separately enqueued `NULL` keys remained distinct and valid.
- Standalone external-consumer test/build passed against the new error-only
  Heartbeat signature. The complete performance matrix passed.
- Exact residual coverage gaps are preexisting entropy/error and unrelated
  operation/Worker branches. No changed production function or defect path is
  uncovered.
- Workflow remains active. Two fresh attributable independent reviews of the
  final candidate remain orchestrator-owned; this worker claims no admission.

Exact commands, artifact hashes, setup notes, and the remaining review gate
are recorded under
`workflow/features/reusable-v0-admission/evidence/verification.md`.
