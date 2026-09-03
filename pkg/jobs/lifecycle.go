package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const claimSQL = `WITH expired_exhausted AS (
    SELECT id
    FROM public.gotth_jobs
    WHERE queue = $1 AND state = 'running'
      AND lease_until <= clock_timestamp() AND attempts >= max_attempts
    ORDER BY lease_until, created_at, id
    FOR UPDATE SKIP LOCKED
    LIMIT 100
), exhausted AS (
    UPDATE public.gotth_jobs AS job
    SET state = 'dead', lease_token = NULL, lease_owner = NULL,
        lease_until = NULL, finished_at = clock_timestamp(),
        updated_at = clock_timestamp(),
        last_error = 'lease expired after final attempt'
    FROM expired_exhausted
    WHERE job.id = expired_exhausted.id
), candidate AS (
    SELECT id
    FROM public.gotth_jobs
    WHERE queue = $1 AND (
        (state = 'pending' AND available_at <= clock_timestamp())
        OR
        (state = 'running' AND lease_until <= clock_timestamp() AND attempts < max_attempts)
    )
    ORDER BY
        CASE WHEN state = 'pending' THEN available_at ELSE lease_until END,
        created_at, id
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE public.gotth_jobs AS job
SET state = 'running', attempts = job.attempts + 1,
    lease_token = $2, lease_owner = $3,
    lease_until = clock_timestamp() + ($4 * interval '1 microsecond'),
    updated_at = clock_timestamp(), finished_at = NULL,
    last_error = CASE WHEN job.state = 'running'
        THEN 'lease expired before acknowledgement' ELSE job.last_error END
FROM candidate
WHERE job.id = candidate.id
RETURNING ` + qualifiedJobColumns

const heartbeatSQL = `UPDATE public.gotth_jobs
SET lease_until = clock_timestamp() + ($3 * interval '1 microsecond'),
    updated_at = clock_timestamp()
WHERE id = $1 AND lease_token = $2 AND state = 'running'
  AND lease_until > clock_timestamp()
RETURNING ` + jobColumns

const completeSQL = `UPDATE public.gotth_jobs
SET state = 'succeeded', lease_token = NULL, lease_owner = NULL,
    lease_until = NULL, finished_at = clock_timestamp(),
    updated_at = clock_timestamp()
WHERE id = $1 AND lease_token = $2 AND state = 'running'
  AND lease_until > clock_timestamp()
RETURNING ` + jobColumns

const failSQL = `UPDATE public.gotth_jobs
SET state = CASE WHEN $4 OR attempts >= max_attempts THEN 'dead' ELSE 'pending' END,
    available_at = CASE WHEN $4 OR attempts >= max_attempts THEN available_at
        ELSE clock_timestamp() + ($5 * interval '1 microsecond') END,
    lease_token = NULL, lease_owner = NULL, lease_until = NULL,
    last_error = $3,
    finished_at = CASE WHEN $4 OR attempts >= max_attempts THEN clock_timestamp() ELSE NULL END,
    updated_at = clock_timestamp()
WHERE id = $1 AND lease_token = $2 AND state = 'running'
  AND lease_until > clock_timestamp()
RETURNING ` + jobColumns

const leaseStateSQL = `SELECT state FROM public.gotth_jobs WHERE id = $1`

type claimResult struct {
	job   Job
	found bool
}

// Claim atomically returns one eligible attempt with a new random fencing
// token. ErrNoJob means no row was eligible at this instant.
//
// Complexity: local time and space are tight Theta(1); database time is an
// indexed exhausted-lease update plus one ordered lock/claim operation whose
// scan cost Q depends on eligible and concurrently locked rows.
func (repository *PostgreSQL) Claim(ctx context.Context, request ClaimRequest) (Job, error) {
	if repository == nil || nilLike(repository.database) || ctx == nil {
		return Job{}, fmt.Errorf("%w: repository and context are required", ErrInvalid)
	}
	if err := validateClaim(request); err != nil {
		return Job{}, err
	}
	token, err := newLeaseToken()
	if err != nil {
		return Job{}, err
	}
	return repository.claimWithToken(ctx, request, token)
}

// claimWithToken is the deterministic claim seam used by tests after the
// production token has already been generated.
//
// Complexity: local time is O(q+w+t), Omega(q+w+t), tight Theta(q+w+t) for
// queue, worker, and token validation; auxiliary space O(1), Omega(1), tight
// Theta(1); database cost is the claimSQL operation.
func (repository *PostgreSQL) claimWithToken(ctx context.Context, request ClaimRequest, token string) (Job, error) {
	if repository == nil || nilLike(repository.database) || ctx == nil {
		return Job{}, fmt.Errorf("%w: repository and context are required", ErrInvalid)
	}
	if err := validateClaim(request); err != nil {
		return Job{}, err
	}
	if len(token) != 64 || !isLowerHex(token) {
		return Job{}, fmt.Errorf("%w: lease token is invalid", ErrInvalid)
	}
	result, err := transact(ctx, repository.database, func(transaction pgx.Tx) (claimResult, error) {
		job, err := scanJob(transaction.QueryRow(ctx, claimSQL,
			request.Queue, token, request.Worker, request.LeaseDuration.Microseconds(),
		))
		if errors.Is(err, pgx.ErrNoRows) {
			return claimResult{}, nil
		}
		if err != nil {
			return claimResult{}, fmt.Errorf("claim job: %w", err)
		}
		return claimResult{job: job, found: true}, nil
	})
	if err != nil {
		return Job{}, err
	}
	if !result.found {
		return Job{}, ErrNoJob
	}
	return result.job, nil
}

// Heartbeat replaces the expiry of the exact active lease using the database
// clock. It never extends from the prior expiry.
//
// Complexity: local validation time O(i+t), Omega(i+t), tight Theta(i+t) for
// ID and token bytes; auxiliary space O(1), Omega(1), tight Theta(1);
// database cost is one indexed update and one indexed state read on rejection.
func (repository *PostgreSQL) Heartbeat(ctx context.Context, lease Lease, duration time.Duration) (Job, error) {
	if err := validateLeaseOperation(repository, ctx, lease); err != nil {
		return Job{}, err
	}
	if duration < MinLeaseDuration || duration > MaxLeaseDuration {
		return Job{}, fmt.Errorf("%w: lease must be between %s and %s", ErrInvalid, MinLeaseDuration, MaxLeaseDuration)
	}
	return repository.leaseMutation(ctx, lease, heartbeatSQL, duration.Microseconds())
}

// Complete acknowledges successful handling only for the exact active lease.
//
// Complexity: local validation time O(i+t), Omega(i+t), tight Theta(i+t);
// auxiliary space O(1), Omega(1), tight Theta(1); database cost is one indexed
// update and one indexed state read on rejection.
func (repository *PostgreSQL) Complete(ctx context.Context, lease Lease) (Job, error) {
	if err := validateLeaseOperation(repository, ctx, lease); err != nil {
		return Job{}, err
	}
	return repository.leaseMutation(ctx, lease, completeSQL)
}

// Fail records a bounded handler failure and either schedules the next attempt
// or moves the job to dead according to the explicit permanence and attempt
// policy.
//
// Complexity: for message bytes m plus ID/token bytes i+t, local validation
// time O(m+i+t), Omega(i+t), tight Theta(m+i+t); auxiliary space O(1),
// Omega(1), tight Theta(1); database cost is one indexed update and one indexed
// state read on rejection.
func (repository *PostgreSQL) Fail(ctx context.Context, lease Lease, failure Failure) (Job, error) {
	if err := validateLeaseOperation(repository, ctx, lease); err != nil {
		return Job{}, err
	}
	if err := validateFailure(failure); err != nil {
		return Job{}, err
	}
	return repository.leaseMutation(ctx, lease, failSQL, failure.Message, failure.Permanent, failure.RetryAfter.Microseconds())
}

// leaseMutation runs one fenced update and classifies a rejected lease without
// committing partial state.
//
// Complexity: local time and space are tight Theta(1); database cost is one
// indexed update, one indexed state read on rejection, and one transaction.
func (repository *PostgreSQL) leaseMutation(ctx context.Context, lease Lease, statement string, arguments ...any) (Job, error) {
	return transact(ctx, repository.database, func(transaction pgx.Tx) (Job, error) {
		queryArguments := []any{lease.JobID, lease.Token}
		queryArguments = append(queryArguments, arguments...)
		job, err := scanJob(transaction.QueryRow(ctx, statement, queryArguments...))
		if err == nil {
			return job, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return Job{}, fmt.Errorf("update job lease: %w", err)
		}
		return Job{}, classifyLease(ctx, transaction, lease.JobID)
	})
}

// classifyLease maps a failed fenced update to a stable public error using the
// current row state in the same transaction.
//
// Complexity: local time and space are tight Theta(1); database cost is one
// primary-key read.
func classifyLease(ctx context.Context, transaction pgx.Tx, id string) error {
	var state string
	if err := transaction.QueryRow(ctx, leaseStateSQL, id).Scan(&state); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("classify rejected job lease: %w", err)
	}
	if State(state) == StateCanceled {
		return ErrCanceled
	}
	return ErrLeaseLost
}

// validateLeaseOperation checks shared repository, context, identifier, and
// token requirements before a database mutation.
//
// Complexity: for ID bytes i and token bytes t, time O(i+t), Omega(1), tight
// Theta(i+t) for valid input; auxiliary space O(1), Omega(1), tight Theta(1).
func validateLeaseOperation(repository *PostgreSQL, ctx context.Context, lease Lease) error {
	if repository == nil || nilLike(repository.database) || ctx == nil {
		return fmt.Errorf("%w: repository and context are required", ErrInvalid)
	}
	if len(lease.JobID) != 32 || !isLowerHex(lease.JobID) || len(lease.Token) != 64 || !isLowerHex(lease.Token) {
		return fmt.Errorf("%w: lease is invalid", ErrInvalid)
	}
	return nil
}
