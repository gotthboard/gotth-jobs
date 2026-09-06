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
  duplicate fingerprint and job come from one retaining row read. `EnqueueTx`
  first verifies Read Committed isolation and never retries the caller's
  transaction. Only `Enqueue` owns commit classification.
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
`LeaseReconciliationError` performs the same secret-safe traversal for
commit-unknown Heartbeat, Complete, and Fail results and exposes the affected
job and lease only through reconciliation accessors. Worker never retries
those acknowledgements implicitly.

## PostgreSQL decoding

Every statement returning job columns passes `QueryExecModeDescribeExec` and
binary result formats for the text, bytea, integer, and timestamptz OIDs
through the actual pgx query call. This overrides every supported connection
default, ensures pgx knows result OIDs, prevents bytea's text decoder from
allocating a decoded payload before the bounded scanner runs, and preserves
binary decoding across PostgreSQL's full finite timestamp range. The tradeoff is two protocol round
trips per job-returning statement. The scanner rejects SQL NULL and oversized
payloads before payload allocation and makes one ownership copy of accepted
borrowed binary bytes. Row validation requires non-NULL mandatory timestamps
and checks every mandatory or present optional timestamp for UTC, finite
PostgreSQL range, and microsecond precision after pgx timestamps are normalized
to UTC.

## Worker renewal budget

`HeartbeatInterval` must be positive and at most half `LeaseDuration`. The
unused half budgets Claim return time, goroutine scheduling, and the renewal
round trip; arbitrary pauses can still exceed the lease, so this is not a hard
liveness guarantee.

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
