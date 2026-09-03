# gotth-jobs

Reserved for reusable durable background-job mechanics shared by GOTTH
applications.

## Intended boundary

This project may eventually own job envelopes, atomic claiming, leases,
heartbeats, cancellation, bounded retries, idempotency, dead-letter handling,
and worker observability. Consumers retain job meaning, authorization,
transactional enqueue decisions, payload minimization, and recovery policy.

## Non-goals

- A distributed scheduler selected before workload requirements exist.
- Product notification, media, federation, or webhook policy.
- Claiming exactly-once side effects where the underlying systems cannot
  provide them.

## Status

Placeholder only. There is no implementation, API, release, tag, compatibility
promise, or dependency to pin.
