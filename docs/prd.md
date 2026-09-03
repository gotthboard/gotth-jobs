# Product requirements

## Problem

GOTTH applications need durable background work without copying subtly
different queue tables and retry loops into every product. The dangerous parts
are not starting goroutines; they are atomic claims, stale-worker fencing,
unknown commit outcomes, bounded retry, cancellation, dead-letter recovery,
and truthful delivery guarantees.

## Requirements

- `JOB-001`: Store job envelopes durably in PostgreSQL 17 with explicit size
  limits and no product-specific payload interpretation.
- `JOB-002`: Let a consumer enqueue inside its existing `pgx.Tx` so domain
  mutation and job creation can commit atomically.
- `JOB-003`: Deduplicate an exact idempotent request and reject reuse of the
  same queue/key pair for different request bytes or scheduling policy.
- `JOB-004`: Claim exactly one eligible job in deterministic order without
  blocking concurrent claimers on already locked candidates.
- `JOB-005`: Fence every running attempt with an opaque random token and an
  expiring database-clock lease.
- `JOB-006`: Reject stale, expired, canceled, or replaced leases during
  heartbeat, completion, and failure transitions.
- `JOB-007`: Bound attempts and retry delay; move exhausted or permanent
  failures to a dead state with bounded diagnostic text.
- `JOB-008`: Cancel pending or running work idempotently while documenting
  that cancellation is cooperative and cannot undo external effects.
- `JOB-009`: Expose bounded dead-letter inspection, explicit redrive, exact
  job lookup, and per-queue state counts.
- `JOB-010`: Provide a one-job-at-a-time worker loop with heartbeat,
  cancellation propagation, panic containment, permanent-error
  classification, and bounded polling.
- `JOB-011`: Classify transaction commit failures as an unknown outcome and
  never retry them implicitly.
- `JOB-012`: Keep migration application, credentials, payload encryption,
  retention, authorization, and handler idempotency consumer-owned.
- `JOB-013`: Publish under the MIT license selected by the maintainer.

## Non-goals

- Exactly-once external side effects.
- A cron scheduler, workflow DAG, event bus, or distributed framework.
- Databases other than PostgreSQL 17 in the first compatibility contract.
- Product event definitions, authorization, payload schemas, or disclosure
  policy.
- Automatic schema migration, dead-letter deletion, or retention.

## Acceptance

Every requirement must trace to design, implementation, tests, and evidence.
Verification includes format, vet, race, repeated race, coverage, fuzz,
PostgreSQL 17 boundary and concurrency integration, clean-clone consumer
compilation, performance admission, and two fresh cold Judge passes.
