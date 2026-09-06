package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestStoredJobValidationRejectsEveryImpossibleShape(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	valid := Job{
		ID: "0123456789abcdef0123456789abcdef", Queue: "default", Kind: "send",
		Payload: []byte("payload"), State: StatePending, MaxAttempts: 3,
		AvailableAt: now, CreatedAt: now, UpdatedAt: now,
	}
	mutations := []func(*Job){
		func(job *Job) { job.ID = strings.Repeat("g", 32) },
		func(job *Job) { job.Queue = "bad queue" },
		func(job *Job) { job.Kind = "bad kind" },
		func(job *Job) { job.Payload = make([]byte, MaxPayloadBytes+1) },
		func(job *Job) { job.IdempotencyKey = strings.Repeat("i", MaxIdempotencyKeyBytes+1) },
		func(job *Job) { job.LastError = strings.Repeat("e", MaxFailureBytes+1) },
		func(job *Job) { job.LastError = string([]byte{0xff}) },
		func(job *Job) { job.Attempts = 4 },
		func(job *Job) { job.MaxAttempts = 0 },
		func(job *Job) { job.State = "unknown" },
		func(job *Job) { job.Lease = Lease{JobID: job.ID, Token: strings.Repeat("ab", 32)} },
		func(job *Job) { job.State = StateRunning },
		func(job *Job) { job.State = StateSucceeded },
	}
	for index, mutate := range mutations {
		job := valid
		job.Payload = append([]byte(nil), valid.Payload...)
		mutate(&job)
		if err := validateStoredJob(job); err == nil {
			t.Fatalf("mutation %d was accepted: %+v", index, job)
		}
	}
	running := valid
	running.State = StateRunning
	running.Attempts = 1
	running.Lease = Lease{JobID: running.ID, Token: strings.Repeat("ab", 32)}
	running.LeaseOwner = "worker"
	running.LeaseUntil = now.Add(time.Minute)
	if err := validateStoredJob(running); err != nil {
		t.Fatalf("valid running job rejected: %v", err)
	}
	terminal := valid
	terminal.State = StateDead
	terminal.Attempts = 1
	terminal.FinishedAt = now
	if err := validateStoredJob(terminal); err != nil {
		t.Fatalf("valid terminal job rejected: %v", err)
	}
}

func TestStoredJobValidationStateAttemptBoundaries(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	tests := []struct {
		name     string
		state    State
		attempts int
		valid    bool
	}{
		{name: "pending zero", state: StatePending, attempts: 0, valid: true},
		{name: "pending one", state: StatePending, attempts: 1, valid: true},
		{name: "pending below maximum", state: StatePending, attempts: 2, valid: true},
		{name: "pending at maximum", state: StatePending, attempts: 3},
		{name: "running zero", state: StateRunning, attempts: 0},
		{name: "running one", state: StateRunning, attempts: 1, valid: true},
		{name: "running at maximum", state: StateRunning, attempts: 3, valid: true},
		{name: "succeeded zero", state: StateSucceeded, attempts: 0},
		{name: "succeeded one", state: StateSucceeded, attempts: 1, valid: true},
		{name: "succeeded at maximum", state: StateSucceeded, attempts: 3, valid: true},
		{name: "dead zero", state: StateDead, attempts: 0},
		{name: "dead one", state: StateDead, attempts: 1, valid: true},
		{name: "dead at maximum", state: StateDead, attempts: 3, valid: true},
		{name: "canceled zero", state: StateCanceled, attempts: 0, valid: true},
		{name: "canceled one", state: StateCanceled, attempts: 1, valid: true},
		{name: "canceled at maximum", state: StateCanceled, attempts: 3, valid: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			job := Job{
				ID: "0123456789abcdef0123456789abcdef", Queue: "default", Kind: "send",
				State: test.state, Attempts: test.attempts, MaxAttempts: 3,
				AvailableAt: now, CreatedAt: now, UpdatedAt: now,
			}
			switch test.state {
			case StateRunning:
				job.Lease = Lease{JobID: job.ID, Token: strings.Repeat("ab", 32)}
				job.LeaseOwner = "worker"
				job.LeaseUntil = now.Add(time.Minute)
			case StateSucceeded, StateDead, StateCanceled:
				job.FinishedAt = now
			}
			err := validateStoredJob(job)
			if test.valid && err != nil {
				t.Fatalf("validateStoredJob(%s, %d) = %v", test.state, test.attempts, err)
			}
			if !test.valid && err == nil {
				t.Fatalf("validateStoredJob(%s, %d) accepted an impossible row", test.state, test.attempts)
			}
		})
	}
}

func TestTransactionFailurePaths(t *testing.T) {
	failure := errors.New("failure")
	if _, err := transact(context.Background(), &stubDatabase{beginErr: failure}, func(pgx.Tx) (int, error) { return 0, nil }); !errors.Is(err, failure) {
		t.Fatalf("transact(begin) = %v", err)
	}
	tx := &stubTx{rollbackErr: failure}
	if _, err := transact(context.Background(), &stubDatabase{tx: tx}, func(pgx.Tx) (int, error) { return 0, ErrStateConflict }); !errors.Is(err, ErrStateConflict) || !strings.Contains(err.Error(), "rollback") {
		t.Fatalf("transact(operation+rollback) = %v", err)
	}
	if _, err := transact[int](nil, &stubDatabase{}, func(pgx.Tx) (int, error) { return 0, nil }); !errors.Is(err, ErrInvalid) {
		t.Fatalf("transact(nil) = %v", err)
	}
	if nilLike(1) || !nilLike((chan int)(nil)) {
		t.Fatal("nilLike scalar/channel classification is wrong")
	}
}

func TestEnqueueFailurePaths(t *testing.T) {
	request := EnqueueRequest{Queue: "default", Kind: "send", MaxAttempts: 1}
	failure := errors.New("failure")
	var nilRepository *PostgreSQL
	if _, _, err := nilRepository.Enqueue(context.Background(), request); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil Enqueue = %v", err)
	}
	tests := []struct {
		name    string
		request EnqueueRequest
		rows    []pgx.Row
		want    string
	}{
		{name: "insert", request: request, rows: []pgx.Row{stubRow{err: failure}}, want: "insert job"},
		{name: "unexpected no row", request: request, rows: []pgx.Row{stubRow{err: pgx.ErrNoRows}}, want: "without an idempotency key"},
		{name: "fingerprint read", request: EnqueueRequest{Queue: "default", Kind: "send", IdempotencyKey: "key", MaxAttempts: 1}, rows: []pgx.Row{stubRow{err: pgx.ErrNoRows}, stubRow{err: failure}}, want: "fingerprint"},
		{name: "idempotent job read", request: EnqueueRequest{Queue: "default", Kind: "send", IdempotencyKey: "key", MaxAttempts: 1}, rows: func() []pgx.Row {
			fingerprint := requestFingerprint(EnqueueRequest{Queue: "default", Kind: "send", IdempotencyKey: "key", MaxAttempts: 1})
			return []pgx.Row{stubRow{err: pgx.ErrNoRows}, stubRow{values: []any{fingerprint[:]}}, stubRow{err: failure}}
		}(), want: "read idempotent job"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := &stubTx{rows: test.rows}
			repository, _ := NewPostgreSQL(&stubDatabase{tx: tx})
			if _, _, err := repository.Enqueue(context.Background(), test.request); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Enqueue() = %v, want text %q", err, test.want)
			}
		})
	}
}

func TestLifecycleDatabaseFailurePaths(t *testing.T) {
	failure := errors.New("failure")
	validClaim := ClaimRequest{Queue: "default", Worker: "worker", LeaseDuration: time.Second}
	if _, err := (*PostgreSQL)(nil).Claim(context.Background(), validClaim); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil Claim = %v", err)
	}
	repository, _ := NewPostgreSQL(&stubDatabase{tx: &stubTx{rows: []pgx.Row{stubRow{err: failure}}}})
	if _, err := repository.claimWithToken(context.Background(), validClaim, strings.Repeat("ab", 32)); err == nil || !strings.Contains(err.Error(), "claim job") {
		t.Fatalf("claim query failure = %v", err)
	}
	if _, err := repository.claimWithToken(context.Background(), validClaim, "bad"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("claim bad token = %v", err)
	}

	lease := Lease{JobID: "0123456789abcdef0123456789abcdef", Token: strings.Repeat("ab", 32)}
	if _, err := (*PostgreSQL)(nil).Heartbeat(context.Background(), lease, time.Second); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil heartbeat = %v", err)
	}
	if _, err := (*PostgreSQL)(nil).Fail(context.Background(), lease, Failure{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil fail = %v", err)
	}
	tx := &stubTx{rows: []pgx.Row{stubRow{err: failure}}}
	repository, _ = NewPostgreSQL(&stubDatabase{tx: tx})
	if _, err := repository.Complete(context.Background(), lease); err == nil || !strings.Contains(err.Error(), "update job lease") {
		t.Fatalf("complete query failure = %v", err)
	}
	tx = &stubTx{rows: []pgx.Row{stubRow{err: pgx.ErrNoRows}, stubRow{err: failure}}}
	repository, _ = NewPostgreSQL(&stubDatabase{tx: tx})
	if _, err := repository.Complete(context.Background(), lease); err == nil || !strings.Contains(err.Error(), "classify") {
		t.Fatalf("complete classify failure = %v", err)
	}
	if _, err := repository.Heartbeat(context.Background(), lease, MaxLeaseDuration+1); !errors.Is(err, ErrInvalid) {
		t.Fatalf("heartbeat long lease = %v", err)
	}
	if _, err := repository.Heartbeat(context.Background(), lease, time.Second+time.Nanosecond); !errors.Is(err, ErrInvalid) {
		t.Fatalf("heartbeat sub-microsecond lease = %v", err)
	}
	if _, err := repository.Fail(context.Background(), lease, Failure{Permanent: true, RetryAfter: time.Second}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("fail invalid = %v", err)
	}
}

func TestReadAndOperationsFailurePaths(t *testing.T) {
	failure := errors.New("failure")
	id := "0123456789abcdef0123456789abcdef"
	if _, err := (*PostgreSQL)(nil).Get(context.Background(), id); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil Get = %v", err)
	}
	if _, err := (*PostgreSQL)(nil).Counts(context.Background(), "default"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil Counts = %v", err)
	}
	if _, err := (*PostgreSQL)(nil).Cancel(context.Background(), id); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil Cancel = %v", err)
	}
	repository, _ := NewPostgreSQL(&stubDatabase{directRows: []pgx.Row{stubRow{err: failure}}})
	if _, err := repository.Get(context.Background(), id); err == nil || !strings.Contains(err.Error(), "get job") {
		t.Fatalf("Get query failure = %v", err)
	}
	repository, _ = NewPostgreSQL(&stubDatabase{directRows: []pgx.Row{stubRow{err: failure}}})
	if _, err := repository.Counts(context.Background(), "default"); err == nil || !strings.Contains(err.Error(), "count jobs") {
		t.Fatalf("Counts query failure = %v", err)
	}
	repository, _ = NewPostgreSQL(&stubDatabase{queryErr: failure})
	if _, err := repository.ListDead(context.Background(), "default", nil, 1); err == nil || !strings.Contains(err.Error(), "list dead") {
		t.Fatalf("ListDead query failure = %v", err)
	}
	for _, rows := range []*stubRows{
		{rows: []stubRow{{err: failure}}},
		{rows: []stubRow{jobRow(id, EnqueueRequest{Queue: "default", Kind: "send", MaxAttempts: 1}, StatePending)}},
		{err: failure},
	} {
		repository, _ = NewPostgreSQL(&stubDatabase{rows: rows})
		if _, err := repository.ListDead(context.Background(), "default", nil, 1); err == nil {
			t.Fatal("ListDead accepted broken rows")
		}
	}

	for _, test := range []struct {
		name      string
		operation func(*PostgreSQL) (Job, error)
		rows      []pgx.Row
		want      error
	}{
		{name: "cancel update", operation: func(r *PostgreSQL) (Job, error) { return r.Cancel(context.Background(), id) }, rows: []pgx.Row{stubRow{err: failure}}},
		{name: "cancel missing", operation: func(r *PostgreSQL) (Job, error) { return r.Cancel(context.Background(), id) }, rows: []pgx.Row{stubRow{err: pgx.ErrNoRows}, stubRow{err: pgx.ErrNoRows}}, want: ErrNotFound},
		{name: "cancel fallback", operation: func(r *PostgreSQL) (Job, error) { return r.Cancel(context.Background(), id) }, rows: []pgx.Row{stubRow{err: pgx.ErrNoRows}, stubRow{err: failure}}},
		{name: "redrive update", operation: func(r *PostgreSQL) (Job, error) { return r.Redrive(context.Background(), id) }, rows: []pgx.Row{stubRow{err: failure}}},
		{name: "redrive missing", operation: func(r *PostgreSQL) (Job, error) { return r.Redrive(context.Background(), id) }, rows: []pgx.Row{stubRow{err: pgx.ErrNoRows}, stubRow{err: pgx.ErrNoRows}}, want: ErrNotFound},
		{name: "redrive fallback", operation: func(r *PostgreSQL) (Job, error) { return r.Redrive(context.Background(), id) }, rows: []pgx.Row{stubRow{err: pgx.ErrNoRows}, stubRow{err: failure}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository, _ := NewPostgreSQL(&stubDatabase{tx: &stubTx{rows: test.rows}})
			_, err := test.operation(repository)
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("operation = %v, want %v", err, test.want)
			}
			if test.want == nil && err == nil {
				t.Fatal("operation returned nil error")
			}
		})
	}
}
