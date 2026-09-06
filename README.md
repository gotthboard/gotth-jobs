# gotth-jobs

`gotth-jobs` is a PostgreSQL-backed durable background-job library for Go. Its
public package lives at `pkg/jobs`; the module root contains no Go package.

The library owns bounded job envelopes, idempotent enqueue, deterministic
claim order, expiring leases, opaque fencing tokens, heartbeats, cooperative
cancellation, bounded retry scheduling, dead-letter inspection and redrive,
queue counts, and a single-job worker loop.

## Delivery contract

Execution is **at least once**. A worker may perform an external side effect
and lose its lease before acknowledging completion. Fencing protects the job
record; it cannot make an arbitrary external system transactional. Handlers
must therefore use a consumer-owned idempotency mechanism whenever duplicate
effects are unacceptable.

Every running attempt carries a random lease token. Heartbeat, completion, and
failure transitions require the exact active token and reject an expired,
canceled, or replaced lease. Cancellation invalidates the lease and cancels a
cooperative in-process handler on its next heartbeat, but cannot reverse an
effect the handler already performed.

## Boundary

The first durable backend is PostgreSQL 17 using `FOR UPDATE SKIP LOCKED`, which
PostgreSQL documents specifically as suitable for multiple consumers of a
queue-like table. The library provides immutable migration SQL but never
applies it. Consumers own database credentials, migration orchestration,
backups, retention, payload encryption, authorization, job meaning, and the
decision to enqueue work.

Consumers that need atomic domain mutation plus enqueue call `EnqueueTx` on
their existing `pgx.Tx`. The library does not pretend that enqueueing after a
separate domain commit is reliable.

If `Claim` returns a nonzero job with `ErrCommitOutcomeUnknown`, only its ID
and lease token may be used to reconcile the durable row. The job must not be
handled until the committed state and exact token are confirmed.

## Limits

- queue and kind: 128 bytes each;
- worker ID: 256 bytes;
- idempotency key: 256 bytes;
- payload: 1 MiB;
- failure text: 4 KiB;
- attempts: 1 through 100;
- lease: 1 second through 1 hour;
- retry delay: zero through 24 hours;
- scheduled availability and dead-letter cursors: finite PostgreSQL
  `timestamptz` range at microsecond precision;
- dead-letter page: 1 through 100 records.

## Status

Unreleased pre-1.0 implementation. The API may change until a real consumer
pins its first compatibility contract. No tag currently exists.

## Development and distribution

- Canonical development: <https://git.dannyhunn.com/agents/gotth-jobs>
- Public Go import and future releases: <https://github.com/gotthboard/gotth-jobs>

Forgejo remains authoritative and mirrors one way to GitHub. See
[`docs/distribution.md`](docs/distribution.md),
[`docs/RELEASING.md`](docs/RELEASING.md), and [`LICENSE`](LICENSE).
