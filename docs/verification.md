# Verification status

Current repair verification:

- Independent Judge 4 rejected candidate
  `258aeba1bc43ec39cb21ec93bf6698948854e6dc` for an idempotency snapshot race,
  unbounded prevalidation payload copying, and Worker reconciliation-value
  loss. Exact repair source
  `62d565aa1d3f4ebf19cc4d39bf87d2764c676c8b` addresses all three findings.
- Expected-red tests proved the old fallback authenticated A then returned
  replacement B, an oversized payload reached `BeginTx`, and `Worker.Run`
  returned a generic error with no reconciliation Job. All pass after repair.
- The conflict fallback now scans fingerprint and complete job from one row
  and statement snapshot under `FOR KEY SHARE`. PostgreSQL integration observes
  a replacement delete waiting until transaction end, then proves A conflicts
  after B replaces it.
- `Enqueue` and `EnqueueTx` validate before copying. `Enqueue` prepares one
  bounded copy before `BeginTx`; both call one private prepared helper.
  Allocation and transaction/query spies cover oversized input at both public
  boundaries.
- `Worker.Run` returns `ClaimReconciliationError` for a nonzero commit-unknown
  Claim, preserving the original error and exact ID/token without running the
  handler or placing the token in error text.
- Focused local tests, 10 repeats, the full package suite, vet, and
  integration-tag compilation passed under Go 1.26.6 with `GOMAXPROCS=2` and
  `-p=1`.
- An exact detached clean clone on `development` passed format, vet, unit,
  build, full race, 50 affected race repeats, and coverage. Overall statement
  coverage is 96.9%; `Enqueue`, `EnqueueTx`, preparation, all fingerprint row
  scanners, and every new reconciliation error method are 100% covered.
- PostgreSQL 17.10 race and coverage integration passed against the pinned
  image, followed by 10 focused race repeats of the retaining-row
  interleaving. The container was removed and the exact clone remained clean.
- Standalone external-consumer test/build and the complete performance matrix
  passed against exact repair source. Fuzz and graph gates were not invalidated
  and remain ancestor evidence at `72c62231`.
- Exact residual gaps are the existing enqueue entropy-failure seam, terminal
  timestamp row rejection, Worker nil/configuration/attempt propagation, and
  retry-delay failure. No defect-specific statement is uncovered.
- Workflow remains active. Two attributable fresh independent reviews of the
  final candidate remain required and are orchestrator-owned.

Exact commands, artifact hashes, expected-red logs, coverage residuals, and
the remaining review gate are recorded under
`workflow/features/reusable-v0-admission/evidence/`.
