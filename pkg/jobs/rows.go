package jobs

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const jobColumns = `id, queue, kind, payload, idempotency_key, state,
attempts, max_attempts, available_at, created_at, updated_at,
lease_token, lease_owner, lease_until, last_error, finished_at`

const qualifiedJobColumns = `job.id, job.queue, job.kind, job.payload,
job.idempotency_key, job.state, job.attempts, job.max_attempts,
job.available_at, job.created_at, job.updated_at, job.lease_token,
job.lease_owner, job.lease_until, job.last_error, job.finished_at`

type boundedPayloadScanner struct {
	value []byte
}

var _ pgtype.BytesScanner = (*boundedPayloadScanner)(nil)

// ScanBytes rejects an untrusted bytea length before allocation and makes the
// one owning copy required beyond pgx's borrowed driver-memory lifetime.
//
// Complexity: for accepted payload size p, time and auxiliary space are tight
// Theta(p); rejection time and auxiliary space are tight Theta(1).
func (scanner *boundedPayloadScanner) ScanBytes(source []byte) error {
	if len(source) > MaxPayloadBytes {
		return fmt.Errorf("stored job payload exceeds the schema contract")
	}
	if source == nil {
		scanner.value = nil
		return nil
	}
	scanner.value = make([]byte, len(source))
	copy(scanner.value, source)
	return nil
}

// scanJob converts one untrusted database row into a copied public value and
// rejects impossible states even if database constraints were bypassed.
//
// Complexity: for payload size p, time and auxiliary space are O(p), Omega(1),
// and tight Theta(p) for an accepted payload; one delegated row scan is
// required. Oversized payload rejection is constant-time and allocates no
// payload-sized storage.
func scanJob(row pgx.Row) (Job, error) {
	return scanJobRow(row, nil)
}

// scanJobWithFingerprint converts a fingerprint and complete job selected
// from one row and statement snapshot into copied public values.
//
// Complexity: for payload size p, time and auxiliary space are O(p), Omega(1),
// and tight Theta(p) for an accepted payload; one delegated row scan is
// required. Oversized payload rejection is constant-time and allocates no
// payload-sized storage.
func scanJobWithFingerprint(row pgx.Row) ([]byte, Job, error) {
	var fingerprint []byte
	job, err := scanJobRow(row, &fingerprint)
	if err != nil {
		return nil, Job{}, err
	}
	return fingerprint, job, nil
}

// scanJobRow owns the shared decoding and validation for ordinary job rows
// and idempotency rows with a leading fingerprint column.
//
// Complexity: for payload size p, time and auxiliary space are O(p), Omega(1),
// and tight Theta(p) for an accepted payload; one delegated row scan is
// required. Oversized payload rejection is constant-time and allocates no
// payload-sized storage.
func scanJobRow(row pgx.Row, fingerprint *[]byte) (Job, error) {
	var job Job
	var payload boundedPayloadScanner
	var state string
	var key, token, owner *string
	var leaseUntil, finishedAt *time.Time
	var err error
	if fingerprint == nil {
		err = row.Scan(
			&job.ID, &job.Queue, &job.Kind, &payload, &key, &state,
			&job.Attempts, &job.MaxAttempts, &job.AvailableAt, &job.CreatedAt,
			&job.UpdatedAt, &token, &owner, &leaseUntil, &job.LastError,
			&finishedAt,
		)
	} else {
		err = row.Scan(
			fingerprint, &job.ID, &job.Queue, &job.Kind, &payload, &key,
			&state, &job.Attempts, &job.MaxAttempts, &job.AvailableAt,
			&job.CreatedAt, &job.UpdatedAt, &token, &owner, &leaseUntil,
			&job.LastError, &finishedAt,
		)
	}
	if err != nil {
		return Job{}, err
	}
	job.Payload = payload.value
	job.State = State(state)
	if key != nil {
		job.IdempotencyKey = *key
	}
	if token != nil {
		job.Lease = Lease{JobID: job.ID, Token: *token}
	}
	if owner != nil {
		job.LeaseOwner = *owner
	}
	if leaseUntil != nil {
		job.LeaseUntil = leaseUntil.UTC()
	}
	if finishedAt != nil {
		job.FinishedAt = finishedAt.UTC()
	}
	job.AvailableAt = job.AvailableAt.UTC()
	job.CreatedAt = job.CreatedAt.UTC()
	job.UpdatedAt = job.UpdatedAt.UTC()
	if err := validateStoredJob(job); err != nil {
		return Job{}, err
	}
	return job, nil
}

// validateStoredJob checks invariants that remain security-relevant after a
// database row crosses the library boundary.
//
// Complexity: for q queue, k kind, i key, w owner, e error, and token bytes,
// time O(q+k+i+w+e+token), Omega(q+k), tight Theta(q+k+i+w+e+token) because
// payload validation reads only its length and state/attempt checks are
// constant-time; auxiliary space O(1), Omega(1), tight Theta(1).
func validateStoredJob(job Job) error {
	if len(job.ID) != 32 || !isLowerHex(job.ID) {
		return fmt.Errorf("stored job ID violates the schema contract")
	}
	if err := validateName("stored queue", job.Queue, MaxQueueBytes); err != nil {
		return fmt.Errorf("stored job violates the schema contract: %w", err)
	}
	if err := validateName("stored kind", job.Kind, MaxKindBytes); err != nil {
		return fmt.Errorf("stored job violates the schema contract: %w", err)
	}
	if len(job.Payload) > MaxPayloadBytes || len(job.IdempotencyKey) > MaxIdempotencyKeyBytes || len(job.LastError) > MaxFailureBytes || job.Attempts < 0 || job.Attempts > job.MaxAttempts || job.MaxAttempts < 1 || job.MaxAttempts > MaxAttempts {
		return fmt.Errorf("stored job violates the schema contract")
	}
	if !utf8.ValidString(job.IdempotencyKey) || !utf8.ValidString(job.LeaseOwner) || !utf8.ValidString(job.LastError) || strings.IndexByte(job.IdempotencyKey, 0) >= 0 || strings.IndexByte(job.LeaseOwner, 0) >= 0 || strings.IndexByte(job.LastError, 0) >= 0 {
		return fmt.Errorf("stored job has invalid text")
	}
	if (job.State == StatePending && job.Attempts >= job.MaxAttempts) ||
		((job.State == StateRunning || job.State == StateSucceeded || job.State == StateDead) && job.Attempts == 0) {
		return fmt.Errorf("stored job has invalid state and attempt combination")
	}
	switch job.State {
	case StatePending, StateSucceeded, StateDead, StateCanceled:
		if job.Lease.Token != "" || job.LeaseOwner != "" || !job.LeaseUntil.IsZero() {
			return fmt.Errorf("stored non-running job has lease state")
		}
	case StateRunning:
		if job.Lease.JobID != job.ID || len(job.Lease.Token) != 64 || !isLowerHex(job.Lease.Token) || job.LeaseOwner == "" || len(job.LeaseOwner) > MaxWorkerBytes || job.LeaseUntil.IsZero() {
			return fmt.Errorf("stored running job has invalid lease state")
		}
	default:
		return fmt.Errorf("stored job has unknown state %q", job.State)
	}
	terminal := job.State == StateSucceeded || job.State == StateDead || job.State == StateCanceled
	if terminal == job.FinishedAt.IsZero() {
		return fmt.Errorf("stored job has invalid terminal timestamp state")
	}
	return nil
}

// isLowerHex reports whether every byte is a lowercase hexadecimal digit.
//
// Complexity: for n bytes, time O(n), Omega(1), tight Theta(n) for valid
// input; auxiliary space O(1), Omega(1), tight Theta(1).
func isLowerHex(value string) bool {
	for index := range len(value) {
		character := value[index]
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
