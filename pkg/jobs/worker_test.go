package jobs

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type stubStore struct {
	claim     func(context.Context, ClaimRequest) (Job, error)
	heartbeat func(context.Context, Lease, time.Duration) error
	complete  func(context.Context, Lease) (Job, error)
	fail      func(context.Context, Lease, Failure) (Job, error)
}

func (store *stubStore) Claim(ctx context.Context, request ClaimRequest) (Job, error) {
	return store.claim(ctx, request)
}

func (store *stubStore) Heartbeat(ctx context.Context, lease Lease, duration time.Duration) error {
	return store.heartbeat(ctx, lease, duration)
}

func (store *stubStore) Complete(ctx context.Context, lease Lease) (Job, error) {
	return store.complete(ctx, lease)
}

func (store *stubStore) Fail(ctx context.Context, lease Lease, failure Failure) (Job, error) {
	return store.fail(ctx, lease, failure)
}

func validWorker(store Store, handler Handler) Worker {
	return Worker{
		Store:             store,
		Queue:             "default",
		WorkerID:          "worker-1",
		LeaseDuration:     time.Second,
		HeartbeatInterval: 10 * time.Millisecond,
		PollInterval:      time.Millisecond,
		RetryPolicy:       RetryPolicy{Initial: time.Second, Maximum: time.Minute},
		Handler:           handler,
	}
}

func claimedJob(attempt int) Job {
	now := time.Unix(1_900_000_000, 0).UTC()
	return Job{
		ID: "0123456789abcdef0123456789abcdef", Queue: "default", Kind: "send",
		Payload: []byte{}, State: StateRunning, Attempts: attempt, MaxAttempts: 3,
		AvailableAt: now, CreatedAt: now, UpdatedAt: now,
		Lease:      Lease{JobID: "0123456789abcdef0123456789abcdef", Token: strings.Repeat("ab", 32)},
		LeaseOwner: "worker-1", LeaseUntil: now.Add(time.Minute),
	}
}

func TestWorkerRunClaimsHandlesAndCompletes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	job := claimedJob(1)
	claims := 0
	store := &stubStore{
		claim: func(context.Context, ClaimRequest) (Job, error) {
			claims++
			return job, nil
		},
		heartbeat: func(context.Context, Lease, time.Duration) error { return nil },
		complete: func(_ context.Context, lease Lease) (Job, error) {
			if lease != job.Lease {
				t.Fatalf("complete lease = %+v", lease)
			}
			cancel()
			return Job{State: StateSucceeded}, nil
		},
		fail: func(context.Context, Lease, Failure) (Job, error) { t.Fatal("unexpected fail"); return Job{}, nil },
	}
	handled := 0
	worker := validWorker(store, func(_ context.Context, got Job) error {
		handled++
		if got.ID != job.ID {
			t.Fatalf("handler job = %+v", got)
		}
		return nil
	})
	if err := worker.Run(ctx); !errors.Is(err, context.Canceled) || claims != 1 || handled != 1 {
		t.Fatalf("Run() = %v, claims=%d handled=%d", err, claims, handled)
	}
}

func TestWorkerRunPreservesUnknownClaimOutcomeForReconciliation(t *testing.T) {
	job := claimedJob(1)
	claimFailure := errors.Join(ErrCommitOutcomeUnknown, errors.New("commit connection lost"))
	handled := false
	store := &stubStore{claim: func(context.Context, ClaimRequest) (Job, error) {
		return job, claimFailure
	}}
	worker := validWorker(store, func(context.Context, Job) error {
		handled = true
		return nil
	})

	err := worker.Run(context.Background())
	if !errors.Is(err, ErrCommitOutcomeUnknown) || !errors.Is(err, claimFailure) {
		t.Fatalf("Run() = %v, want original unknown-commit error", err)
	}
	var reconciliation *ClaimReconciliationError
	if !errors.As(err, &reconciliation) {
		t.Fatalf("Run() error %T does not expose reconciliation job", err)
	}
	got := reconciliation.ReconciliationJob()
	if got.ID != job.ID || got.Lease.Token != job.Lease.Token {
		t.Fatalf("reconciliation job ID/token = %q/%q", got.ID, got.Lease.Token)
	}
	if strings.Contains(err.Error(), job.Lease.Token) {
		t.Fatal("Run() error text leaks lease token")
	}
	if handled {
		t.Fatal("handler ran for an unconfirmed claim")
	}
}

func TestWorkerPreservesUnknownAcknowledgementOutcomes(t *testing.T) {
	job := claimedJob(1)
	unknown := errors.Join(ErrCommitOutcomeUnknown, errors.New("connection lost after commit"))
	tests := []struct {
		name      string
		handler   Handler
		configure func(*stubStore, *int)
		wantState State
	}{
		{
			name: "heartbeat", wantState: StateRunning,
			handler: func(ctx context.Context, _ Job) error {
				<-ctx.Done()
				return ctx.Err()
			},
			configure: func(store *stubStore, calls *int) {
				store.heartbeat = func(context.Context, Lease, time.Duration) error {
					*calls++
					return unknown
				}
			},
		},
		{
			name: "complete", wantState: StateSucceeded,
			handler: func(context.Context, Job) error { return nil },
			configure: func(store *stubStore, calls *int) {
				store.complete = func(context.Context, Lease) (Job, error) {
					*calls++
					result := job
					result.State = StateSucceeded
					result.Payload = []byte("complete-result")
					return result, unknown
				}
			},
		},
		{
			name: "fail", wantState: StatePending,
			handler: func(context.Context, Job) error { return errors.New("handler failed") },
			configure: func(store *stubStore, calls *int) {
				store.fail = func(context.Context, Lease, Failure) (Job, error) {
					*calls++
					result := job
					result.State = StatePending
					result.Payload = []byte("fail-result")
					return result, unknown
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			store := &stubStore{
				heartbeat: func(context.Context, Lease, time.Duration) error { return nil },
				complete:  func(context.Context, Lease) (Job, error) { return Job{}, errors.New("unexpected complete") },
				fail:      func(context.Context, Lease, Failure) (Job, error) { return Job{}, errors.New("unexpected fail") },
			}
			test.configure(store, &calls)
			worker := validWorker(store, test.handler)
			worker.HeartbeatInterval = time.Millisecond

			err := worker.runAttempt(context.Background(), job)
			if !errors.Is(err, ErrCommitOutcomeUnknown) || !errors.Is(err, unknown) {
				t.Fatalf("runAttempt() = %v, want original unknown-commit error", err)
			}
			var reconciliation *LeaseReconciliationError
			if !errors.As(err, &reconciliation) {
				t.Fatalf("runAttempt() error %T has no lease reconciliation accessors", err)
			}
			got := reconciliation.ReconciliationJob()
			if got.ID != job.ID || got.State != test.wantState {
				t.Fatalf("reconciliation job = %+v, want ID %s state %s", got, job.ID, test.wantState)
			}
			if reconciliation.ReconciliationLease() != job.Lease {
				t.Fatalf("reconciliation lease = %+v, want %+v", reconciliation.ReconciliationLease(), job.Lease)
			}
			if strings.Contains(err.Error(), job.ID) || strings.Contains(err.Error(), job.Lease.Token) {
				t.Fatalf("reconciliation error leaks identity or token: %q", err)
			}
			if calls != 1 {
				t.Fatalf("acknowledgement calls = %d, want 1", calls)
			}
		})
	}
}

func TestWorkerAttemptClassifiesRetryPermanentAndPanic(t *testing.T) {
	job := claimedJob(2)
	tests := []struct {
		name          string
		handler       Handler
		wantPermanent bool
		wantMessage   string
		wantDelay     time.Duration
	}{
		{name: "retry", handler: func(context.Context, Job) error { return errors.New("temporary") }, wantMessage: "temporary", wantDelay: 2 * time.Second},
		{name: "permanent", handler: func(context.Context, Job) error { return Permanent(errors.New("invalid destination")) }, wantPermanent: true, wantMessage: "invalid destination"},
		{name: "panic", handler: func(context.Context, Job) error { panic("secret panic value") }, wantMessage: "handler panicked", wantDelay: 2 * time.Second},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got Failure
			store := &stubStore{
				heartbeat: func(context.Context, Lease, time.Duration) error { return nil },
				complete:  func(context.Context, Lease) (Job, error) { t.Fatal("unexpected complete"); return Job{}, nil },
				fail: func(_ context.Context, _ Lease, failure Failure) (Job, error) {
					got = failure
					return Job{State: StatePending}, nil
				},
			}
			worker := validWorker(store, test.handler)
			if err := worker.runAttempt(context.Background(), job); err != nil {
				t.Fatalf("runAttempt() = %v", err)
			}
			if got.Permanent != test.wantPermanent || got.Message != test.wantMessage || got.RetryAfter != test.wantDelay {
				t.Fatalf("failure = %+v", got)
			}
		})
	}
}

func TestWorkerHeartbeatCancellationCancelsCooperativeHandler(t *testing.T) {
	job := claimedJob(1)
	handlerCanceled := make(chan struct{})
	store := &stubStore{
		heartbeat: func(context.Context, Lease, time.Duration) error { return ErrCanceled },
		complete:  func(context.Context, Lease) (Job, error) { t.Fatal("unexpected complete"); return Job{}, nil },
		fail:      func(context.Context, Lease, Failure) (Job, error) { t.Fatal("unexpected fail"); return Job{}, nil },
	}
	worker := validWorker(store, func(ctx context.Context, _ Job) error {
		<-ctx.Done()
		close(handlerCanceled)
		return ctx.Err()
	})
	if err := worker.runAttempt(context.Background(), job); err != nil {
		t.Fatalf("runAttempt(canceled) = %v", err)
	}
	select {
	case <-handlerCanceled:
	default:
		t.Fatal("handler context was not canceled")
	}
}

func TestWorkerHandlerCompletionCancelsBlockedHeartbeat(t *testing.T) {
	job := claimedJob(1)
	heartbeatStarted := make(chan struct{})
	completeCalls := 0
	store := &stubStore{
		heartbeat: func(ctx context.Context, _ Lease, _ time.Duration) error {
			close(heartbeatStarted)
			<-ctx.Done()
			return ctx.Err()
		},
		complete: func(context.Context, Lease) (Job, error) {
			completeCalls++
			return Job{State: StateSucceeded}, nil
		},
		fail: func(context.Context, Lease, Failure) (Job, error) {
			t.Fatal("unexpected fail")
			return Job{}, nil
		},
	}
	worker := validWorker(store, func(context.Context, Job) error {
		<-heartbeatStarted
		return nil
	})
	worker.HeartbeatInterval = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- worker.runAttempt(ctx, job)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runAttempt() = %v", err)
		}
	case <-time.After(250 * time.Millisecond):
		cancel()
		err := <-done
		t.Fatalf("runAttempt blocked joining heartbeat; cleanup returned %v", err)
	}
	if completeCalls != 1 {
		t.Fatalf("Complete calls = %d, want 1", completeCalls)
	}
}

func TestWorkerUnknownHeartbeatOutranksParentCancellation(t *testing.T) {
	job := claimedJob(1)
	heartbeatStarted := make(chan struct{})
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	completeCalls := 0
	failCalls := 0
	store := &stubStore{
		heartbeat: func(ctx context.Context, _ Lease, _ time.Duration) error {
			close(heartbeatStarted)
			<-ctx.Done()
			return errors.Join(ErrCommitOutcomeUnknown, ctx.Err())
		},
		complete: func(context.Context, Lease) (Job, error) {
			completeCalls++
			return Job{}, nil
		},
		fail: func(context.Context, Lease, Failure) (Job, error) {
			failCalls++
			return Job{}, nil
		},
	}
	worker := validWorker(store, func(ctx context.Context, _ Job) error {
		<-heartbeatStarted
		cancelParent()
		<-ctx.Done()
		return ctx.Err()
	})
	worker.HeartbeatInterval = time.Millisecond

	err := worker.runAttempt(parent, job)
	assertLeaseReconciliationError(t, err, job)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runAttempt() = %v, want joined parent cancellation", err)
	}
	if completeCalls != 0 || failCalls != 0 {
		t.Fatalf("post-unknown acknowledgements = complete %d, fail %d", completeCalls, failCalls)
	}
}

func TestWorkerUnknownHeartbeatOutranksLocalTeardownCancellation(t *testing.T) {
	job := claimedJob(1)
	heartbeatStarted := make(chan struct{})
	completeCalls := 0
	failCalls := 0
	store := &stubStore{
		heartbeat: func(ctx context.Context, _ Lease, _ time.Duration) error {
			close(heartbeatStarted)
			<-ctx.Done()
			return errors.Join(ErrCommitOutcomeUnknown, ctx.Err())
		},
		complete: func(context.Context, Lease) (Job, error) {
			completeCalls++
			return Job{}, nil
		},
		fail: func(context.Context, Lease, Failure) (Job, error) {
			failCalls++
			return Job{}, nil
		},
	}
	worker := validWorker(store, func(context.Context, Job) error {
		<-heartbeatStarted
		return nil
	})
	worker.HeartbeatInterval = time.Millisecond

	err := worker.runAttempt(context.Background(), job)
	assertLeaseReconciliationError(t, err, job)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runAttempt() = %v, want joined local teardown cancellation", err)
	}
	if completeCalls != 0 || failCalls != 0 {
		t.Fatalf("post-unknown acknowledgements = complete %d, fail %d", completeCalls, failCalls)
	}
}

func assertLeaseReconciliationError(t *testing.T, err error, job Job) {
	t.Helper()
	if !errors.Is(err, ErrCommitOutcomeUnknown) {
		t.Fatalf("runAttempt() = %v, want ErrCommitOutcomeUnknown", err)
	}
	var reconciliation *LeaseReconciliationError
	if !errors.As(err, &reconciliation) {
		t.Fatalf("runAttempt() error %T has no lease reconciliation accessors", err)
	}
	if reconciliation.ReconciliationJob().ID != job.ID || reconciliation.ReconciliationLease() != job.Lease {
		t.Fatalf("reconciliation identity = %+v/%+v, want %s/%+v", reconciliation.ReconciliationJob(), reconciliation.ReconciliationLease(), job.ID, job.Lease)
	}
	if strings.Contains(err.Error(), job.ID) || strings.Contains(err.Error(), job.Lease.Token) {
		t.Fatalf("reconciliation error leaks identity or token: %q", err)
	}
}

func TestWorkerBoundsFailureText(t *testing.T) {
	job := claimedJob(1)
	var got Failure
	store := &stubStore{
		heartbeat: func(context.Context, Lease, time.Duration) error { return nil },
		complete:  func(context.Context, Lease) (Job, error) { return Job{}, errors.New("unexpected") },
		fail: func(_ context.Context, _ Lease, failure Failure) (Job, error) {
			got = failure
			return Job{}, nil
		},
	}
	worker := validWorker(store, func(context.Context, Job) error {
		return errors.New(strings.Repeat("é", MaxFailureBytes) + "\x00")
	})
	if err := worker.runAttempt(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if len(got.Message) > MaxFailureBytes || strings.ContainsRune(got.Message, 0) || !strings.HasSuffix(got.Message, "…") {
		t.Fatalf("bounded failure is invalid: bytes=%d suffix=%q", len(got.Message), got.Message[len(got.Message)-3:])
	}
}

func TestWorkerPollsWithoutBusyLoopAndValidatesConfiguration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Millisecond)
	defer cancel()
	var mutex sync.Mutex
	claims := 0
	store := &stubStore{claim: func(context.Context, ClaimRequest) (Job, error) {
		mutex.Lock()
		claims++
		mutex.Unlock()
		return Job{}, ErrNoJob
	}}
	worker := validWorker(store, func(context.Context, Job) error { return nil })
	if err := worker.Run(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run(empty) = %v", err)
	}
	mutex.Lock()
	defer mutex.Unlock()
	if claims < 1 || claims > 20 {
		t.Fatalf("claim calls = %d, want bounded polling", claims)
	}

	invalid := []Worker{
		{},
		validWorker(nil, func(context.Context, Job) error { return nil }),
		validWorker(store, nil),
	}
	badHeartbeat := validWorker(store, func(context.Context, Job) error { return nil })
	badHeartbeat.HeartbeatInterval = badHeartbeat.LeaseDuration
	invalid = append(invalid, badHeartbeat)
	tooLittleMargin := validWorker(store, func(context.Context, Job) error { return nil })
	tooLittleMargin.HeartbeatInterval = tooLittleMargin.LeaseDuration/2 + time.Microsecond
	invalid = append(invalid, tooLittleMargin)
	for _, candidate := range invalid {
		if err := candidate.validate(); !errors.Is(err, ErrInvalid) {
			t.Fatalf("validate(%+v) = %v", candidate, err)
		}
	}
	atBoundary := validWorker(store, func(context.Context, Job) error { return nil })
	atBoundary.HeartbeatInterval = atBoundary.LeaseDuration / 2
	if err := atBoundary.validate(); err != nil {
		t.Fatalf("half-lease heartbeat margin rejected: %v", err)
	}
}

func TestWorkerRejectsInvalidClaimedAttempt(t *testing.T) {
	store := &stubStore{claim: func(context.Context, ClaimRequest) (Job, error) {
		return Job{Queue: "default", State: StatePending}, nil
	}}
	worker := validWorker(store, func(context.Context, Job) error { return nil })
	if err := worker.Run(context.Background()); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Run(invalid claim) = %v", err)
	}
}

func TestWorkerRejectsClaimedAttemptTimestampAndPayloadShapes(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	tests := []struct {
		name   string
		mutate func(*Job)
	}{
		{name: "NULL payload", mutate: func(job *Job) { job.Payload = nil }},
		{name: "missing available", mutate: func(job *Job) { job.AvailableAt = time.Time{} }},
		{name: "non UTC created", mutate: func(job *Job) { job.CreatedAt = now.In(time.FixedZone("not-utc", 3600)) }},
		{name: "out of range updated", mutate: func(job *Job) { job.UpdatedAt = maximumPostgreSQLTimestamp.Add(time.Microsecond) }},
		{name: "sub-microsecond lease", mutate: func(job *Job) { job.LeaseUntil = now.Add(time.Nanosecond) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			job := claimedJob(1)
			test.mutate(&job)
			handled := false
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			store := &stubStore{
				claim:     func(context.Context, ClaimRequest) (Job, error) { return job, nil },
				heartbeat: func(context.Context, Lease, time.Duration) error { return nil },
				complete: func(context.Context, Lease) (Job, error) {
					cancel()
					return job, nil
				},
				fail: func(context.Context, Lease, Failure) (Job, error) { return job, nil },
			}
			worker := validWorker(store, func(context.Context, Job) error { handled = true; return nil })
			if err := worker.Run(ctx); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Run(%s) = %v, want ErrInvalid", test.name, err)
			}
			if handled {
				t.Fatal("handler ran for invalid claimed job")
			}
		})
	}
}

func TestWorkerRunAndAttemptFailurePaths(t *testing.T) {
	ordinary := errors.New("store failure")
	store := &stubStore{claim: func(context.Context, ClaimRequest) (Job, error) { return Job{}, ordinary }}
	worker := validWorker(store, func(context.Context, Job) error { return nil })
	if err := worker.Run(context.Background()); !errors.Is(err, ordinary) {
		t.Fatalf("Run(claim failure) = %v", err)
	}

	job := claimedJob(1)
	tests := []struct {
		name      string
		handler   Handler
		heartbeat func(context.Context, Lease, time.Duration) error
		complete  func(context.Context, Lease) (Job, error)
		fail      func(context.Context, Lease, Failure) (Job, error)
		want      error
	}{
		{
			name:      "lease lost heartbeat",
			handler:   func(ctx context.Context, _ Job) error { <-ctx.Done(); return ctx.Err() },
			heartbeat: func(context.Context, Lease, time.Duration) error { return ErrLeaseLost },
			complete:  func(context.Context, Lease) (Job, error) { return Job{}, nil },
			fail:      func(context.Context, Lease, Failure) (Job, error) { return Job{}, nil },
			want:      ErrLeaseLost,
		},
		{
			name:      "heartbeat context cancellation",
			handler:   func(ctx context.Context, _ Job) error { <-ctx.Done(); return ctx.Err() },
			heartbeat: func(context.Context, Lease, time.Duration) error { return context.Canceled },
			complete:  func(context.Context, Lease) (Job, error) { return Job{}, nil },
			fail:      func(context.Context, Lease, Failure) (Job, error) { return Job{}, nil },
			want:      context.Canceled,
		},
		{
			name: "complete canceled", handler: func(context.Context, Job) error { return nil },
			heartbeat: func(context.Context, Lease, time.Duration) error { return nil },
			complete:  func(context.Context, Lease) (Job, error) { return Job{}, ErrCanceled },
			fail:      func(context.Context, Lease, Failure) (Job, error) { return Job{}, nil },
		},
		{
			name: "complete failure", handler: func(context.Context, Job) error { return nil },
			heartbeat: func(context.Context, Lease, time.Duration) error { return nil },
			complete:  func(context.Context, Lease) (Job, error) { return Job{}, ordinary },
			fail:      func(context.Context, Lease, Failure) (Job, error) { return Job{}, nil },
			want:      ordinary,
		},
		{
			name: "fail canceled", handler: func(context.Context, Job) error { return ordinary },
			heartbeat: func(context.Context, Lease, time.Duration) error { return nil },
			complete:  func(context.Context, Lease) (Job, error) { return Job{}, nil },
			fail:      func(context.Context, Lease, Failure) (Job, error) { return Job{}, ErrCanceled },
		},
		{
			name: "fail failure", handler: func(context.Context, Job) error { return ordinary },
			heartbeat: func(context.Context, Lease, time.Duration) error { return nil },
			complete:  func(context.Context, Lease) (Job, error) { return Job{}, nil },
			fail:      func(context.Context, Lease, Failure) (Job, error) { return Job{}, ordinary },
			want:      ordinary,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &stubStore{heartbeat: test.heartbeat, complete: test.complete, fail: test.fail}
			worker := validWorker(store, test.handler)
			err := worker.runAttempt(context.Background(), job)
			if !errors.Is(err, test.want) {
				t.Fatalf("runAttempt() = %v, want %v", err, test.want)
			}
		})
	}
}

func TestWorkerContextAndConfigurationEdges(t *testing.T) {
	job := claimedJob(1)
	store := &stubStore{
		heartbeat: func(context.Context, Lease, time.Duration) error { return nil },
		complete:  func(context.Context, Lease) (Job, error) { return job, nil },
		fail:      func(context.Context, Lease, Failure) (Job, error) { return job, nil },
	}
	worker := validWorker(store, func(context.Context, Job) error { return nil })
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := worker.runAttempt(canceled, job); !errors.Is(err, context.Canceled) {
		t.Fatalf("runAttempt(canceled) = %v", err)
	}
	stop := make(chan struct{})
	if err := worker.heartbeat(canceled, stop, job.Lease); !errors.Is(err, context.Canceled) {
		t.Fatalf("heartbeat(canceled) = %v", err)
	}

	badPoll := worker
	badPoll.PollInterval = time.Minute + 1
	badRetry := worker
	badRetry.RetryPolicy = RetryPolicy{Initial: time.Second, Maximum: 0}
	badQueue := worker
	badQueue.Queue = "bad queue"
	badRetryPrecision := worker
	badRetryPrecision.RetryPolicy = RetryPolicy{Initial: time.Nanosecond, Maximum: time.Microsecond}
	for _, candidate := range []Worker{badPoll, badRetry, badQueue, badRetryPrecision} {
		if err := candidate.validate(); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid worker accepted: %+v, %v", candidate, err)
		}
	}
	invalidLease := job
	invalidLease.Lease.Token = "bad"
	if err := validateClaimedAttempt(invalidLease, "default", "worker-1"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("validateClaimedAttempt(bad lease) = %v", err)
	}
	wrongOwner := job
	wrongOwner.LeaseOwner = "worker-2"
	if err := validateClaimedAttempt(wrongOwner, "default", "worker-1"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("validateClaimedAttempt(wrong owner) = %v", err)
	}
}

var _ Store = (*stubStore)(nil)
