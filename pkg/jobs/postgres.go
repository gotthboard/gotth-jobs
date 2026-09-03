package jobs

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
)

const rollbackTimeout = 5 * time.Second

// Database is the exact pgx-compatible PostgreSQL surface required by the
// library. Both pgx.Conn and pgxpool.Pool satisfy it.
type Database interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// PostgreSQL is the durable PostgreSQL 17 job repository.
type PostgreSQL struct {
	database Database
}

// NewPostgreSQL binds the durable queue to one pgx-compatible PostgreSQL
// database without opening connections or applying migrations.
//
// Complexity: time O(1), Omega(1), tight Theta(1); auxiliary space O(1),
// Omega(1), tight Theta(1).
func NewPostgreSQL(database Database) (*PostgreSQL, error) {
	if nilLike(database) {
		return nil, fmt.Errorf("%w: database is required", ErrInvalid)
	}
	return &PostgreSQL{database: database}, nil
}

// nilLike recognizes nil interfaces and interfaces containing nil pointer-like
// values so constructors fail before the first database call.
//
// Complexity: time O(1), Omega(1), tight Theta(1); auxiliary space O(1),
// Omega(1), tight Theta(1); delegated reflect inspection is constant here.
func nilLike(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

// transact executes one explicit transaction, never retries, and classifies a
// commit error as an unknown outcome.
//
// Complexity: local time and space are tight Theta(1); total time is
// B+F+C+R and total auxiliary space is S_F, where B, F, C, R, and S_F are the
// delegated begin, body, commit, rollback, and body-space costs.
func transact[T any](ctx context.Context, database Database, operation func(pgx.Tx) (T, error)) (value T, err error) {
	if ctx == nil {
		return value, fmt.Errorf("%w: context is required", ErrInvalid)
	}
	transaction, beginErr := database.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadWrite,
	})
	if beginErr != nil {
		return value, fmt.Errorf("begin job transaction: %w", beginErr)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
		defer cancel()
		rollbackErr := transaction.Rollback(cleanup)
		if err != nil && rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback job transaction: %w", rollbackErr))
		}
	}()

	value, err = operation(transaction)
	if err != nil {
		return value, err
	}
	if commitErr := transaction.Commit(ctx); commitErr != nil {
		return value, errors.Join(ErrCommitOutcomeUnknown, fmt.Errorf("commit job transaction: %w", commitErr))
	}
	return value, nil
}
