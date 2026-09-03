# Changelog

This repository records user-visible and compatibility-relevant changes here.
Released sections use Semantic Versioning; unreleased work remains under
`Unreleased` and does not imply a tag.

## Unreleased

### 2026-09-03 09:25 CDT — Implement the durable PostgreSQL job engine

Commit: current commit; hash assigned by Git after commit

Affected files:

- `go.mod`
- `go.sum`
- `pkg/jobs/**`
- `docs/runtime-boundary.md`
- `docs/performance.md`
- `docs/verification.md`
- `workflow/COVERAGE.md`

Explanation:

Implement the first complete reusable library boundary: immutable PostgreSQL
schema, atomic and idempotent enqueue, bounded nonblocking claim, database-clock
leases, random fencing tokens, heartbeats, completion and failure transitions,
cooperative cancellation, bounded retries, dead-letter pagination and redrive,
queue counts, and a serial worker with panic containment. Real PostgreSQL tests
caught and closed nil-payload encoding, ambiguous SQL, and rolled-back
lease-reaping defects that fake rows could not reveal.

Verification:

- `go test ./pkg/jobs`
- `go vet ./pkg/jobs`
- `go test -race ./pkg/jobs`
- PostgreSQL 17.10 integration and concurrency suite
- two five-second fuzz admissions totaling 147,908 executions
- 96.4% statement coverage with explicit residual gaps
- PostgreSQL performance workload matrix

Risks / non-goals:

- Delivery is at least once; external side effects remain consumer-idempotent.
- No tag, consumer pin, remote push, live database, or deployment changes.
- Final clean-clone, graph, and cold-review admission remain pending.

### 2026-09-03 08:57 CDT — Define the durable job library contract

Commit: `a1835f320f66107545f58a6462a17ee9d97cf95f`

Affected files:

- `LICENSE`
- `README.md`
- `CONTRIBUTING.md`
- `SECURITY.md`
- `Makefile`
- `go.mod`
- `docs/**`
- `workflow.toml`
- `workflow/**`

Explanation:

Replace the placeholder claim with explicit requirements, architecture,
runtime boundaries, implementation units, workflow state, and the maintainer's
MIT licensing decision. The admitted mechanism is PostgreSQL 17 with
at-least-once execution and lease fencing; it does not promise exactly-once
external effects or invent a generic backend framework.

Verification:

- PostgreSQL 17 locking and transaction documentation reviewed
- pgx 5.10.0 transaction source contract reviewed
- documentation and workflow consistency inspection

Risks / non-goals:

- Implementation and verification remain pending.
- No tag, consumer pin, remote push, live database, or deployment changes.

### 2026-09-03 00:42 CDT — Establish GitHub public distribution

Commit: `0244f3cfc76058ebc0de0aade689982bdbefa6f5`

Affected files:

- `README.md`
- `CONTRIBUTING.md`
- `SECURITY.md`
- `docs/distribution.md`
- `docs/RELEASING.md`

Explanation:

Declare GitHub as the public distribution endpoint while retaining Forgejo as
canonical development, define maturity and support honestly, and document the
independent release process. The reserved namespace remains a documentation-only placeholder and makes no API or release claim.

Verification:

- exact old-import search
- documentation contract audit

Risks / non-goals:

- No license is selected.
- No existing tag is changed and no new release is created.
- Mirror direction, repository ownership, and account type are unchanged.
