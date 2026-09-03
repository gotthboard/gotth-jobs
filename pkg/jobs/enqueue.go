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
ON CONFLICT (queue, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
RETURNING ` + jobColumns

const fingerprintSQL = `SELECT request_fingerprint FROM public.gotth_jobs
WHERE queue = $1 AND idempotency_key = $2`

const idempotentJobSQL = `SELECT ` + jobColumns + ` FROM public.gotth_jobs
WHERE queue = $1 AND idempotency_key = $2`

type enqueueResult struct {
	job     Job
	created bool
}

// Enqueue inserts a durable job in its own transaction. An exact idempotent
// duplicate returns the existing job with created=false. Commit failure wraps
// ErrCommitOutcomeUnknown and is never retried.
//
// Complexity: for q queue, k kind, p payload, and i key bytes, local time and
// auxiliary space are Theta(q+k+p+i); database cost is one transaction with
// one insert, plus two indexed reads only on an idempotency conflict.
func (repository *PostgreSQL) Enqueue(ctx context.Context, request EnqueueRequest) (Job, bool, error) {
	if repository == nil || repository.database == nil {
		return Job{}, false, fmt.Errorf("%w: repository is required", ErrInvalid)
	}
	result, err := transact(ctx, repository.database, func(transaction pgx.Tx) (enqueueResult, error) {
		job, created, enqueueErr := repository.EnqueueTx(ctx, transaction, request)
		return enqueueResult{job: job, created: created}, enqueueErr
	})
	return result.job, result.created, err
}

// EnqueueTx inserts a job using the caller's existing transaction and never
// commits or rolls it back. The caller owns commit-outcome reconciliation.
//
// Complexity: for q queue, k kind, p payload, and i key bytes, local time and
// auxiliary space are Theta(q+k+p+i); database cost is one insert, plus two
// indexed reads only on an idempotency conflict.
func (repository *PostgreSQL) EnqueueTx(ctx context.Context, transaction pgx.Tx, request EnqueueRequest) (Job, bool, error) {
	if repository == nil || repository.database == nil || ctx == nil || nilLike(transaction) {
		return Job{}, false, fmt.Errorf("%w: repository, context, and transaction are required", ErrInvalid)
	}
	request = cloneEnqueue(request)
	if err := validateEnqueue(request); err != nil {
		return Job{}, false, err
	}
	id, err := newJobID()
	if err != nil {
		return Job{}, false, err
	}
	fingerprint := requestFingerprint(request)
	job, err := scanJob(transaction.QueryRow(ctx, enqueueSQL,
		id, request.Queue, request.Kind, request.Payload, request.IdempotencyKey,
		fingerprint[:], request.MaxAttempts, availableArgument(request.AvailableAt),
	))
	if err == nil {
		return job, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, fmt.Errorf("insert job: %w", err)
	}
	if request.IdempotencyKey == "" {
		return Job{}, false, fmt.Errorf("insert job returned no row without an idempotency key")
	}

	var stored []byte
	if err := transaction.QueryRow(ctx, fingerprintSQL, request.Queue, request.IdempotencyKey).Scan(&stored); err != nil {
		return Job{}, false, fmt.Errorf("read idempotent job fingerprint: %w", err)
	}
	if len(stored) != len(fingerprint) || subtle.ConstantTimeCompare(stored, fingerprint[:]) != 1 {
		return Job{}, false, ErrIdempotencyConflict
	}
	job, err = scanJob(transaction.QueryRow(ctx, idempotentJobSQL, request.Queue, request.IdempotencyKey))
	if err != nil {
		return Job{}, false, fmt.Errorf("read idempotent job: %w", err)
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
