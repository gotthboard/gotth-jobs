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
			var source []byte
			switch value := value.(type) {
			case nil:
			case string:
				source = []byte(value)
			case []byte:
				source = value
			default:
				return errors.New("unsupported byte scanner source")
			}
			if err := target.ScanBytes(source); err != nil {
				return err
			}
		case *bool:
			*target = value.(bool)
		case *int:
			*target = value.(int)
		case *int64:
			*target = value.(int64)
		case *time.Time:
			if value == nil {
				*target = time.Time{}
			} else {
				*target = value.(time.Time)
			}
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

type borrowedScannerRow struct {
	stubRow
	fingerprint bool
}

func (row borrowedScannerRow) Scan(destinations ...any) error {
	variableColumns := []int{0, 1, 2, 3, 4, 5, 11, 12, 14}
	if row.fingerprint {
		variableColumns = []int{0, 1, 2, 3, 4, 5, 6, 12, 13, 15}
	}
	for _, index := range variableColumns {
		if _, ok := destinations[index].(pgtype.BytesScanner); !ok {
			return errors.New("variable-width job column does not use pgtype.BytesScanner")
		}
	}
	return row.stubRow.Scan(destinations...)
}

type stubTx struct {
	pgx.Tx
	rows         []pgx.Row
	queryRow     func(string, ...any) pgx.Row
	statements   []string
	arguments    [][]any
	queryOptions [][]any
	commitErr    error
	rollbackErr  error
	commits      int
	rollbacks    int
	isolation    string
	isolationErr error
}

func (tx *stubTx) QueryRow(_ context.Context, statement string, arguments ...any) pgx.Row {
	options, sqlArguments := splitPGXQueryOptions(arguments)
	tx.statements = append(tx.statements, statement)
	tx.arguments = append(tx.arguments, append([]any(nil), sqlArguments...))
	tx.queryOptions = append(tx.queryOptions, options)
	if statement == transactionIsolationSQL {
		if tx.isolationErr != nil {
			return stubRow{err: tx.isolationErr}
		}
		isolation := tx.isolation
		if isolation == "" {
			isolation = string(pgx.ReadCommitted)
		}
		return stubRow{values: []any{isolation}}
	}
	if tx.queryRow != nil {
		return tx.queryRow(statement, sqlArguments...)
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
	queryOptions   [][]any
	beginOptions   []pgx.TxOptions
}

func (database *stubDatabase) BeginTx(_ context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	database.beginOptions = append(database.beginOptions, options)
	return database.tx, database.beginErr
}

func (database *stubDatabase) Query(_ context.Context, statement string, arguments ...any) (pgx.Rows, error) {
	options, sqlArguments := splitPGXQueryOptions(arguments)
	database.statements = append(database.statements, statement)
	database.queryArguments = append(database.queryArguments, append([]any(nil), sqlArguments...))
	database.queryOptions = append(database.queryOptions, options)
	return database.rows, database.queryErr
}

func (database *stubDatabase) QueryRow(_ context.Context, statement string, arguments ...any) pgx.Row {
	options, sqlArguments := splitPGXQueryOptions(arguments)
	database.statements = append(database.statements, statement)
	database.queryArguments = append(database.queryArguments, append([]any(nil), sqlArguments...))
	database.queryOptions = append(database.queryOptions, options)
	if len(database.directRows) == 0 {
		return stubRow{err: errors.New("unexpected query row")}
	}
	row := database.directRows[0]
	database.directRows = database.directRows[1:]
	return row
}

func splitPGXQueryOptions(arguments []any) ([]any, []any) {
	var options []any
	for len(arguments) > 0 {
		switch arguments[0].(type) {
		case pgx.QueryExecMode, pgx.QueryResultFormatsByOID:
			options = append(options, arguments[0])
			arguments = arguments[1:]
		default:
			return options, arguments
		}
	}
	return options, arguments
}

func assertJobQueryOptions(t *testing.T, options []any) {
	t.Helper()
	if len(options) != 2 || options[0] != pgx.QueryExecModeDescribeExec {
		t.Fatalf("job query options = %#v", options)
	}
	formats, ok := options[1].(pgx.QueryResultFormatsByOID)
	wantOIDs := []uint32{pgtype.TextOID, pgtype.ByteaOID, pgtype.Int4OID, pgtype.TimestamptzOID}
	if !ok || len(formats) != len(wantOIDs) {
		t.Fatalf("job result formats = %#v", options[1])
	}
	for _, oid := range wantOIDs {
		if formats[oid] != pgx.BinaryFormatCode {
			t.Fatalf("job result format for OID %d = %d, want binary", oid, formats[oid])
		}
	}
}

func jobRow(id string, request EnqueueRequest, state State) stubRow {
	now := time.Unix(1_900_000_000, 0).UTC()
	availableAt := request.AvailableAt
	if availableAt.IsZero() {
		availableAt = now
	}
	payload := request.Payload
	if payload == nil {
		payload = []byte{}
	}
	var key any
	if request.IdempotencyKey != "" {
		key = request.IdempotencyKey
	}
	return stubRow{values: []any{
		id, request.Queue, request.Kind, payload, key, string(state), 0,
		request.MaxAttempts, availableAt, now, now, nil, nil, nil, "", nil,
	}}
}

func TestEnqueueTxRejectsNonReadCommittedIsolationBeforeInsert(t *testing.T) {
	request := EnqueueRequest{Queue: "default", Kind: "send", Payload: []byte("payload"), MaxAttempts: 1}
	inserted := false
	tx := &stubTx{isolation: string(pgx.RepeatableRead)}
	tx.queryRow = func(statement string, _ ...any) pgx.Row {
		inserted = true
		return jobRow("0123456789abcdef0123456789abcdef", request, StatePending)
	}
	repository, _ := NewPostgreSQL(&stubDatabase{tx: tx})

	if _, _, err := repository.EnqueueTx(context.Background(), tx, request); !errors.Is(err, ErrInvalid) {
		t.Fatalf("EnqueueTx(repeatable read) = %v, want ErrInvalid", err)
	}
	if inserted {
		t.Fatal("EnqueueTx inserted before rejecting transaction isolation")
	}
}

func TestEnqueueTxReturnsIsolationInspectionFailureBeforeInsert(t *testing.T) {
	failure := errors.New("isolation read failed")
	tx := &stubTx{isolationErr: failure}
	repository, _ := NewPostgreSQL(&stubDatabase{tx: tx})
	request := EnqueueRequest{Queue: "default", Kind: "send", Payload: []byte("payload"), MaxAttempts: 1}

	if _, _, err := repository.EnqueueTx(context.Background(), tx, request); !errors.Is(err, failure) {
		t.Fatalf("EnqueueTx(isolation failure) = %v", err)
	}
	if len(tx.statements) != 1 || tx.statements[0] != transactionIsolationSQL {
		t.Fatalf("EnqueueTx issued statements %#v", tx.statements)
	}
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
	assertJobQueryOptions(t, tx.queryOptions[0])
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
			for _, options := range tx.queryOptions {
				assertJobQueryOptions(t, options)
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
