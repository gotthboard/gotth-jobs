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

var jobResultFormats = pgx.QueryResultFormatsByOID{
	pgtype.TextOID:        pgx.BinaryFormatCode,
	pgtype.ByteaOID:       pgx.BinaryFormatCode,
	pgtype.Int4OID:        pgx.BinaryFormatCode,
	pgtype.TimestamptzOID: pgx.BinaryFormatCode,
}

// jobQueryArguments forces pgx to describe each job-returning statement and
// decode every job-column type from its binary result format regardless of the
// connection's configured default query mode. DescribeExec is required because
// the OID result map is not consulted by pgx's Exec or SimpleProtocol paths.
//
// Complexity: for n SQL arguments, time and auxiliary space are tight
// Theta(n) for the query-owned argument slice. Query execution performs the
// two protocol round trips required by pgx DescribeExec.
func jobQueryArguments(arguments ...any) []any {
	result := make([]any, 0, len(arguments)+2)
	result = append(result, pgx.QueryExecModeDescribeExec, jobResultFormats)
	return append(result, arguments...)
}

type boundedBytesScanner struct {
	label   string
	minimum int
	maximum int
	value   []byte
}

var _ pgtype.BytesScanner = (*boundedBytesScanner)(nil)

// ScanBytes rejects an untrusted bytea length before allocation and makes the
// one owning copy required beyond pgx's borrowed driver-memory lifetime.
//
// Complexity: for accepted source size n, time and auxiliary space are tight
// Theta(n); rejection time and auxiliary space are tight Theta(1).
func (scanner *boundedBytesScanner) ScanBytes(source []byte) error {
	if source == nil {
		return fmt.Errorf("stored job %s is NULL", scanner.label)
	}
	if len(source) < scanner.minimum || len(source) > scanner.maximum {
		return fmt.Errorf("stored job %s violates the schema length contract", scanner.label)
	}
	scanner.value = make([]byte, len(source))
	copy(scanner.value, source)
	return nil
}

type boundedTextScanner struct {
	label    string
	minimum  int
	maximum  int
	nullable bool
	present  bool
	value    string
}

var _ pgtype.BytesScanner = (*boundedTextScanner)(nil)

// ScanBytes checks borrowed text bytes before the one string conversion that
// gives the returned Job ownership beyond pgx's driver-memory lifetime.
//
// Complexity: for accepted source size n, time and auxiliary space are tight
// Theta(n); rejection time and auxiliary space are tight Theta(1).
func (scanner *boundedTextScanner) ScanBytes(source []byte) error {
	if source == nil {
		if scanner.nullable {
			scanner.present = false
			scanner.value = ""
			return nil
		}
		return fmt.Errorf("stored job %s is NULL", scanner.label)
	}
	scanner.present = true
	if len(source) < scanner.minimum || len(source) > scanner.maximum {
		return fmt.Errorf("stored job %s violates the schema length contract", scanner.label)
	}
	scanner.value = string(source)
	return nil
}

// scanJob converts one untrusted database row into a copied public value and
// rejects impossible states even if database constraints were bypassed.
//
// Complexity: for total accepted variable-width data n, time and auxiliary
// space are O(n), Omega(1), and tight Theta(n); one delegated row scan is
// required. A bounded-column length rejection is constant-time and makes no
// ownership allocation for that column.
func scanJob(row pgx.Row) (Job, error) {
	return scanJobRow(row, nil)
}

// scanJobWithFingerprint converts a fingerprint and complete job selected
// from one row and statement snapshot into copied public values.
//
// Complexity: for total accepted variable-width data n, time and auxiliary
// space are O(n), Omega(1), and tight Theta(n); one delegated row scan is
// required. A bounded-column length rejection is constant-time and makes no
// ownership allocation for that column.
func scanJobWithFingerprint(row pgx.Row) ([]byte, Job, error) {
	fingerprint := boundedBytesScanner{
		label: "request fingerprint", minimum: 32, maximum: 32,
	}
	job, err := scanJobRow(row, &fingerprint)
	if err != nil {
		return nil, Job{}, err
	}
	return fingerprint.value, job, nil
}

// scanJobRow owns the shared decoding and validation for ordinary job rows
// and idempotency rows with a leading fingerprint column.
//
// Complexity: for total accepted variable-width data n, time and auxiliary
// space are O(n), Omega(1), and tight Theta(n); one delegated row scan is
// required. A bounded-column length rejection is constant-time and makes no
// ownership allocation for that column.
func scanJobRow(row pgx.Row, fingerprint *boundedBytesScanner) (Job, error) {
	var job Job
	id := boundedTextScanner{label: "ID", minimum: 32, maximum: 32}
	queue := boundedTextScanner{label: "queue", minimum: 1, maximum: MaxQueueBytes}
	kind := boundedTextScanner{label: "kind", minimum: 1, maximum: MaxKindBytes}
	payload := boundedBytesScanner{label: "payload", maximum: MaxPayloadBytes}
	key := boundedTextScanner{label: "idempotency key", minimum: 1, maximum: MaxIdempotencyKeyBytes, nullable: true}
	state := boundedTextScanner{label: "state", minimum: 1, maximum: len(StateSucceeded)}
	token := boundedTextScanner{label: "lease token", minimum: 64, maximum: 64, nullable: true}
	owner := boundedTextScanner{label: "lease owner", minimum: 1, maximum: MaxWorkerBytes, nullable: true}
	lastError := boundedTextScanner{label: "last error", maximum: MaxFailureBytes}
	var availableAt, createdAt, updatedAt, leaseUntil, finishedAt *time.Time
	var err error
	if fingerprint == nil {
		err = row.Scan(
			&id, &queue, &kind, &payload, &key, &state,
			&job.Attempts, &job.MaxAttempts, &availableAt, &createdAt,
			&updatedAt, &token, &owner, &leaseUntil, &lastError,
			&finishedAt,
		)
	} else {
		err = row.Scan(
			fingerprint, &id, &queue, &kind, &payload, &key,
			&state, &job.Attempts, &job.MaxAttempts, &availableAt,
			&createdAt, &updatedAt, &token, &owner, &leaseUntil,
			&lastError, &finishedAt,
		)
	}
	if err != nil {
		return Job{}, err
	}
	if availableAt == nil || createdAt == nil || updatedAt == nil {
		return Job{}, fmt.Errorf("stored job has NULL mandatory timestamp")
	}
	job.ID = id.value
	job.Queue = queue.value
	job.Kind = kind.value
	job.Payload = payload.value
	job.State = State(state.value)
	job.LastError = lastError.value
	if key.present {
		job.IdempotencyKey = key.value
	}
	if token.present {
		job.Lease = Lease{JobID: job.ID, Token: token.value}
	}
	if owner.present {
		job.LeaseOwner = owner.value
	}
	job.AvailableAt = availableAt.UTC()
	job.CreatedAt = createdAt.UTC()
	job.UpdatedAt = updatedAt.UTC()
	if leaseUntil != nil {
		job.LeaseUntil = leaseUntil.UTC()
		if !isPostgreSQLTimestamp(job.LeaseUntil) {
			return Job{}, fmt.Errorf("stored job has invalid lease timestamp")
		}
	}
	if finishedAt != nil {
		job.FinishedAt = finishedAt.UTC()
		if !isPostgreSQLTimestamp(job.FinishedAt) {
			return Job{}, fmt.Errorf("stored job has invalid finished timestamp")
		}
	}
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
	if job.Payload == nil {
		return fmt.Errorf("stored job payload is NULL")
	}
	if len(job.Payload) > MaxPayloadBytes || len(job.IdempotencyKey) > MaxIdempotencyKeyBytes || len(job.LastError) > MaxFailureBytes || job.Attempts < 0 || job.Attempts > job.MaxAttempts || job.MaxAttempts < 1 || job.MaxAttempts > MaxAttempts {
		return fmt.Errorf("stored job violates the schema contract")
	}
	if !isPostgreSQLTimestamp(job.AvailableAt) || !isPostgreSQLTimestamp(job.CreatedAt) || !isPostgreSQLTimestamp(job.UpdatedAt) {
		return fmt.Errorf("stored job has invalid mandatory timestamp")
	}
	if (!job.LeaseUntil.IsZero() && !isPostgreSQLTimestamp(job.LeaseUntil)) ||
		(!job.FinishedAt.IsZero() && !isPostgreSQLTimestamp(job.FinishedAt)) {
		return fmt.Errorf("stored job has invalid optional timestamp")
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
		if job.Lease != (Lease{}) || job.LeaseOwner != "" || !job.LeaseUntil.IsZero() {
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
