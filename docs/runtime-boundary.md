# Runtime boundary contract

## Supported target

- PostgreSQL 17.x; disposable verification uses an exact PostgreSQL 17 image
  digest recorded in feature evidence.
- The database encoding is UTF8. Library-owned transactions explicitly request
  Read Committed and read-write mode rather than inheriting session defaults.
- Go 1.26.6 and `github.com/jackc/pgx/v5` 5.10.0.
- Timestamps and database-bound durations use PostgreSQL's microsecond
  precision. Finer values are rejected instead of being silently truncated.
- Caller-supplied database timestamps, currently enqueue availability and
  non-nil dead-letter cursors, are finite and inclusive from Go's proleptic
  Gregorian `-4713-11-24T00:00:00Z` through
  `294276-12-31T23:59:59.999999Z`, matching PostgreSQL 17's internal
  `MIN_TIMESTAMP` and exclusive `END_TIMESTAMP` bounds. Values outside that
  range are rejected before pgx binary encoding.

## Authoritative contracts read before design

- PostgreSQL 17 `SELECT`: deterministic `ORDER BY` is required with `LIMIT`;
  `SKIP LOCKED` skips rows that cannot immediately be locked and is suitable
  for multiple consumers of a queue-like table, while not providing a
  consistent general-purpose view.
- PostgreSQL 17 explicit locking: row locks block writers/lockers and remain
  until transaction end; locking may cause disk writes.
- PostgreSQL 17 Read Committed isolation: each command gets a new snapshot and
  update predicates are re-evaluated after concurrent row changes.
- pgx 5.10 transaction contract: context cancellation affects each command,
  not the whole transaction; rollback is safe after commit; a commit error can
  leave outcome uncertain and must not be guessed.
- pgx 5.10 bytea contract: `ByteaCodec` prefers binary format and dispatches a
  `pgtype.BytesScanner` directly over borrowed driver memory. The memory is
  valid only until the next database method call, so the scanner must make its
  own accepted copy before returning.
- pgx 5.10 query-mode contract: Exec and SimpleProtocol use text results;
  leading per-query options override the connection default; DescribeExec
  obtains statement/result OIDs each execution; and result formats by OID are
  applied on that described extended-protocol path.

## Correctness-relevant limits

Application-level byte and count limits are fixed in the README and validated
at every exported boundary. Enqueue rejects an oversized payload before
copying it, and library-owned `Enqueue` rejects it before opening a
transaction. An idempotency fallback selects the fingerprint and complete job
from one row and snapshot under `FOR KEY SHARE`. That lock remains through
transaction end and prevents deletion or key replacement after the row is
authenticated; non-key state updates remain compatible with the lock. The
supporting unique index is non-partial, which makes queue/idempotency-key
changes conflict with key-share while PostgreSQL's distinct-`NULL` uniqueness
allows multiple unkeyed rows. One `Claim` can lock and update up to 100
expired, exhausted running rows and can independently lock and update at most
one disjoint eligible candidate row. Those locks remain until the short claim
transaction ends, so the statement can hold at most 101 row locks and can
perform 100 cleanup writes even when it returns no job. The eligible result
remains limited to one job. The migration uses no extension, dynamic
identifier, server-side function, session setting, or global state.

Successful Heartbeat returns one PostgreSQL boolean and no job columns, so its
response bytes and Go allocation are independent of stored payload size. Job
row scans use the binary `BytesScanner` hook for every text, nullable text,
payload, and fingerprint source. Each scanner enforces the exact schema length
before one bounded ownership conversion; fingerprint length is exactly 32,
and nullable key/token/owner presence is retained so present-empty values are
rejected. Every job-returning query forces per-query
DescribeExec plus binary formats for every job-column OID, overriding all
supported pgx connection defaults at a cost of two protocol round trips. This
also avoids pgx text timestamp parsing limits at PostgreSQL's finite range.
pgx network/read storage is not counted as the library ownership copy. Stored
mandatory timestamps must be
present, and all mandatory or present optional timestamps are finite,
microsecond-precision UTC values before crossing the public boundary.

`EnqueueTx` issues one transaction-local isolation inspection and accepts only
Read Committed. It does not commit, roll back, or retry. Repeatable Read and
Serializable callers receive `ErrInvalid`; any consumer retry policy must
restart the complete domain transaction rather than retrying enqueue alone.

Boundary verification covers every library limit at limit-1, limit, limit+1,
and materially beyond where representable. Timestamp coverage applies the same
predicate to enqueue availability and dead-letter cursors. Integration proves
two concurrent claimers cannot receive the same attempt, stale tokens cannot
acknowledge, expired leases are reclaimed, exhausted leases become dead, and
transactional enqueue rolls back with consumer state. Job IDs, row counts, states, and
attempt numbers provide completeness oracles.

## Failure and cleanup

Warnings or partial results are failures. Explicit transactions use bounded
rollback contexts detached from caller cancellation. Commit failures are
reported as `ErrCommitOutcomeUnknown`; no implicit retry occurs. A nonzero Job
returned by Claim under that error carries only the ID and lease token needed
for reconciliation and must not be handled before durable confirmation.
When the same result reaches `Worker.Run`, it is returned in a
`ClaimReconciliationError`; the original error remains traversable, the
handler does not run, and the error string omits the secret token.
Unknown Heartbeat, Complete, and Fail commit outcomes become
`LeaseReconciliationError`; the original error remains traversable, exact job
and lease values are available only through reconciliation accessors, no
identity or token appears in `Error()`, and Worker does not retry. Immediately
after heartbeat join, this classification outranks both parent cancellation
and cancellation caused by local handler teardown.

Handler return closes the heartbeat stop signal and cancels the per-attempt
context before the worker joins the heartbeat goroutine. Context cancellation
caused by that local teardown is ignored for heartbeat classification; parent
cancellation and independently produced heartbeat failures remain errors. The
heartbeat interval may not exceed half the lease, reserving explicit startup,
scheduler, and database round-trip margin. Arbitrary pauses can still consume
that margin, so the bound is not a hard renewal guarantee. The library changes
no session-scoped setting, so pooled-state restoration tests are N/A.

Integration schema reset is separately guarded from connection selection. It
requires the exact opt-in
`GOTTH_JOBS_ALLOW_DESTRUCTIVE_TEST_DATABASE_RESET=true`, then asks PostgreSQL
to prove `current_database() = 'gotth_jobs_test'` and the database comment is
exactly `gotth-jobs:dedicated-destructive-integration-test-v1` before any DDL.
A URL or environment-variable name is not authorization.
