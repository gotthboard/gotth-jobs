# gotth-jobs

`gotth-jobs` is a PostgreSQL-backed durable background-job library for Go. Its
public package lives at `pkg/jobs`; the module root contains no Go package.

> **Distribution:** GitHub is the public clone and future release endpoint.
> Forgejo remains canonical development. Report public bugs through GitHub
> Issues and security vulnerabilities through GitHub private reporting.
> See [the distribution contract](docs/distribution.md).

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

Heartbeat returns only an error and PostgreSQL returns one fixed-size success
scalar; no job envelope or payload is sent back on a successful lease renewal.
Worker heartbeat intervals must be positive and no greater than half the
lease. That reserves scheduler and database round-trip margin before expiry;
it cannot guarantee renewal across arbitrary process, host, or database
pauses.

Worker calls a handler error's `Error()` method once. It caps that returned
source to a 4 KiB-scale prefix before UTF-8 normalization or NUL redaction,
reserves room for an ellipsis when truncating, and sends at most
`MaxFailureBytes` of valid, NUL-free UTF-8 to `Fail`. Work performed inside a
custom `Error()` method is consumer-owned; Worker work after it returns is
independent of the full error length.

Every handler panic unwind, including `panic(nil)` under Go's legacy
`panicnil=1` mode, is acknowledged through `Fail` as a bounded retryable
failure and is never completed as success. Jobs returned by a custom `Store`
are validated before handler execution; an unknown state produces a constant
classified error without copying that untrusted state into diagnostics.

## Boundary

The first durable backend is PostgreSQL 17 using `FOR UPDATE SKIP LOCKED`, which
PostgreSQL documents specifically as suitable for multiple consumers of a
queue-like table. The library provides immutable migration SQL but never
applies it. Consumers own database credentials, migration orchestration,
backups, retention, payload encryption, authorization, job meaning, and the
decision to enqueue work.

Consumers that need atomic domain mutation plus enqueue call `EnqueueTx` on
their existing Read Committed `pgx.Tx`. `EnqueueTx` inspects that isolation
before insertion and rejects Repeatable Read or Serializable transactions. It
never commits, rolls back, or retries the caller transaction; consumers that
retry must retry their whole domain transaction. The library does not pretend
that enqueueing after a separate domain commit is reliable.

An idempotency conflict reads the stored fingerprint and complete job from one
row in one statement snapshot. A key-share lock retains that row identity
through transaction end, so one request cannot authenticate an old row and
return a replacement. The supporting unique index is non-partial so PostgreSQL
treats queue and idempotency key changes as key updates; ordinary PostgreSQL
`NULL` uniqueness still allows any number of jobs without a key.

If `Claim` returns a nonzero job with `ErrCommitOutcomeUnknown`, only its ID
and lease token may be used to reconcile the durable row. The job must not be
handled until the committed state and exact token are confirmed.
`Worker.Run` exposes the same value through `ClaimReconciliationError`, found
with `errors.As`; `ReconciliationJob` is likewise reconciliation-only and the
error text never includes the lease token. Worker classifies an unknown Claim
outcome before `ErrNoJob`, including when both identities are joined; without
a nonzero job it returns the unknown error and does not poll.

Unknown Heartbeat, Complete, or Fail commit outcomes are exposed through
`LeaseReconciliationError`. `ReconciliationJob` and `ReconciliationLease`
identify the affected acknowledgement for durable inspection only. The type
unwraps the original error, omits the job ID and lease token from `Error()`,
and does not authorize an implicit acknowledgement retry. After joining the
heartbeat, Worker gives an unknown commit outcome precedence over parent
cancellation and handler-teardown cancellation so reconciliation is never
silently replaced by a less specific cancellation result. Complete and Fail
apply the same precedence before `ErrCanceled`, including joined errors.

Every query that returns a job overrides the connection default with pgx
`DescribeExec` and requests binary results for every job-column OID. This
preserves the bounded scanner and full timestamp-range contracts even when a
pool defaults to Exec or SimpleProtocol, at the disclosed cost of two protocol
round trips for each such statement. Stored rows with SQL NULL payloads or
missing/invalid timestamps are rejected. Every stored text field and request
fingerprint is length-checked through pgx's borrowed-byte scanner hook before
one bounded ownership conversion; nullable key and lease fields preserve SQL
NULL presence, and present-empty values are rejected. Lease classification
applies the same pre-allocation bound to the stored state scalar and accepts
only the five documented states. `Counts` cross-checks those five buckets
against the total row count, so unknown or NULL states fail as stored-data
corruption instead of being omitted.

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

- Canonical development: <https://git.dannyhunn.com/gotthboard/gotth-jobs>
- Public Go import and future releases: <https://github.com/gotthboard/gotth-jobs>

Forgejo remains authoritative and mirrors one way to GitHub. See
[`docs/distribution.md`](docs/distribution.md),
[`docs/RELEASING.md`](docs/RELEASING.md), and [`LICENSE`](LICENSE).

PostgreSQL integration tests are destructive and fail closed. `make
verify-integration` requires
`GOTTH_JOBS_ALLOW_DESTRUCTIVE_TEST_DATABASE_RESET=true`; PostgreSQL itself
must report the exact database name `gotth_jobs_test` and that database must
carry the comment `gotth-jobs:dedicated-destructive-integration-test-v1`.
The URL variable name alone is never treated as authorization.
