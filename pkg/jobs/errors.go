package jobs

import "errors"

// Stable sentinels let callers classify failures with errors.Is without
// parsing diagnostic text.
var (
	ErrInvalid              = errors.New("invalid job input")
	ErrNotFound             = errors.New("job not found")
	ErrNoJob                = errors.New("no eligible job")
	ErrLeaseLost            = errors.New("job lease lost")
	ErrCanceled             = errors.New("job canceled")
	ErrStateConflict        = errors.New("job state conflict")
	ErrIdempotencyConflict  = errors.New("idempotency key conflicts with an existing request")
	ErrCommitOutcomeUnknown = errors.New("job transaction commit outcome unknown")
)

// ClaimReconciliationError reports that Worker.Run received a nonzero Claim
// result whose commit outcome is unknown. The job is not authorization to run
// its handler; it exists only to reconcile the durable ID and lease token.
type ClaimReconciliationError struct {
	job Job
	err error
}

// Error deliberately omits all job fields, including the secret lease token.
//
// Complexity: time O(1), Omega(1), tight Theta(1); auxiliary space O(1),
// Omega(1), tight Theta(1).
func (err *ClaimReconciliationError) Error() string {
	return "claim worker job commit outcome is unknown; reconciliation required"
}

// Unwrap preserves the original Claim error for errors.Is and errors.As.
//
// Complexity: time O(1), Omega(1), tight Theta(1); auxiliary space O(1),
// Omega(1), tight Theta(1).
func (err *ClaimReconciliationError) Unwrap() error {
	return err.err
}

// ReconciliationJob returns the unconfirmed Claim result. Consumers may use
// its ID and lease token only to inspect durable state; they must not handle
// the job until that state and lease have been confirmed.
//
// Complexity: time O(1), Omega(1), tight Theta(1); auxiliary space O(1),
// Omega(1), tight Theta(1).
func (err *ClaimReconciliationError) ReconciliationJob() Job {
	return err.job
}

// LeaseReconciliationError reports an acknowledgement whose commit outcome
// is unknown. Its values are reconciliation-only and do not authorize a retry
// or another job mutation.
type LeaseReconciliationError struct {
	job   Job
	lease Lease
	err   error
}

// Error deliberately omits the job ID and secret lease token.
//
// Complexity: time O(1), Omega(1), tight Theta(1); auxiliary space O(1),
// Omega(1), tight Theta(1).
func (err *LeaseReconciliationError) Error() string {
	return "worker job acknowledgement commit outcome is unknown; reconciliation required"
}

// Unwrap preserves the original acknowledgement error for errors.Is and
// errors.As.
//
// Complexity: time O(1), Omega(1), tight Theta(1); auxiliary space O(1),
// Omega(1), tight Theta(1).
func (err *LeaseReconciliationError) Unwrap() error {
	return err.err
}

// ReconciliationJob returns the unconfirmed acknowledgement result. For a
// Heartbeat outcome it is the claimed job known to Worker.
//
// Complexity: time O(1), Omega(1), tight Theta(1); auxiliary space O(1),
// Omega(1), tight Theta(1).
func (err *LeaseReconciliationError) ReconciliationJob() Job {
	return err.job
}

// ReconciliationLease returns the exact affected lease. The token is secret
// fencing material for durable reconciliation and must not be logged.
//
// Complexity: time O(1), Omega(1), tight Theta(1); auxiliary space O(1),
// Omega(1), tight Theta(1).
func (err *LeaseReconciliationError) ReconciliationLease() Lease {
	return err.lease
}

type permanentError struct {
	err error
}

func (err permanentError) Error() string { return err.err.Error() }
func (err permanentError) Unwrap() error { return err.err }

// Permanent classifies err as a non-retryable handler failure while
// preserving errors.Is and errors.As traversal. Nil remains nil.
//
// Complexity: time O(1), Omega(1), tight Theta(1); auxiliary space O(1),
// Omega(1), tight Theta(1).
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return permanentError{err: err}
}

// IsPermanent reports whether err contains the marker produced by Permanent.
//
// Complexity: for wrapper depth d, time O(d), Omega(1), with no single tight
// bound across all inputs; auxiliary space O(1), Omega(1), tight Theta(1).
func IsPermanent(err error) bool {
	var target permanentError
	return errors.As(err, &target)
}
