# Implementation specification

## Data model

`gotth_jobs` stores immutable identity/request columns and mutable state,
attempt, lease, failure, and timestamp columns. A non-partial unique index on
`(queue, idempotency_key)` protects key identity for row locking; PostgreSQL's
default distinct-`NULL` behavior permits multiple rows without a key. A
SHA-256 request fingerprint covers queue, kind, payload, maximum attempts, and
requested availability so key reuse with different semantics fails closed.
Availability uses signed Unix seconds plus nanoseconds rather than the
range-limited `time.Time.UnixNano` representation, but public enqueue
validation admits only finite values the pinned PostgreSQL and pgx boundary
can round-trip. Dead-letter cursors use the same timestamp predicate before
reaching pgx.

## Public operations

- `Migrations`: return the immutable migration filesystem.
- `NewPostgreSQL`: validate and retain the minimal database contract.
- `Enqueue` / `EnqueueTx`: validate before copying, copy a bounded payload
  once, fingerprint, insert, or return an exact idempotent duplicate. The
  duplicate fingerprint and job come from one retaining row read. Only
  `Enqueue` owns commit classification.
- `Claim`: reap exhausted expired attempts and atomically claim one eligible
  row with a fresh random token. A produced job returned with
  `ErrCommitOutcomeUnknown` exposes its ID and token for reconciliation only.
- `Heartbeat`: return only an error while extending the exact active,
  unexpired lease; the PostgreSQL update returns one boolean rather than a job.
- `Complete`: transition only the exact active, unexpired lease to succeeded.
- `Fail`: move an exact lease to pending or dead according to explicit,
  bounded failure input and attempt count.
- `Cancel`: idempotently cancel pending/running/canceled jobs and reject other
  terminal transitions.
- `Get`, `Counts`, `ListDead`: bounded observation with payloads rejected by
  source length before one owning copy and a finite PostgreSQL timestamp
  cursor.
- `Redrive`: move exactly one dead job back to pending and reset attempts.
- `RetryPolicy.Delay`: saturating exponential delay without overflow.
- `Worker.Run`: serial claim/handle/heartbeat/acknowledgement loop that cancels
  and joins the per-attempt heartbeat after handler return. A nonzero
  commit-unknown Claim becomes `ClaimReconciliationError`; the handler does not
  run and `ReconciliationJob` is not execution authorization.
- `Permanent`: mark a handler error as non-retryable without changing it for
  `errors.Is`/`errors.As` traversal.

## Failure identities

Exported sentinels are `ErrNotFound`, `ErrNoJob`, `ErrLeaseLost`,
`ErrCanceled`, `ErrStateConflict`, `ErrIdempotencyConflict`,
`ErrCommitOutcomeUnknown`, and `ErrInvalid`. Errors wrap a sentinel and retain
the underlying cause where one exists. `ClaimReconciliationError` is an
exported typed Worker error that unwraps the original commit-unknown Claim
error while keeping its reconciliation job out of `Error()` text.

## Production-unit order

1. value validation, copying, IDs, and retry policy;
2. migration exposure and row scanning;
3. explicit transaction helper and enqueue;
4. claim and fencing transitions;
5. observation, dead-letter listing, cancel, and redrive;
6. worker execution and heartbeat coordination;
7. external consumer and PostgreSQL integration admission.

Each new or materially changed production function receives an adjacent cost
contract naming input variables, database round trips, allocations, and tight
bounds where established.
