package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type stubRows struct {
	pgx.Rows
	rows   []stubRow
	index  int
	err    error
	closed bool
}

func (rows *stubRows) Next() bool {
	return rows.index < len(rows.rows)
}

func (rows *stubRows) Scan(destinations ...any) error {
	if rows.index >= len(rows.rows) {
		return errors.New("scan past end")
	}
	err := rows.rows[rows.index].Scan(destinations...)
	rows.index++
	return err
}

func (rows *stubRows) Err() error { return rows.err }
func (rows *stubRows) Close()     { rows.closed = true }

func TestGetAndCountsReturnBoundedObservations(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	request := EnqueueRequest{Queue: "default", Kind: "send", Payload: []byte("payload"), MaxAttempts: 3, AvailableAt: time.Unix(1_900_000_000, 0).UTC()}
	database := &stubDatabase{directRows: []pgx.Row{
		jobRow(id, request, StatePending),
		stubRow{values: []any{int64(1), int64(2), int64(3), int64(4), int64(5)}},
	}}
	repository, _ := NewPostgreSQL(database)
	job, err := repository.Get(context.Background(), id)
	if err != nil || job.ID != id || string(job.Payload) != "payload" {
		t.Fatalf("Get() = (%+v, %v)", job, err)
	}
	counts, err := repository.Counts(context.Background(), "default")
	if err != nil || counts != (Counts{Pending: 1, Running: 2, Succeeded: 3, Dead: 4, Canceled: 5}) {
		t.Fatalf("Counts() = (%+v, %v)", counts, err)
	}
	if _, err := repository.Get(context.Background(), "bad"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Get(bad) = %v", err)
	}
	if _, err := repository.Counts(context.Background(), "bad queue"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Counts(bad) = %v", err)
	}
}

func TestGetMapsMissingJob(t *testing.T) {
	repository, _ := NewPostgreSQL(&stubDatabase{directRows: []pgx.Row{stubRow{err: pgx.ErrNoRows}}})
	if _, err := repository.Get(context.Background(), "0123456789abcdef0123456789abcdef"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(missing) = %v", err)
	}
}

func TestListDeadClosesRowsAndHonorsCursor(t *testing.T) {
	firstID := "0123456789abcdef0123456789abcdef"
	secondID := "1123456789abcdef0123456789abcdef"
	rows := &stubRows{rows: []stubRow{terminalJobRow(firstID, StateDead, 3, "one"), terminalJobRow(secondID, StateDead, 3, "two")}}
	database := &stubDatabase{rows: rows}
	repository, _ := NewPostgreSQL(database)
	cursor := &DeadCursor{FinishedAt: time.Unix(1_900_000_000, 0).UTC(), ID: strings.Repeat("a", 32)}
	jobs, err := repository.ListDead(context.Background(), "default", cursor, 2)
	if err != nil || len(jobs) != 2 || jobs[0].ID != firstID || jobs[1].ID != secondID || !rows.closed {
		t.Fatalf("ListDead() = (%+v, %v), closed=%t", jobs, err, rows.closed)
	}
	if len(database.queryArguments) != 1 || database.queryArguments[0][3] != 2 {
		t.Fatalf("ListDead arguments = %+v", database.queryArguments)
	}
	emptyRows := &stubRows{}
	repository, _ = NewPostgreSQL(&stubDatabase{rows: emptyRows})
	if jobs, err := repository.ListDead(context.Background(), "default", nil, MaxDeadPage); err != nil || len(jobs) != 0 {
		t.Fatalf("ListDead(maximum) = (%+v, %v)", jobs, err)
	}
	for _, invalid := range []struct {
		cursor *DeadCursor
		limit  int
	}{
		{limit: 0},
		{limit: MaxDeadPage + 1},
		{cursor: &DeadCursor{}, limit: 1},
		{cursor: &DeadCursor{FinishedAt: cursor.FinishedAt.Add(time.Nanosecond), ID: cursor.ID}, limit: 1},
	} {
		if _, err := repository.ListDead(context.Background(), "default", invalid.cursor, invalid.limit); !errors.Is(err, ErrInvalid) {
			t.Fatalf("ListDead(invalid) = %v", err)
		}
	}
}

func TestCancelAndRedriveStateTransitions(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	tests := []struct {
		name      string
		rows      []pgx.Row
		operation func(*PostgreSQL) (Job, error)
		wantState State
		wantErr   error
	}{
		{name: "cancel pending", rows: []pgx.Row{terminalJobRow(id, StateCanceled, 0, "")}, operation: func(r *PostgreSQL) (Job, error) { return r.Cancel(context.Background(), id) }, wantState: StateCanceled},
		{name: "cancel idempotent", rows: []pgx.Row{stubRow{err: pgx.ErrNoRows}, terminalJobRow(id, StateCanceled, 0, "")}, operation: func(r *PostgreSQL) (Job, error) { return r.Cancel(context.Background(), id) }, wantState: StateCanceled},
		{name: "cancel terminal conflict", rows: []pgx.Row{stubRow{err: pgx.ErrNoRows}, terminalJobRow(id, StateSucceeded, 1, "")}, operation: func(r *PostgreSQL) (Job, error) { return r.Cancel(context.Background(), id) }, wantErr: ErrStateConflict},
		{name: "redrive dead", rows: []pgx.Row{pendingJobRow(id, 0, "")}, operation: func(r *PostgreSQL) (Job, error) { return r.Redrive(context.Background(), id) }, wantState: StatePending},
		{name: "redrive conflict", rows: []pgx.Row{stubRow{err: pgx.ErrNoRows}, pendingJobRow(id, 1, "temporary")}, operation: func(r *PostgreSQL) (Job, error) { return r.Redrive(context.Background(), id) }, wantErr: ErrStateConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := &stubTx{rows: test.rows}
			repository, _ := NewPostgreSQL(&stubDatabase{tx: tx})
			job, err := test.operation(repository)
			if !errors.Is(err, test.wantErr) || (test.wantErr == nil && job.State != test.wantState) {
				t.Fatalf("operation = (%+v, %v), want state=%s error=%v", job, err, test.wantState, test.wantErr)
			}
			if test.wantErr == nil && tx.commits != 1 {
				t.Fatalf("successful operation committed %d times", tx.commits)
			}
		})
	}
}

var _ pgx.Rows = (*stubRows)(nil)
