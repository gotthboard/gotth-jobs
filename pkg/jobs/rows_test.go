package jobs

import (
	"runtime"
	"testing"
	"time"
)

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
