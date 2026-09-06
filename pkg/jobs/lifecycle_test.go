package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func runningJobRow(id, token string, attempts int) stubRow {
	now := time.Unix(1_900_000_000, 0).UTC()
	return stubRow{values: []any{
		id, "default", "send", []byte("payload"), nil, string(StateRunning), attempts,
		3, now, now, now, token, "worker-1", now.Add(time.Minute), "", nil,
	}}
}

func terminalJobRow(id string, state State, attempts int, message string) stubRow {
	now := time.Unix(1_900_000_000, 0).UTC()
	return stubRow{values: []any{
		id, "default", "send", []byte("payload"), nil, string(state), attempts,
		3, now, now, now, nil, nil, nil, message, now,
	}}
}

func pendingJobRow(id string, attempts int, message string) stubRow {
	now := time.Unix(1_900_000_000, 0).UTC()
	return stubRow{values: []any{
		id, "default", "send", []byte("payload"), nil, string(StatePending), attempts,
		3, now.Add(time.Minute), now, now, nil, nil, nil, message, nil,
	}}
}

func TestClaimReturnsOneFencedAttempt(t *testing.T) {
	token := strings.Repeat("ab", 32)
	tx := &stubTx{rows: []pgx.Row{runningJobRow("0123456789abcdef0123456789abcdef", token, 1)}}
	repository, _ := NewPostgreSQL(&stubDatabase{tx: tx})

	job, err := repository.claimWithToken(context.Background(), ClaimRequest{Queue: "default", Worker: "worker-1", LeaseDuration: time.Minute}, token)
	if err != nil || job.State != StateRunning || job.Attempts != 1 || job.Lease.Token != token || tx.commits != 1 {
		t.Fatalf("claimWithToken() = (%+v, %v), commits=%d", job, err, tx.commits)
	}
	if len(tx.arguments) != 1 || tx.arguments[0][1] != token || tx.arguments[0][3] != int64(time.Minute/time.Microsecond) {
		t.Fatalf("claim arguments = %+v", tx.arguments)
	}
}

func TestClaimPreservesFencingHandleOnUnknownCommit(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	token := strings.Repeat("ab", 32)
	commitFailure := errors.New("connection lost after commit")
	tx := &stubTx{
		rows:      []pgx.Row{runningJobRow(id, token, 1)},
		commitErr: commitFailure,
	}
	repository, _ := NewPostgreSQL(&stubDatabase{tx: tx})

	job, err := repository.claimWithToken(context.Background(), ClaimRequest{
		Queue: "default", Worker: "worker-1", LeaseDuration: time.Minute,
	}, token)
	if job.ID != id || job.Lease.JobID != id || job.Lease.Token != token {
		t.Fatalf("claim reconciliation handle = %+v", job)
	}
	if !errors.Is(err, ErrCommitOutcomeUnknown) || !errors.Is(err, commitFailure) {
		t.Fatalf("claim commit error = %v", err)
	}
}

func TestClaimNoJobAndInputFailures(t *testing.T) {
	tx := &stubTx{rows: []pgx.Row{stubRow{err: pgx.ErrNoRows}}}
	repository, _ := NewPostgreSQL(&stubDatabase{tx: tx})
	if _, err := repository.Claim(context.Background(), ClaimRequest{Queue: "default", Worker: "worker", LeaseDuration: time.Minute}); !errors.Is(err, ErrNoJob) || tx.commits != 1 {
		t.Fatalf("Claim(empty) = %v, commits=%d", err, tx.commits)
	}
	if _, err := repository.Claim(nil, ClaimRequest{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Claim(nil) = %v", err)
	}
}

func TestHeartbeatCompleteAndFailTransitions(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	token := strings.Repeat("ab", 32)
	lease := Lease{JobID: id, Token: token}
	tests := []struct {
		name      string
		row       stubRow
		operation func(*PostgreSQL) (Job, error)
		wantState State
	}{
		{name: "heartbeat", row: runningJobRow(id, token, 1), operation: func(r *PostgreSQL) (Job, error) { return r.Heartbeat(context.Background(), lease, time.Minute) }, wantState: StateRunning},
		{name: "complete", row: terminalJobRow(id, StateSucceeded, 1, ""), operation: func(r *PostgreSQL) (Job, error) { return r.Complete(context.Background(), lease) }, wantState: StateSucceeded},
		{name: "retry", row: pendingJobRow(id, 1, "temporary"), operation: func(r *PostgreSQL) (Job, error) {
			return r.Fail(context.Background(), lease, Failure{Message: "temporary", RetryAfter: time.Second})
		}, wantState: StatePending},
		{name: "dead", row: terminalJobRow(id, StateDead, 1, "permanent"), operation: func(r *PostgreSQL) (Job, error) {
			return r.Fail(context.Background(), lease, Failure{Message: "permanent", Permanent: true})
		}, wantState: StateDead},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := &stubTx{rows: []pgx.Row{test.row}}
			repository, _ := NewPostgreSQL(&stubDatabase{tx: tx})
			job, err := test.operation(repository)
			if err != nil || job.State != test.wantState || tx.commits != 1 {
				t.Fatalf("operation = (%+v, %v), commits=%d", job, err, tx.commits)
			}
		})
	}
}

func TestLeaseTransitionClassifiesCanceledMissingAndLost(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	token := strings.Repeat("ab", 32)
	lease := Lease{JobID: id, Token: token}
	tests := []struct {
		name     string
		stateRow pgx.Row
		want     error
	}{
		{name: "missing", stateRow: stubRow{err: pgx.ErrNoRows}, want: ErrNotFound},
		{name: "canceled", stateRow: stubRow{values: []any{string(StateCanceled)}}, want: ErrCanceled},
		{name: "replaced or expired", stateRow: stubRow{values: []any{string(StateRunning)}}, want: ErrLeaseLost},
		{name: "terminal", stateRow: stubRow{values: []any{string(StateSucceeded)}}, want: ErrLeaseLost},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := &stubTx{rows: []pgx.Row{stubRow{err: pgx.ErrNoRows}, test.stateRow}}
			repository, _ := NewPostgreSQL(&stubDatabase{tx: tx})
			if _, err := repository.Complete(context.Background(), lease); !errors.Is(err, test.want) || tx.commits != 0 {
				t.Fatalf("Complete() = %v, want %v; commits=%d", err, test.want, tx.commits)
			}
		})
	}
}

func TestLifecycleRejectsMalformedLeaseAndFailure(t *testing.T) {
	repository, _ := NewPostgreSQL(&stubDatabase{tx: &stubTx{}})
	badLeases := []Lease{{}, {JobID: "bad", Token: strings.Repeat("ab", 32)}, {JobID: "0123456789abcdef0123456789abcdef", Token: "bad"}}
	for _, lease := range badLeases {
		if _, err := repository.Complete(context.Background(), lease); !errors.Is(err, ErrInvalid) {
			t.Fatalf("Complete(%+v) = %v", lease, err)
		}
	}
	validLease := Lease{JobID: "0123456789abcdef0123456789abcdef", Token: strings.Repeat("ab", 32)}
	if _, err := repository.Heartbeat(context.Background(), validLease, time.Second-1); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Heartbeat(short) = %v", err)
	}
	if _, err := repository.Fail(context.Background(), validLease, Failure{RetryAfter: MaxRetryDelay + 1}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Fail(long retry) = %v", err)
	}
}
