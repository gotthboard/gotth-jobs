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
