package jobs

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const enqueueSQL = `INSERT INTO public.gotth_jobs (
id, queue, kind, payload, idempotency_key, request_fingerprint,
max_attempts, available_at
) VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7,
COALESCE($8::timestamptz, clock_timestamp()))
ON CONFLICT (queue, idempotency_key) DO NOTHING
RETURNING ` + jobColumns

const idempotentJobSQL = `SELECT request_fingerprint, ` + jobColumns + ` FROM public.gotth_jobs
WHERE queue = $1 AND idempotency_key = $2
FOR KEY SHARE`

const transactionIsolationSQL = `SELECT current_setting('transaction_isolation')`

type enqueueResult struct {
	job     Job
	created bool
}

// Enqueue inserts a durable job in its own transaction. An exact idempotent
// duplicate returns the existing job with created=false. Commit failure wraps
// ErrCommitOutcomeUnknown and is never retried.
//
// Complexity: for q queue, k kind, p payload, and i key bytes, local time is
// Theta(q+k+p+i) and auxiliary space is Theta(p) for valid requests; rejected
// oversized payloads use O(q+k+i) time and Theta(1) auxiliary space. Database
// cost is one transaction with one insert, plus one indexed retaining read
// only on an idempotency conflict, with two DescribeExec protocol round trips
// per job-returning statement.
func (repository *PostgreSQL) Enqueue(ctx context.Context, request EnqueueRequest) (Job, bool, error) {
	if repository == nil || repository.database == nil {
		return Job{}, false, fmt.Errorf("%w: repository is required", ErrInvalid)
	}
	if ctx == nil {
		return Job{}, false, fmt.Errorf("%w: context is required", ErrInvalid)
	}
	prepared, err := prepareEnqueue(request)
	if err != nil {
		return Job{}, false, err
	}
	result, err := transact(ctx, repository.database, func(transaction pgx.Tx) (enqueueResult, error) {
		job, created, enqueueErr := repository.enqueuePrepared(ctx, transaction, prepared)
		return enqueueResult{job: job, created: created}, enqueueErr
	})
	return result.job, result.created, err
}

// EnqueueTx inserts a job using the caller's existing transaction and never
// commits or rolls it back. The transaction must use Read Committed; the
// caller owns commit-outcome reconciliation.
//
// Complexity: for q queue, k kind, p payload, and i key bytes, local time is
// Theta(q+k+p+i) and auxiliary space is Theta(p) for valid requests; rejected
// oversized payloads use O(q+k+i) time and Theta(1) auxiliary space. Database
// cost is one isolation read and one insert, plus one indexed retaining read
// only on an idempotency conflict. Each job-returning statement uses two
// DescribeExec protocol round trips.
func (repository *PostgreSQL) EnqueueTx(ctx context.Context, transaction pgx.Tx, request EnqueueRequest) (Job, bool, error) {
	if repository == nil || repository.database == nil || ctx == nil || nilLike(transaction) {
		return Job{}, false, fmt.Errorf("%w: repository, context, and transaction are required", ErrInvalid)
	}
	if err := validateEnqueue(request); err != nil {
		return Job{}, false, err
	}
	if err := validateEnqueueTxIsolation(ctx, transaction); err != nil {
		return Job{}, false, err
	}
	return repository.enqueuePrepared(ctx, transaction, cloneEnqueue(request))
}

// validateEnqueueTxIsolation enforces the new-snapshot behavior required by
// the idempotency fallback without committing, rolling back, or retrying the
// caller's domain transaction.
//
// Complexity: local time and auxiliary space are tight Theta(1); database
// cost is one transaction-local setting read.
func validateEnqueueTxIsolation(ctx context.Context, transaction pgx.Tx) error {
	var isolation string
	if err := transaction.QueryRow(ctx, transactionIsolationSQL).Scan(&isolation); err != nil {
		return fmt.Errorf("inspect enqueue transaction isolation: %w", err)
	}
	if isolation != string(pgx.ReadCommitted) {
		return fmt.Errorf("%w: EnqueueTx requires Read Committed isolation, got %q", ErrInvalid, isolation)
	}
	return nil
}

// enqueuePrepared executes the transaction-bound statements for a request
// already validated and copied by the public entry point.
//
// Complexity: for q queue, k kind, and p payload bytes, local time is
// Theta(q+k+p) for fingerprinting and scanning while auxiliary space is
// Theta(p); database cost is one insert plus one indexed retaining read only
// on an idempotency conflict, with two DescribeExec protocol round trips per
// job-returning statement.
func (repository *PostgreSQL) enqueuePrepared(ctx context.Context, transaction pgx.Tx, request EnqueueRequest) (Job, bool, error) {
	id, err := newJobID()
	if err != nil {
		return Job{}, false, err
	}
	fingerprint := requestFingerprint(request)
	job, err := scanJob(transaction.QueryRow(ctx, enqueueSQL, jobQueryArguments(
		id, request.Queue, request.Kind, request.Payload, request.IdempotencyKey,
		fingerprint[:], request.MaxAttempts, availableArgument(request.AvailableAt),
	)...))
	if err == nil {
		return job, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, fmt.Errorf("insert job: %w", err)
	}
	if request.IdempotencyKey == "" {
		return Job{}, false, fmt.Errorf("insert job returned no row without an idempotency key")
	}

	stored, job, err := scanJobWithFingerprint(transaction.QueryRow(ctx, idempotentJobSQL, jobQueryArguments(request.Queue, request.IdempotencyKey)...))
	if err != nil {
		return Job{}, false, fmt.Errorf("read idempotent job: %w", err)
	}
	if len(stored) != len(fingerprint) || subtle.ConstantTimeCompare(stored, fingerprint[:]) != 1 {
		return Job{}, false, ErrIdempotencyConflict
	}
	return job, false, nil
}

// availableArgument represents an optional UTC timestamp as a nil or concrete
// query argument.
//
// Complexity: time O(1), Omega(1), tight Theta(1); auxiliary space O(1),
// Omega(1), tight Theta(1).
func availableArgument(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
