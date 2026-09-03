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
admitted, including a claim after lease expiry.

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
Commit errors wrap `ErrCommitOutcomeUnknown`; callers reconcile by reading the
known job ID and token before retrying. No unknown outcome is retried by the
library.

Complete, fail, and heartbeat use the exact `(job_id, lease_token)` pair and
require a non-expired running lease. An old worker cannot mutate the record
after another attempt receives a new token. External systems do not see this
token unless the consumer deliberately carries it into a system with its own
fencing contract.

## Worker

`Worker.Run` executes one job at a time. A heartbeat goroutine is bounded to
the active handler. Losing or canceling the lease cancels the handler context.
Handler panic becomes a bounded retryable failure instead of terminating the
worker process. Shutdown stops heartbeats and leaves the lease to expire; it
does not falsely record a handler failure.

## Trust boundary

Job rows, error text, and payloads read from PostgreSQL are untrusted. The
consumer authenticates callers and decides who may enqueue, inspect, cancel,
or redrive. The library validates every public input and copies payload bytes
at the API boundary. It never logs payloads, idempotency keys, or errors.
