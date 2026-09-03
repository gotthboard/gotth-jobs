# Verification status

Current implementation verification:

- Go 1.26.6 format, vet, unit, race, and coverage gates pass locally.
- Statement coverage is 96.5% under the canonical local compiler and 96.0%
  under the remote integration compiler. Residual branches are injected
  entropy failure, impossible embedded-filesystem failure, and defensive
  invalid/error paths; no claimed state transition lacks direct or PostgreSQL
  integration coverage.
- PostgreSQL 17.10 integration passes against the exact pinned image, including
  UTF8 and version oracles, transaction rollback, concurrent idempotency,
  concurrent claim, lease replacement, stale-token rejection, heartbeat,
  retry, exhaustion, cancellation, dead listing, redrive, counts, future
  scheduling, and the worker loop.
- Fuzz admissions on the admitted source executed 111,741 envelope inputs and
  45,631 diagnostic-text inputs without a product failure.
- The performance matrix and its limitations are in `docs/performance.md`.
- Fifty consecutive race runs pass.
- A clean clone at the admitted source revision passes readonly race, vet, and
  build gates; a separate external module imports and compiles the public API.
- Graphify reports 214 nodes and 491 edges with no dangling endpoints,
  self-loops, exact duplicate edges, or same-endpoint collision groups.
- Two cold reviews failed and were repaired; two subsequent fresh cold reviews
  are recorded as clean in feature evidence.

Exact commands, revisions, hashes, residual coverage, environment differences,
and review findings are recorded under
`workflow/features/reusable-v0-admission/evidence/`.
