# Architecture

## Package layout

The sole public package is `pkg/jobs`. Repository governance, workflow, and
module metadata remain outside it. Immutable schema migrations live under
`pkg/jobs/migrations` and are exposed as an `fs.FS`; the library never applies
them.

## State machine

```text
pending --claim--> running --complete--> succeeded
   |                 |  |
 cancel              |  +--fail/exhaust--> dead --redrive--> pending
   v                 |
canceled <-----------+ cancel
                     +--retry------------> pending
```

`pending`, `running`, `succeeded`, `dead`, and `canceled` are the only states.
The database constrains lease columns to exist only in `running` and terminal
timestamps only in terminal states. Attempts increment when a claim is
admitted, including a claim after lease expiry. Pending rows have fewer than
`max_attempts`; running, succeeded, and dead rows have at least one attempt;
canceled rows may have zero or more attempts through `max_attempts`.

## Enqueue mechanism

Public enqueue entry points validate all caller-controlled bounds before
copying payload memory. `Enqueue` completes that preparation before opening
its library-owned transaction; both entry points then use one private prepared
statement path, so a valid payload is not copied twice. On an idempotency
conflict, one `SELECT` reads the fingerprint and complete job from the same row
and Read Committed statement snapshot. `FOR KEY SHARE` retains that row's key
identity until transaction end, preventing delete-and-replacement between
authentication and return without blocking ordinary state-only updates. A
non-partial unique index on `(queue, idempotency_key)` makes those columns key
columns for row locking; PostgreSQL's default distinct-`NULL` uniqueness still
permits multiple jobs without an idempotency key.

The conflict fallback depends on a fresh statement snapshot. Library-owned
transactions request Read Committed, and `EnqueueTx` verifies the caller's
transaction isolation with `current_setting('transaction_isolation')` before
insertion. Repeatable Read and Serializable are rejected. `EnqueueTx` never
retries a statement or transaction; a consumer choosing transaction retries
must retry the entire domain transaction itself.

## Claim mechanism

A short Read Committed transaction first marks expired attempts dead when they
have exhausted `max_attempts`. It then selects one eligible row ordered by
`available_at`, `created_at`, and `id` using `FOR UPDATE SKIP LOCKED`, and
updates that row with a new token, owner, attempt count, and database-clock
lease. PostgreSQL explicitly describes `SKIP LOCKED` as an inconsistent view
suited to multiple consumers of a queue-like table. That inconsistency is the
mechanism, not a general read contract.

## Fencing and outcomes

Mutation methods use explicit transactions. Query-stage errors roll back.
Commit errors wrap `ErrCommitOutcomeUnknown`; Claim preserves a produced job's
ID and lease token in its value return so callers can reconcile the durable
row. That value is reconciliation-only while the error is non-nil and does not
authorize handling. No unknown outcome is retried by the library.

Complete, fail, and heartbeat use the exact `(job_id, lease_token)` pair and
require a non-expired running lease. An old worker cannot mutate the record
after another attempt receives a new token. External systems do not see this
token unless the consumer deliberately carries it into a system with its own
fencing contract. Heartbeat returns only an error and its update returns one
boolean from PostgreSQL, so renewal response traffic and allocation do not
scale with payload size. Complete and fail continue to return the transitioned
job.

If Worker receives `ErrCommitOutcomeUnknown` from Heartbeat, Complete, or
Fail, it returns `LeaseReconciliationError`. The typed error unwraps the
original failure and exposes the exact affected lease plus the known or
returned job only for reconciliation. Its text omits job identity and the
secret token, and Worker never retries the acknowledgement implicitly.

## Worker

`Worker.Run` executes one job at a time. A heartbeat goroutine is bounded to
the active handler. After the handler returns, the worker signals heartbeat
stop and cancels its attempt context before joining; context cancellation from
that local teardown does not suppress acknowledgement. Parent cancellation and
genuine heartbeat failures remain visible. Losing or canceling the lease
cancels the handler context. Handler panic becomes a bounded retryable failure
instead of terminating the worker process. Shutdown stops heartbeats and
leaves the lease to expire; it does not falsely record a handler failure.

If Claim returns a nonzero job with `ErrCommitOutcomeUnknown`, `Worker.Run`
returns `ClaimReconciliationError` without invoking the handler. The typed
error unwraps the original failure and exposes the unconfirmed job only for
durable ID/token reconciliation; its text omits every job field.

Worker accepts a heartbeat interval no greater than half the lease duration.
The remaining half is an explicit budget for post-Claim startup, scheduling,
and heartbeat database round trips. This reduces deterministic first-renewal
expiry risk but is not a hard liveness guarantee across arbitrary runtime,
host, network, or database pauses.

## Trust boundary

Job rows, error text, and payloads read from PostgreSQL are untrusted. The
consumer authenticates callers and decides who may enqueue, inspect, cancel,
or redrive. The library validates every public input and copies payload bytes
at the API boundary. Every job-returning query forces pgx `DescribeExec` and
binary formats for every job-column OID, overriding connection defaults
including Exec and SimpleProtocol. `DescribeExec` obtains result OIDs and costs two
protocol round trips per statement. The binary `BytesScanner` checks pgx's
borrowed source length before payload allocation and makes exactly one copy
into library-owned memory; pgx's network/read buffers remain separate runtime
storage. SQL NULL payloads are distinct from empty bytea and are rejected.
Mandatory stored timestamps must be present; every mandatory or present
optional timestamp is normalized after pgx decoding and validated as finite,
microsecond-precision UTC before exposure. Custom Store jobs must already use
that UTC contract. The library never logs payloads, idempotency keys, or
errors.
