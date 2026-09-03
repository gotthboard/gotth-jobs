package jobs

import (
	"errors"
	"testing"
	"time"
)

func FuzzEnvelopeValidationNeverPanics(f *testing.F) {
	f.Add("default", "send", []byte("payload"), "key", uint8(3), int64(0))
	f.Add("bad queue", "", []byte(nil), "\x00", uint8(0), int64(-1))
	f.Fuzz(func(t *testing.T, queue, kind string, payload []byte, key string, attempts uint8, availableNanos int64) {
		request := EnqueueRequest{
			Queue: queue, Kind: kind, Payload: payload, IdempotencyKey: key,
			MaxAttempts: int(attempts), AvailableAt: time.Unix(0, availableNanos).UTC(),
		}
		_ = validateEnqueue(request)
		copy := cloneEnqueue(request)
		if len(copy.Payload) != len(payload) {
			t.Fatalf("payload length changed from %d to %d", len(payload), len(copy.Payload))
		}
		_ = requestFingerprint(copy)
	})
}

func FuzzBoundedFailureIsValid(f *testing.F) {
	f.Add("ordinary error")
	f.Add("bad\x00text")
	f.Fuzz(func(t *testing.T, text string) {
		message := boundedFailure(errors.New(text))
		if err := validateFailure(Failure{Message: message}); err != nil {
			t.Fatalf("boundedFailure produced invalid text: %v", err)
		}
	})
}
