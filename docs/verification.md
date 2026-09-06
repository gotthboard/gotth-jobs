# Verification status

Current repair verification:

- Judge 3 rejected candidate
  `b195469add429001a35b3c9658ecfd52e07c08e3` for Claim reconciliation-value
  loss and heartbeat teardown liveness. Exact repair source
  `6655331ae4e3b7509b826a03db11c36cee9a6ca2` addresses both findings.
- Expected-red tests proved Claim zeroed its produced ID/token on commit error
  and `runAttempt` hung after handler return while Heartbeat waited on
  `ctx.Done()`. Both focused regressions pass after repair.
- The transaction-wrapper audit found no analogous value loss: Enqueue unwraps
  and returns its result fields, while Cancel, Redrive, and lease mutations
  return `transact`'s value/error pair directly. Enqueue's existing behavior is
  now asserted under commit failure.
- Focused local tests, the full package suite, and vet passed under Go 1.26.6
  with `GOMAXPROCS=2` and `-p=1`.
- An exact clean clone on `development` passed format, vet, unit, build, full
  race, 50 focused race repeats, and coverage. Unit statement coverage is
  96.3%; every changed production statement has a nonzero count.
- PostgreSQL 17.10 race and coverage integration passed against the pinned
  image. Integration statement coverage is 96.5%; the container was removed
  and the exact clone remained clean.
- `claimWithToken` is 88.9% and `runAttempt` is 97.4% covered. Their uncovered
  blocks are preexisting invalid-input/public entropy and retry-delay-error
  paths, not either repaired behavior. There is no repair-specific gap.
- External-consumer evidence remains at signature-compatible source `9f6acc74`;
  performance, fuzz, and graph results remain ancestor evidence at `72c62231`.
  They were not rerun or represented as current-source results.
- Workflow remains active. Two attributable fresh independent reviews of the
  final candidate remain required and are orchestrator-owned.

Exact commands, artifact hashes, expected-red logs, coverage residuals, and
the remaining review gate are recorded under
`workflow/features/reusable-v0-admission/evidence/`.
