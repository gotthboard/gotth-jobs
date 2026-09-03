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
type Store interface {
	Claim(context.Context, ClaimRequest) (Job, error)
	Heartbeat(context.Context, Lease, time.Duration) (Job, error)
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

var errHandlerPanicked = errors.New("handler panicked")

// Run claims and handles one job at a time until ctx ends or a store/lease
// failure makes continued operation dishonest.
//
// Complexity: for c claim cycles and total handler/database work H, time
// O(c)+H, Omega(1), with no finite tight bound because ctx controls lifetime;
// auxiliary space O(1), Omega(1), tight Theta(1) beyond one copied job payload
// and one bounded heartbeat goroutine.
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

// validate rejects configurations that could busy-loop, outlive their lease,
// or omit a required execution boundary.
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
	if worker.HeartbeatInterval <= 0 || worker.HeartbeatInterval >= worker.LeaseDuration {
		return fmt.Errorf("%w: heartbeat interval must be positive and shorter than the lease", ErrInvalid)
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
// count; total time includes handler H plus h delegated heartbeat calls and one
// completion/failure call; auxiliary space O(1), Omega(1), tight Theta(1)
// beyond the handler's own space.
func (worker Worker) runAttempt(ctx context.Context, job Job) error {
	attemptContext, cancel := context.WithCancel(ctx)
	stopHeartbeat := make(chan struct{})
	heartbeatResult := make(chan error, 1)
	go func() {
		err := worker.heartbeat(attemptContext, stopHeartbeat, job.Lease)
		if err != nil {
			cancel()
		}
		heartbeatResult <- err
	}()

	handlerErr := callHandler(attemptContext, worker.Handler, job)
	close(stopHeartbeat)
	heartbeatErr := <-heartbeatResult
	cancel()

	if err := ctx.Err(); err != nil {
		return err
	}
	if heartbeatErr != nil {
		if errors.Is(heartbeatErr, ErrCanceled) {
			return nil
		}
		return fmt.Errorf("heartbeat worker job: %w", heartbeatErr)
	}
	if handlerErr == nil {
		_, err := worker.Store.Complete(ctx, job.Lease)
		if errors.Is(err, ErrCanceled) {
			return nil
		}
		if err != nil {
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
	_, err := worker.Store.Fail(ctx, job.Lease, Failure{
		Message: boundedFailure(handlerErr), RetryAfter: delay,
		Permanent: permanent,
	})
	if errors.Is(err, ErrCanceled) {
		return nil
	}
	if err != nil {
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
			if _, err := worker.Store.Heartbeat(ctx, lease, worker.LeaseDuration); err != nil {
				return err
			}
		}
	}
}

// callHandler contains a panic within the current attempt without copying the
// panic value into persisted failure text.
//
// Complexity: delegated handler time and space dominate; local time and space
// are tight Theta(1).
func callHandler(ctx context.Context, handler Handler, job Job) (err error) {
	defer func() {
		if recover() != nil {
			err = errHandlerPanicked
		}
	}()
	return handler(ctx, job)
}

// boundedFailure converts arbitrary error text into valid UTF-8 without NULs
// and truncates it on a rune boundary with an explicit ellipsis.
//
// Complexity: for n error bytes, time O(n), Omega(n), tight Theta(n);
// auxiliary space O(n), Omega(n), tight Theta(n).
func boundedFailure(err error) string {
	message := strings.ToValidUTF8(err.Error(), "�")
	message = strings.ReplaceAll(message, "\x00", "�")
	if len(message) <= MaxFailureBytes {
		return message
	}
	const suffix = "…"
	end := MaxFailureBytes - len(suffix)
	for end > 0 && !utf8.ValidString(message[:end]) {
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
