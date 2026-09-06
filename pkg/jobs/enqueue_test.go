package jobs

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type stubRow struct {
	values []any
	err    error
}

func (row stubRow) Scan(destinations ...any) error {
	if row.err != nil {
		return row.err
	}
	if len(destinations) != len(row.values) {
		return errors.New("unexpected scan width")
	}
	for index, destination := range destinations {
		value := row.values[index]
		switch target := destination.(type) {
		case *string:
			*target = value.(string)
		case *[]byte:
			*target = append((*target)[:0], value.([]byte)...)
		case pgtype.BytesScanner:
			if err := target.ScanBytes(value.([]byte)); err != nil {
				return err
			}
		case *bool:
			*target = value.(bool)
		case *int:
			*target = value.(int)
		case *int64:
			*target = value.(int64)
		case *time.Time:
			*target = value.(time.Time)
		case **string:
			if value == nil {
				*target = nil
			} else {
				copy := value.(string)
				*target = &copy
			}
		case **time.Time:
			if value == nil {
				*target = nil
			} else {
				copy := value.(time.Time)
				*target = &copy
			}
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}

type stubTx struct {
	pgx.Tx
	rows        []pgx.Row
	queryRow    func(string, ...any) pgx.Row
	statements  []string
	arguments   [][]any
	commitErr   error
	rollbackErr error
	commits     int
	rollbacks   int
}

func (tx *stubTx) QueryRow(_ context.Context, statement string, arguments ...any) pgx.Row {
	tx.statements = append(tx.statements, statement)
	tx.arguments = append(tx.arguments, append([]any(nil), arguments...))
	if tx.queryRow != nil {
		return tx.queryRow(statement, arguments...)
	}
	if len(tx.rows) == 0 {
		return stubRow{err: errors.New("unexpected query")}
	}
	row := tx.rows[0]
	tx.rows = tx.rows[1:]
	return row
}

func (tx *stubTx) Commit(context.Context) error {
	tx.commits++
	return tx.commitErr
}

func (tx *stubTx) Rollback(context.Context) error {
	tx.rollbacks++
	return tx.rollbackErr
}

type stubDatabase struct {
	tx             *stubTx
	beginErr       error
	rows           pgx.Rows
	queryErr       error
	directRows     []pgx.Row
	statements     []string
	queryArguments [][]any
	beginOptions   []pgx.TxOptions
}

func (database *stubDatabase) BeginTx(_ context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	database.beginOptions = append(database.beginOptions, options)
	return database.tx, database.beginErr
}

func (database *stubDatabase) Query(_ context.Context, statement string, arguments ...any) (pgx.Rows, error) {
	database.statements = append(database.statements, statement)
	database.queryArguments = append(database.queryArguments, append([]any(nil), arguments...))
	return database.rows, database.queryErr
}

func (database *stubDatabase) QueryRow(_ context.Context, statement string, arguments ...any) pgx.Row {
	database.statements = append(database.statements, statement)
	database.queryArguments = append(database.queryArguments, append([]any(nil), arguments...))
	if len(database.directRows) == 0 {
		return stubRow{err: errors.New("unexpected query row")}
	}
	row := database.directRows[0]
	database.directRows = database.directRows[1:]
	return row
}

func jobRow(id string, request EnqueueRequest, state State) stubRow {
	now := time.Unix(1_900_000_000, 0).UTC()
	var key any
	if request.IdempotencyKey != "" {
		key = request.IdempotencyKey
	}
	return stubRow{values: []any{
		id, request.Queue, request.Kind, request.Payload, key, string(state), 0,
		request.MaxAttempts, request.AvailableAt, now, now, nil, nil, nil, "", nil,
	}}
}

func idempotentJobRow(fingerprint []byte, row stubRow) stubRow {
	return stubRow{values: append([]any{fingerprint}, row.values...)}
}

func TestEnqueueCommitsCreatedJobAndCopiesPayload(t *testing.T) {
	request := EnqueueRequest{Queue: "default", Kind: "send", Payload: []byte("payload"), MaxAttempts: 3, AvailableAt: time.Unix(1_900_000_000, 0).UTC()}
	tx := &stubTx{rows: []pgx.Row{jobRow("0123456789abcdef0123456789abcdef", request, StatePending)}}
	database := &stubDatabase{tx: tx}
	repository, err := NewPostgreSQL(database)
	if err != nil {
		t.Fatalf("NewPostgreSQL() = %v", err)
	}

	job, created, err := repository.Enqueue(context.Background(), request)
	if err != nil || !created || job.ID == "" || tx.commits != 1 {
		t.Fatalf("Enqueue() = (%+v, %t, %v), commits=%d", job, created, err, tx.commits)
	}
	request.Payload[0] = 'X'
	if string(job.Payload) != "payload" || reflect.DeepEqual(job.Payload, request.Payload) {
		t.Fatalf("returned payload aliases caller: %q", job.Payload)
	}
	if len(tx.arguments) != 1 || tx.arguments[0][0] == "" {
		t.Fatalf("insert arguments missing generated ID: %+v", tx.arguments)
	}
	if len(database.beginOptions) != 1 || database.beginOptions[0].IsoLevel != pgx.ReadCommitted || database.beginOptions[0].AccessMode != pgx.ReadWrite {
		t.Fatalf("transaction options = %+v", database.beginOptions)
	}
}

func TestEnqueueIdempotentDuplicateAndConflict(t *testing.T) {
	request := EnqueueRequest{Queue: "default", Kind: "send", Payload: []byte("payload"), IdempotencyKey: "key", MaxAttempts: 3, AvailableAt: time.Unix(1_900_000_000, 0).UTC()}
	fingerprint := requestFingerprint(request)
	for _, test := range []struct {
		name        string
		fingerprint []byte
		wantErr     error
	}{
		{name: "same", fingerprint: fingerprint[:]},
		{name: "different", fingerprint: make([]byte, len(fingerprint)), wantErr: ErrIdempotencyConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			tx := &stubTx{rows: []pgx.Row{
				stubRow{err: pgx.ErrNoRows},
				idempotentJobRow(
					test.fingerprint,
					jobRow("0123456789abcdef0123456789abcdef", request, StatePending),
				),
			}}
			repository, err := NewPostgreSQL(&stubDatabase{tx: tx})
			if err != nil {
				t.Fatal(err)
			}
			job, created, err := repository.Enqueue(context.Background(), request)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Enqueue() error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr == nil && (created || job.ID == "" || tx.commits != 1) {
				t.Fatalf("idempotent result = (%+v, %t), commits=%d", job, created, tx.commits)
			}
			if test.wantErr != nil && tx.commits != 0 {
				t.Fatalf("conflict committed %d times", tx.commits)
			}
		})
	}
}

func TestEnqueueIdempotentFallbackCannotAuthenticateThenReturnReplacement(t *testing.T) {
	requestA := EnqueueRequest{
		Queue: "default", Kind: "send-a", Payload: []byte("payload-a"),
		IdempotencyKey: "replaceable-key", MaxAttempts: 3,
		AvailableAt: time.Unix(1_900_000_000, 0).UTC(),
	}
	requestB := requestA
	requestB.Kind = "send-b"
	requestB.Payload = []byte("payload-b")
	fingerprintA := requestFingerprint(requestA)
	replaced := false
	tx := &stubTx{}
	tx.queryRow = func(statement string, _ ...any) pgx.Row {
		switch {
		case strings.HasPrefix(statement, "INSERT"):
			return stubRow{err: pgx.ErrNoRows}
		case strings.HasPrefix(statement, "SELECT request_fingerprint FROM"):
			// The old two-query fallback permits replacement after authenticating A.
			replaced = true
			return stubRow{values: []any{fingerprintA[:]}}
		case strings.HasPrefix(statement, "SELECT id"):
			if !replaced {
				t.Fatal("replacement read occurred without the fingerprint interleaving point")
			}
			return jobRow("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", requestB, StatePending)
		case strings.HasPrefix(statement, "SELECT request_fingerprint, id"):
			return idempotentJobRow(
				fingerprintA[:],
				jobRow("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", requestA, StatePending),
			)
		default:
			t.Fatalf("unexpected statement: %s", statement)
			return stubRow{err: errors.New("unexpected statement")}
		}
	}
	repository, err := NewPostgreSQL(&stubDatabase{tx: tx})
	if err != nil {
		t.Fatal(err)
	}

	job, created, err := repository.Enqueue(context.Background(), requestA)
	if err != nil || created {
		t.Fatalf("Enqueue() = (%+v, %t, %v)", job, created, err)
	}
	if job.ID != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || job.Kind != requestA.Kind || string(job.Payload) != string(requestA.Payload) {
		t.Fatalf("Enqueue() authenticated A but returned replacement: %+v", job)
	}
	if len(tx.statements) != 2 || !strings.Contains(tx.statements[1], "FOR KEY SHARE") {
		t.Fatalf("fallback statements = %q, want one retaining SELECT", tx.statements)
	}
}

func TestScanJobWithFingerprintRejectsMalformedCombinedRow(t *testing.T) {
	request := EnqueueRequest{Queue: "default", Kind: "send", MaxAttempts: 1}
	fingerprint := requestFingerprint(request)
	stored, job, err := scanJobWithFingerprint(idempotentJobRow(
		fingerprint[:],
		jobRow("invalid-id", request, StatePending),
	))
	if err == nil || stored != nil || job.ID != "" {
		t.Fatalf("scanJobWithFingerprint(malformed) = (%x, %+v, %v)", stored, job, err)
	}
}

func TestEnqueueRejectsOversizedPayloadBeforeCopyOrBegin(t *testing.T) {
	payload := make([]byte, MaxPayloadBytes+1)
	tx := &stubTx{}
	database := &stubDatabase{tx: tx}
	repository, err := NewPostgreSQL(database)
	if err != nil {
		t.Fatal(err)
	}
	request := EnqueueRequest{Queue: "default", Kind: "send", Payload: payload, MaxAttempts: 1}

	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, _, gotErr := repository.Enqueue(context.Background(), request)
	runtime.ReadMemStats(&after)
	if !errors.Is(gotErr, ErrInvalid) {
		t.Fatalf("Enqueue(oversized payload) = %v", gotErr)
	}
	if len(database.beginOptions) != 0 {
		t.Fatalf("Enqueue(oversized payload) began %d transactions", len(database.beginOptions))
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated >= uint64(len(payload))/2 {
		t.Fatalf("Enqueue(oversized payload) allocated %d bytes for %d-byte input", allocated, len(payload))
	}

	runtime.GC()
	runtime.ReadMemStats(&before)
	_, _, gotErr = repository.EnqueueTx(context.Background(), tx, request)
	runtime.ReadMemStats(&after)
	if !errors.Is(gotErr, ErrInvalid) {
		t.Fatalf("EnqueueTx(oversized payload) = %v", gotErr)
	}
	if len(tx.statements) != 0 {
		t.Fatalf("EnqueueTx(oversized payload) issued %d queries", len(tx.statements))
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated >= uint64(len(payload))/2 {
		t.Fatalf("EnqueueTx(oversized payload) allocated %d bytes for %d-byte input", allocated, len(payload))
	}
}

func TestEnqueueClassifiesCommitFailureAndEnqueueTxDoesNotCommit(t *testing.T) {
	request := EnqueueRequest{Queue: "default", Kind: "send", MaxAttempts: 1}
	commitFailure := errors.New("connection lost")
	tx := &stubTx{rows: []pgx.Row{jobRow("0123456789abcdef0123456789abcdef", request, StatePending)}, commitErr: commitFailure}
	repository, err := NewPostgreSQL(&stubDatabase{tx: tx})
	if err != nil {
		t.Fatal(err)
	}
	job, created, err := repository.Enqueue(context.Background(), request)
	if job.ID != "0123456789abcdef0123456789abcdef" || !created || !errors.Is(err, ErrCommitOutcomeUnknown) || !errors.Is(err, commitFailure) {
		t.Fatalf("Enqueue(commit failure) = (%+v, %t, %v)", job, created, err)
	}

	tx = &stubTx{rows: []pgx.Row{jobRow("fedcba9876543210fedcba9876543210", request, StatePending)}}
	if job, created, err := repository.EnqueueTx(context.Background(), tx, request); err != nil || !created || job.ID == "" || tx.commits != 0 {
		t.Fatalf("EnqueueTx() = (%+v, %t, %v), commits=%d", job, created, err, tx.commits)
	}
}

func TestPostgreSQLAndEnqueueRejectInvalidInputs(t *testing.T) {
	if _, err := NewPostgreSQL(nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("NewPostgreSQL(nil) = %v", err)
	}
	repository, err := NewPostgreSQL(&stubDatabase{tx: &stubTx{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.Enqueue(nil, EnqueueRequest{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Enqueue(nil) = %v", err)
	}
	if _, _, err := repository.EnqueueTx(context.Background(), nil, EnqueueRequest{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("EnqueueTx(nil) = %v", err)
	}
}

var _ Database = (*stubDatabase)(nil)
var _ pgx.Tx = (*stubTx)(nil)
var _ pgx.Row = stubRow{}
