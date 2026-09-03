# Runtime boundary contract

## Supported target

- PostgreSQL 17.x; disposable verification uses an exact PostgreSQL 17 image
  digest recorded in feature evidence.
- The database encoding is UTF8. Library-owned transactions explicitly request
  Read Committed and read-write mode rather than inheriting session defaults.
- Go 1.26.6 and `github.com/jackc/pgx/v5` 5.10.0.

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

## Correctness-relevant limits

Application-level byte and count limits are fixed in the README and validated
at every exported boundary. PostgreSQL row-lock cardinality has no configured
hard count for this library because claim locks exactly one row. The migration
uses no extension, dynamic identifier, server-side function, session setting,
or global state.

Boundary verification covers every library limit at limit-1, limit, limit+1,
and materially beyond where representable. Integration proves two concurrent
claimers cannot receive the same attempt, stale tokens cannot acknowledge,
expired leases are reclaimed, exhausted leases become dead, and transactional
enqueue rolls back with consumer state. Job IDs, row counts, states, and
attempt numbers provide completeness oracles.

## Failure and cleanup

Warnings or partial results are failures. Explicit transactions use bounded
rollback contexts detached from caller cancellation. Commit failures are
reported as `ErrCommitOutcomeUnknown`; no implicit retry occurs. The library
changes no session-scoped setting, so pooled-state restoration tests are N/A.
