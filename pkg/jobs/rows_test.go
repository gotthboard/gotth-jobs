package jobs

import (
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestJobQueryArgumentsForceDescribedBinaryBytea(t *testing.T) {
	arguments := jobQueryArguments("value", 7)
	if len(arguments) != 4 || arguments[0] != pgx.QueryExecModeDescribeExec {
		t.Fatalf("jobQueryArguments() = %#v", arguments)
	}
	formats, ok := arguments[1].(pgx.QueryResultFormatsByOID)
	wantFormats := pgx.QueryResultFormatsByOID{
		pgtype.TextOID:        pgx.BinaryFormatCode,
		pgtype.ByteaOID:       pgx.BinaryFormatCode,
		pgtype.Int4OID:        pgx.BinaryFormatCode,
		pgtype.TimestamptzOID: pgx.BinaryFormatCode,
	}
	if !ok || !reflect.DeepEqual(formats, wantFormats) {
		t.Fatalf("job result formats = %#v", arguments[1])
	}
	if arguments[2] != "value" || arguments[3] != 7 {
		t.Fatalf("SQL arguments = %#v", arguments[2:])
	}
}

func measuredScanAllocation(row stubRow) (Job, error, uint64) {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	job, err := scanJob(row)
	runtime.ReadMemStats(&after)
	return job, err, after.TotalAlloc - before.TotalAlloc
}

func TestScanJobOwnsMaximumPayloadWithOneCopy(t *testing.T) {
	payload := make([]byte, MaxPayloadBytes)
	payload[0], payload[len(payload)-1] = 0x11, 0x22
	request := EnqueueRequest{
		Queue: "default", Kind: "maximum", Payload: payload, MaxAttempts: 1,
		AvailableAt: time.Unix(1_900_000_000, 0).UTC(),
	}

	job, err, allocated := measuredScanAllocation(jobRow("0123456789abcdef0123456789abcdef", request, StatePending))
	if err != nil {
		t.Fatalf("scanJob(maximum payload) = %v", err)
	}
	if allocated < MaxPayloadBytes || allocated >= MaxPayloadBytes+MaxPayloadBytes/2 {
		t.Fatalf("scanJob(maximum payload) allocated %d bytes, want one payload-sized copy", allocated)
	}
	payload[0] = 0x33
	if job.Payload[0] != 0x11 {
		t.Fatal("scanned payload aliases the row source")
	}
	job.Payload[len(job.Payload)-1] = 0x44
	if payload[len(payload)-1] != 0x22 {
		t.Fatal("row source aliases the returned payload")
	}
}

func TestScanJobRejectsOversizedPayloadBeforeAllocation(t *testing.T) {
	payload := make([]byte, MaxPayloadBytes+1)
	request := EnqueueRequest{
		Queue: "default", Kind: "oversized", Payload: payload, MaxAttempts: 1,
		AvailableAt: time.Unix(1_900_000_000, 0).UTC(),
	}

	job, err, allocated := measuredScanAllocation(jobRow("0123456789abcdef0123456789abcdef", request, StatePending))
	if err == nil || job.ID != "" || job.Payload != nil {
		t.Fatalf("scanJob(oversized payload) = (%+v, %v), want zero job and error", job, err)
	}
	if allocated >= MaxPayloadBytes/2 {
		t.Fatalf("scanJob(oversized payload) allocated %d bytes before rejection", allocated)
	}
}

func validPendingStoredJob() Job {
	now := time.Unix(1_900_000_000, 0).UTC()
	return Job{
		ID: "0123456789abcdef0123456789abcdef", Queue: "default", Kind: "send",
		Payload: []byte{}, State: StatePending, MaxAttempts: 3,
		AvailableAt: now, CreatedAt: now, UpdatedAt: now,
	}
}

func TestStoredJobRejectsNullPayload(t *testing.T) {
	job := validPendingStoredJob()
	job.Payload = nil
	if err := validateStoredJob(job); err == nil || !strings.Contains(err.Error(), "NULL") {
		t.Fatalf("validateStoredJob(NULL payload) = %v", err)
	}

	row := jobRow(job.ID, EnqueueRequest{Queue: job.Queue, Kind: job.Kind, MaxAttempts: job.MaxAttempts}, StatePending)
	row.values[3] = []byte(nil)
	if _, err := scanJob(row); err == nil || !strings.Contains(err.Error(), "NULL") {
		t.Fatalf("scanJob(NULL payload) = %v", err)
	}
}

func TestStoredJobRejectsInvalidMandatoryTimestamps(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	nonUTC := now.In(time.FixedZone("not-utc", 3600))
	outOfRange := maximumPostgreSQLTimestamp.Add(time.Microsecond)
	subMicrosecond := now.Add(time.Nanosecond)
	tests := []struct {
		name   string
		mutate func(*Job)
	}{
		{name: "available missing", mutate: func(job *Job) { job.AvailableAt = time.Time{} }},
		{name: "available non UTC", mutate: func(job *Job) { job.AvailableAt = nonUTC }},
		{name: "available range", mutate: func(job *Job) { job.AvailableAt = outOfRange }},
		{name: "available precision", mutate: func(job *Job) { job.AvailableAt = subMicrosecond }},
		{name: "created missing", mutate: func(job *Job) { job.CreatedAt = time.Time{} }},
		{name: "created non UTC", mutate: func(job *Job) { job.CreatedAt = nonUTC }},
		{name: "created range", mutate: func(job *Job) { job.CreatedAt = outOfRange }},
		{name: "created precision", mutate: func(job *Job) { job.CreatedAt = subMicrosecond }},
		{name: "updated missing", mutate: func(job *Job) { job.UpdatedAt = time.Time{} }},
		{name: "updated non UTC", mutate: func(job *Job) { job.UpdatedAt = nonUTC }},
		{name: "updated range", mutate: func(job *Job) { job.UpdatedAt = outOfRange }},
		{name: "updated precision", mutate: func(job *Job) { job.UpdatedAt = subMicrosecond }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			job := validPendingStoredJob()
			test.mutate(&job)
			if err := validateStoredJob(job); err == nil {
				t.Fatalf("validateStoredJob(%s) accepted %+v", test.name, job)
			}
		})
	}
}

func TestStoredJobRejectsInvalidOptionalTimestamps(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	invalidTimes := []struct {
		name  string
		value time.Time
	}{
		{name: "non UTC", value: now.In(time.FixedZone("not-utc", -3600))},
		{name: "range", value: minimumPostgreSQLTimestamp.Add(-time.Microsecond)},
		{name: "precision", value: now.Add(time.Nanosecond)},
	}
	for _, test := range invalidTimes {
		t.Run("lease "+test.name, func(t *testing.T) {
			job := claimedJob(1)
			job.LeaseUntil = test.value
			if err := validateStoredJob(job); err == nil {
				t.Fatalf("validateStoredJob(lease %s) accepted %+v", test.name, job)
			}
		})
		t.Run("finished "+test.name, func(t *testing.T) {
			job := validPendingStoredJob()
			job.State = StateSucceeded
			job.Attempts = 1
			job.FinishedAt = test.value
			if err := validateStoredJob(job); err == nil {
				t.Fatalf("validateStoredJob(finished %s) accepted %+v", test.name, job)
			}
		})
	}
}

func TestStoredJobAcceptsPostgreSQLTimestampBoundaries(t *testing.T) {
	job := validPendingStoredJob()
	job.AvailableAt = minimumPostgreSQLTimestamp
	job.CreatedAt = minimumPostgreSQLTimestamp
	job.UpdatedAt = maximumPostgreSQLTimestamp
	if err := validateStoredJob(job); err != nil {
		t.Fatalf("validateStoredJob(mandatory boundaries) = %v", err)
	}

	job.State = StateRunning
	job.Attempts = 1
	job.Lease = Lease{JobID: job.ID, Token: strings.Repeat("ab", 32)}
	job.LeaseOwner = "worker"
	job.LeaseUntil = maximumPostgreSQLTimestamp
	if err := validateStoredJob(job); err != nil {
		t.Fatalf("validateStoredJob(lease boundary) = %v", err)
	}

	job.State = StateSucceeded
	job.Lease = Lease{}
	job.LeaseOwner = ""
	job.LeaseUntil = time.Time{}
	job.FinishedAt = maximumPostgreSQLTimestamp
	if err := validateStoredJob(job); err != nil {
		t.Fatalf("validateStoredJob(finished boundary) = %v", err)
	}
}

func TestScanJobRejectsPresentZeroOptionalTimestamp(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	tests := []struct {
		name  string
		row   stubRow
		index int
	}{
		{name: "lease_until", row: runningJobRow(id, strings.Repeat("ab", 32), 1), index: 13},
		{name: "finished_at", row: terminalJobRow(id, StateSucceeded, 1, ""), index: 15},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.row.values[test.index] = time.Time{}
			if _, err := scanJob(test.row); err == nil {
				t.Fatalf("scanJob(present zero %s) succeeded", test.name)
			}
		})
	}
}

func TestStoredJobRejectsTerminalTimestampShape(t *testing.T) {
	missing := validPendingStoredJob()
	missing.State = StateSucceeded
	missing.Attempts = 1
	if err := validateStoredJob(missing); err == nil {
		t.Fatal("validateStoredJob(succeeded without finished timestamp) succeeded")
	}

	nonterminal := validPendingStoredJob()
	nonterminal.FinishedAt = nonterminal.UpdatedAt
	if err := validateStoredJob(nonterminal); err == nil {
		t.Fatal("validateStoredJob(pending with finished timestamp) succeeded")
	}
}

func TestScanJobRejectsNullMandatoryTimestampColumns(t *testing.T) {
	request := EnqueueRequest{Queue: "default", Kind: "send", MaxAttempts: 1}
	for _, test := range []struct {
		name  string
		index int
	}{
		{name: "available_at", index: 8},
		{name: "created_at", index: 9},
		{name: "updated_at", index: 10},
	} {
		t.Run(test.name, func(t *testing.T) {
			row := jobRow("0123456789abcdef0123456789abcdef", request, StatePending)
			row.values[test.index] = nil
			if _, err := scanJob(row); err == nil {
				t.Fatalf("scanJob(NULL %s) succeeded", test.name)
			}
		})
	}
}
