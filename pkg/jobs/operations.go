package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const getSQL = `SELECT ` + jobColumns + ` FROM public.gotth_jobs WHERE id = $1`

const countsSQL = `SELECT
    count(*) FILTER (WHERE state = 'pending'),
    count(*) FILTER (WHERE state = 'running'),
    count(*) FILTER (WHERE state = 'succeeded'),
    count(*) FILTER (WHERE state = 'dead'),
    count(*) FILTER (WHERE state = 'canceled')
FROM public.gotth_jobs WHERE queue = $1`

const listDeadSQL = `SELECT ` + jobColumns + ` FROM public.gotth_jobs
WHERE queue = $1 AND state = 'dead'
  AND ($2::timestamptz IS NULL OR (finished_at, id) > ($2, $3))
ORDER BY finished_at, id
LIMIT $4`

const cancelSQL = `UPDATE public.gotth_jobs
SET state = 'canceled', lease_token = NULL, lease_owner = NULL,
    lease_until = NULL, finished_at = clock_timestamp(),
    updated_at = clock_timestamp()
WHERE id = $1 AND state IN ('pending', 'running')
RETURNING ` + jobColumns

const redriveSQL = `UPDATE public.gotth_jobs
SET state = 'pending', attempts = 0, available_at = clock_timestamp(),
    lease_token = NULL, lease_owner = NULL, lease_until = NULL,
    last_error = '', finished_at = NULL, updated_at = clock_timestamp()
WHERE id = $1 AND state = 'dead'
RETURNING ` + jobColumns

// Get returns one copied job by opaque ID without mutating queue state.
//
// Complexity: for payload bytes p and ID bytes i, local time O(p+i), Omega(i),
// tight Theta(p+i); auxiliary space O(p), Omega(p), tight Theta(p); database
// cost is one primary-key read.
func (repository *PostgreSQL) Get(ctx context.Context, id string) (Job, error) {
	if err := validateRead(repository, ctx); err != nil {
		return Job{}, err
	}
	if err := validateJobID(id); err != nil {
		return Job{}, err
	}
	job, err := scanJob(repository.database.QueryRow(ctx, getSQL, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("get job: %w", err)
	}
	return job, nil
}

// Counts returns exact per-state counts for one queue.
//
// Complexity: for queue bytes q, local time O(q), Omega(q), tight Theta(q)
// and auxiliary space O(1), Omega(1), tight Theta(1); database cost is one
// aggregate scan over rows selected by queue.
func (repository *PostgreSQL) Counts(ctx context.Context, queue string) (Counts, error) {
	if err := validateRead(repository, ctx); err != nil {
		return Counts{}, err
	}
	if err := validateName("queue", queue, MaxQueueBytes); err != nil {
		return Counts{}, err
	}
	var counts Counts
	if err := repository.database.QueryRow(ctx, countsSQL, queue).Scan(
		&counts.Pending, &counts.Running, &counts.Succeeded, &counts.Dead,
		&counts.Canceled,
	); err != nil {
		return Counts{}, fmt.Errorf("count jobs: %w", err)
	}
	return counts, nil
}

// ListDead returns at most limit dead jobs after an optional stable
// `(finished_at,id)` cursor.
//
// Complexity: for n returned jobs with total payload bytes p and queue/ID
// validation bytes v, local time O(n+p+v), Omega(v), tight Theta(n+p+v);
// auxiliary space O(n+p), Omega(n), tight Theta(n+p); database cost is one
// indexed ordered scan bounded by limit.
func (repository *PostgreSQL) ListDead(ctx context.Context, queue string, cursor *DeadCursor, limit int) ([]Job, error) {
	if err := validateRead(repository, ctx); err != nil {
		return nil, err
	}
	if err := validateName("queue", queue, MaxQueueBytes); err != nil {
		return nil, err
	}
	if limit < 1 || limit > MaxDeadPage {
		return nil, fmt.Errorf("%w: dead-letter limit must be between 1 and %d", ErrInvalid, MaxDeadPage)
	}
	var cursorTime any
	var cursorID string
	if cursor != nil {
		if cursor.FinishedAt.IsZero() || cursor.FinishedAt.Location() != time.UTC || cursor.FinishedAt.Nanosecond()%int(time.Microsecond) != 0 {
			return nil, fmt.Errorf("%w: dead-letter cursor time is invalid", ErrInvalid)
		}
		if err := validateJobID(cursor.ID); err != nil {
			return nil, fmt.Errorf("%w: dead-letter cursor ID is invalid", ErrInvalid)
		}
		cursorTime = cursor.FinishedAt
		cursorID = cursor.ID
	}
	rows, err := repository.database.Query(ctx, listDeadSQL, queue, cursorTime, cursorID, limit)
	if err != nil {
		return nil, fmt.Errorf("list dead jobs: %w", err)
	}
	defer rows.Close()
	jobs := make([]Job, 0, limit)
	for rows.Next() {
		job, scanErr := scanJob(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan dead job: %w", scanErr)
		}
		if job.State != StateDead {
			return nil, fmt.Errorf("dead-letter query returned state %q", job.State)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dead jobs: %w", err)
	}
	return jobs, nil
}

// Cancel idempotently transitions pending or running work to canceled. It
// cannot reverse a handler side effect that already occurred.
//
// Complexity: for ID bytes i and payload bytes p in the returned row, local
// time O(i+p), Omega(i), tight Theta(i+p); auxiliary space O(p), Omega(p),
// tight Theta(p); database cost is one primary-key update and one read only
// when no transition occurs.
func (repository *PostgreSQL) Cancel(ctx context.Context, id string) (Job, error) {
	if err := validateMutation(repository, ctx, id); err != nil {
		return Job{}, err
	}
	return transact(ctx, repository.database, func(transaction pgx.Tx) (Job, error) {
		job, err := scanJob(transaction.QueryRow(ctx, cancelSQL, id))
		if err == nil {
			return job, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return Job{}, fmt.Errorf("cancel job: %w", err)
		}
		job, err = scanJob(transaction.QueryRow(ctx, getSQL, id))
		if errors.Is(err, pgx.ErrNoRows) {
			return Job{}, ErrNotFound
		}
		if err != nil {
			return Job{}, fmt.Errorf("read unmodified job after cancel: %w", err)
		}
		if job.State == StateCanceled {
			return job, nil
		}
		return Job{}, ErrStateConflict
	})
}

// Redrive explicitly returns one dead job to pending and resets its attempt
// budget. It rejects every other state.
//
// Complexity: for ID bytes i and payload bytes p in the returned row, local
// time O(i+p), Omega(i), tight Theta(i+p); auxiliary space O(p), Omega(p),
// tight Theta(p); database cost is one primary-key update and one read only
// when no transition occurs.
func (repository *PostgreSQL) Redrive(ctx context.Context, id string) (Job, error) {
	if err := validateMutation(repository, ctx, id); err != nil {
		return Job{}, err
	}
	return transact(ctx, repository.database, func(transaction pgx.Tx) (Job, error) {
		job, err := scanJob(transaction.QueryRow(ctx, redriveSQL, id))
		if err == nil {
			return job, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return Job{}, fmt.Errorf("redrive job: %w", err)
		}
		if _, err := scanJob(transaction.QueryRow(ctx, getSQL, id)); errors.Is(err, pgx.ErrNoRows) {
			return Job{}, ErrNotFound
		} else if err != nil {
			return Job{}, fmt.Errorf("read unmodified job after redrive: %w", err)
		}
		return Job{}, ErrStateConflict
	})
}

// validateRead checks the common read boundary.
//
// Complexity: time O(1), Omega(1), tight Theta(1); auxiliary space O(1),
// Omega(1), tight Theta(1).
func validateRead(repository *PostgreSQL, ctx context.Context) error {
	if repository == nil || nilLike(repository.database) || ctx == nil {
		return fmt.Errorf("%w: repository and context are required", ErrInvalid)
	}
	return nil
}

// validateMutation checks the common repository, context, and job-ID boundary.
//
// Complexity: for ID bytes i, time O(i), Omega(1), tight Theta(i) for valid
// input; auxiliary space O(1), Omega(1), tight Theta(1).
func validateMutation(repository *PostgreSQL, ctx context.Context, id string) error {
	if err := validateRead(repository, ctx); err != nil {
		return err
	}
	return validateJobID(id)
}

// validateJobID accepts exactly one lowercase 128-bit hexadecimal ID.
//
// Complexity: for ID bytes i, time O(i), Omega(1), tight Theta(i) for valid
// input; auxiliary space O(1), Omega(1), tight Theta(1).
func validateJobID(id string) error {
	if len(id) != 32 || !isLowerHex(id) {
		return fmt.Errorf("%w: job ID is invalid", ErrInvalid)
	}
	return nil
}
