package jobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Store is the exact lease lifecycle required by Worker. PostgreSQL satisfies
// it; test doubles can model handler and shutdown behavior without a database.
// Complete and Fail must preserve any produced Job when returning
// ErrCommitOutcomeUnknown so Worker can expose it for reconciliation.
type Store interface {
	Claim(context.Context, ClaimRequest) (Job, error)
	Heartbeat(context.Context, Lease, time.Duration) error
	Complete(context.Context, Lease) (Job, error)
	Fail(context.Context, Lease, Failure) (Job, error)
}

// Handler performs one at-least-once attempt. It must honor context
// cancellation and make non-repeatable external effects idempotent.
type Handler func(context.Context, Job) error

// Worker runs one job at a time with a bounded heartbeat goroutine.
type Worker struct {
	Store             Store
	Queue             string
	WorkerID          string
	LeaseDuration     time.Duration
	HeartbeatInterval time.Duration
	PollInterval      time.Duration
	RetryPolicy       RetryPolicy
	Handler           Handler
}

var (
	errHandlerPanicked = errors.New("handler panicked")
	errHandlerFinished = errors.New("handler finished")
)

// Run claims and handles one job at a time until ctx ends or a store/lease
// failure makes continued operation dishonest. Unknown acknowledgement commit
// outcomes return LeaseReconciliationError and are never retried implicitly.
//
// Complexity: for c claim cycles and total handler/database work H, time
// O(c)+H, Omega(1), with no finite tight bound because ctx controls lifetime;
// auxiliary space O(1), Omega(1), tight Theta(1) beyond one copied job payload
// plus one MaxFailureBytes normalization buffer and one bounded heartbeat
// goroutine.
func (worker Worker) Run(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("%w: context is required", ErrInvalid)
	}
	if err := worker.validate(); err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		job, err := worker.Store.Claim(ctx, ClaimRequest{
			Queue: worker.Queue, Worker: worker.WorkerID,
			LeaseDuration: worker.LeaseDuration,
		})
		if errors.Is(err, ErrNoJob) {
			if err := waitContext(ctx, worker.PollInterval); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			if errors.Is(err, ErrCommitOutcomeUnknown) && job.ID != "" {
				return &ClaimReconciliationError{job: job, err: err}
			}
			return fmt.Errorf("claim worker job: %w", err)
		}
		if err := validateClaimedAttempt(job, worker.Queue, worker.WorkerID); err != nil {
			return err
		}
		if err := worker.runAttempt(ctx, job); err != nil {
			return err
		}
	}
}

// validateClaimedAttempt rejects a Store implementation that returns a job
// outside the requested queue or without a usable running lease.
//
// Complexity: for all variable job text bytes n, time O(n), Omega(1), tight
// Theta(n) for a valid job; payload validation reads only its length;
// auxiliary space O(1), Omega(1), tight Theta(1).
func validateClaimedAttempt(job Job, queue, workerID string) error {
	if job.Queue != queue || job.LeaseOwner != workerID {
		return fmt.Errorf("%w: store returned an invalid claimed job", ErrInvalid)
	}
	if err := validateStoredJob(job); err != nil {
		return fmt.Errorf("%w: store returned an invalid claimed job: %v", ErrInvalid, err)
	}
	return nil
}

// validate rejects configurations that could busy-loop, omit the explicit
// half-lease renewal budget, or omit a required execution boundary.
//
// Complexity: for queue bytes q and worker bytes w, time O(q+w), Omega(q),
// tight Theta(q+w); auxiliary space O(1), Omega(1), tight Theta(1).
func (worker Worker) validate() error {
	if nilLike(worker.Store) || worker.Handler == nil {
		return fmt.Errorf("%w: worker store and handler are required", ErrInvalid)
	}
	if err := validateClaim(ClaimRequest{Queue: worker.Queue, Worker: worker.WorkerID, LeaseDuration: worker.LeaseDuration}); err != nil {
		return err
	}
	if worker.HeartbeatInterval <= 0 || worker.HeartbeatInterval > worker.LeaseDuration/2 {
		return fmt.Errorf("%w: heartbeat interval must be positive and no more than half the lease", ErrInvalid)
	}
	if worker.PollInterval <= 0 || worker.PollInterval > time.Minute {
		return fmt.Errorf("%w: poll interval must be between zero and one minute", ErrInvalid)
	}
	if _, err := worker.RetryPolicy.Delay(1); err != nil {
		return err
	}
	if worker.RetryPolicy.Initial%time.Microsecond != 0 || worker.RetryPolicy.Maximum%time.Microsecond != 0 {
		return fmt.Errorf("%w: retry policy must use PostgreSQL microsecond precision", ErrInvalid)
	}
	return nil
}

// runAttempt coordinates one handler with one heartbeat lifetime, then records
// exactly one success or failure transition while the lease remains valid.
//
// Complexity: local coordination time is O(h), Omega(1), where h is heartbeat
// count; total time includes handler H plus h delegated constant-response
// heartbeat calls and one cancellation/join plus one completion/failure call;
// auxiliary space O(1), Omega(1), tight Theta(1) beyond the handler's own
// space, with failure normalization capped at MaxFailureBytes source bytes.
func (worker Worker) runAttempt(ctx context.Context, job Job) error {
	attemptContext, cancel := context.WithCancelCause(ctx)
	stopHeartbeat := make(chan struct{})
	heartbeatResult := make(chan error, 1)
	go func() {
		err := worker.heartbeat(attemptContext, stopHeartbeat, job.Lease)
		if err != nil {
			cancel(err)
		}
		heartbeatResult <- err
	}()

	handlerErr := callHandler(attemptContext, worker.Handler, job)
	close(stopHeartbeat)
	cancel(errHandlerFinished)
	heartbeatErr := <-heartbeatResult

	if errors.Is(heartbeatErr, ErrCommitOutcomeUnknown) {
		return &LeaseReconciliationError{job: job, lease: job.Lease, err: heartbeatErr}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	stoppedAfterHandler := errors.Is(heartbeatErr, context.Canceled) &&
		errors.Is(context.Cause(attemptContext), errHandlerFinished)
	if heartbeatErr != nil && !stoppedAfterHandler {
		if errors.Is(heartbeatErr, ErrCanceled) {
			return nil
		}
		return fmt.Errorf("heartbeat worker job: %w", heartbeatErr)
	}
	if handlerErr == nil {
		completed, err := worker.Store.Complete(ctx, job.Lease)
		if errors.Is(err, ErrCanceled) {
			return nil
		}
		if err != nil {
			if errors.Is(err, ErrCommitOutcomeUnknown) {
				return &LeaseReconciliationError{job: completed, lease: job.Lease, err: err}
			}
			return fmt.Errorf("complete worker job: %w", err)
		}
		return nil
	}

	permanent := IsPermanent(handlerErr)
	var delay time.Duration
	if !permanent {
		var err error
		delay, err = worker.RetryPolicy.Delay(job.Attempts)
		if err != nil {
			return err
		}
	}
	failed, err := worker.Store.Fail(ctx, job.Lease, Failure{
		Message: boundedFailure(handlerErr), RetryAfter: delay,
		Permanent: permanent,
	})
	if errors.Is(err, ErrCanceled) {
		return nil
	}
	if err != nil {
		if errors.Is(err, ErrCommitOutcomeUnknown) {
			return &LeaseReconciliationError{job: failed, lease: job.Lease, err: err}
		}
		return fmt.Errorf("fail worker job: %w", err)
	}
	return nil
}

// heartbeat extends the lease on a fixed interval until the attempt stops,
// its context ends, or the store rejects the lease.
//
// Complexity: for h ticks, time O(h)+H, Omega(1), with no finite tight bound
// because lifetime is externally controlled; H is delegated heartbeat time;
// auxiliary space O(1), Omega(1), tight Theta(1).
func (worker Worker) heartbeat(ctx context.Context, stop <-chan struct{}, lease Lease) error {
	ticker := time.NewTicker(worker.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := worker.Store.Heartbeat(ctx, lease, worker.LeaseDuration); err != nil {
				return err
			}
		}
	}
}

// callHandler contains every panic unwind within the current attempt without
// inspecting or copying the panic value. The completion flag distinguishes a
// normal nil return from panic(nil) when legacy panicnil behavior is enabled.
//
// Complexity: delegated handler time and space dominate; local time and space
// are tight Theta(1).
func callHandler(ctx context.Context, handler Handler, job Job) (err error) {
	completed := false
	defer func() {
		if !completed {
			_ = recover()
			err = errHandlerPanicked
		}
	}()
	err = handler(ctx, job)
	completed = true
	return err
}

// boundedFailure converts a bounded prefix of arbitrary error text into valid
// UTF-8 without NULs and truncates it on a rune boundary with an ellipsis.
//
// Complexity: err.Error() is delegated. After it returns n bytes, local time is
// tight Theta(1+min(n, MaxFailureBytes)) and auxiliary space is
// O(MaxFailureBytes), both independent of the full source length once capped.
func boundedFailure(err error) string {
	const suffix = "…"
	source := err.Error()
	sourceLimit := MaxFailureBytes
	truncated := len(source) > sourceLimit
	if truncated {
		sourceLimit -= len(suffix)
	}
	source = source[:min(len(source), sourceLimit)]

	message := strings.ToValidUTF8(source, "�")
	message = strings.ReplaceAll(message, "\x00", "�")
	if !truncated && len(message) <= MaxFailureBytes {
		return message
	}
	end := min(len(message), MaxFailureBytes-len(suffix))
	for end < len(message) && end > 0 && !utf8.RuneStart(message[end]) {
		end--
	}
	return message[:end] + suffix
}

// waitContext waits once without leaking a timer after cancellation.
//
// Complexity: time is the delegated wait duration unless canceled; local time
// and auxiliary space are tight Theta(1).
func waitContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
