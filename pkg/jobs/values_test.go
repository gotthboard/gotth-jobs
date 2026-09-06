package jobs

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestEnqueueRequestValidationAndCopy(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	valid := EnqueueRequest{Queue: "mail.outbound", Kind: "digest/send:v1", Payload: []byte("payload"), IdempotencyKey: "digest-42", MaxAttempts: 3, AvailableAt: now}
	if err := validateEnqueue(valid); err != nil {
		t.Fatalf("validateEnqueue(valid) = %v", err)
	}

	copy := cloneEnqueue(valid)
	valid.Payload[0] = 'X'
	if string(copy.Payload) != "payload" {
		t.Fatalf("clone payload = %q, want independent copy", copy.Payload)
	}
	empty := cloneEnqueue(EnqueueRequest{})
	if empty.Payload == nil || len(empty.Payload) != 0 {
		t.Fatalf("clone empty payload = %#v, want owned non-nil empty slice", empty.Payload)
	}

	tests := []struct {
		name string
		edit func(*EnqueueRequest)
	}{
		{name: "empty queue", edit: func(r *EnqueueRequest) { r.Queue = "" }},
		{name: "oversize queue", edit: func(r *EnqueueRequest) { r.Queue = strings.Repeat("q", MaxQueueBytes+1) }},
		{name: "invalid queue", edit: func(r *EnqueueRequest) { r.Queue = "bad queue" }},
		{name: "empty kind", edit: func(r *EnqueueRequest) { r.Kind = "" }},
		{name: "oversize kind", edit: func(r *EnqueueRequest) { r.Kind = strings.Repeat("k", MaxKindBytes+1) }},
		{name: "oversize payload", edit: func(r *EnqueueRequest) { r.Payload = make([]byte, MaxPayloadBytes+1) }},
		{name: "oversize key", edit: func(r *EnqueueRequest) { r.IdempotencyKey = strings.Repeat("i", MaxIdempotencyKeyBytes+1) }},
		{name: "nul key", edit: func(r *EnqueueRequest) { r.IdempotencyKey = "bad\x00key" }},
		{name: "sub-microsecond availability", edit: func(r *EnqueueRequest) { r.AvailableAt = now.Add(time.Nanosecond) }},
		{name: "zero attempts", edit: func(r *EnqueueRequest) { r.MaxAttempts = 0 }},
		{name: "too many attempts", edit: func(r *EnqueueRequest) { r.MaxAttempts = MaxAttempts + 1 }},
		{name: "non utc availability", edit: func(r *EnqueueRequest) { r.AvailableAt = now.In(time.FixedZone("other", 3600)) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := cloneEnqueue(valid)
			request.Payload = []byte("payload")
			test.edit(&request)
			if err := validateEnqueue(request); !errors.Is(err, ErrInvalid) {
				t.Fatalf("validateEnqueue() = %v, want ErrInvalid", err)
			}
		})
	}

	for _, size := range []int{MaxPayloadBytes - 1, MaxPayloadBytes} {
		request := valid
		request.Payload = make([]byte, size)
		if err := validateEnqueue(request); err != nil {
			t.Fatalf("payload size %d rejected: %v", size, err)
		}
	}
}

func TestClaimAndFailureValidationBoundaries(t *testing.T) {
	claim := ClaimRequest{Queue: "default", Worker: strings.Repeat("w", MaxWorkerBytes), LeaseDuration: time.Hour}
	if err := validateClaim(claim); err != nil {
		t.Fatalf("validateClaim(limit) = %v", err)
	}

	invalidClaims := []ClaimRequest{
		{},
		{Queue: "default", Worker: "worker", LeaseDuration: time.Second - 1},
		{Queue: "default", Worker: "worker", LeaseDuration: time.Hour + 1},
		{Queue: "default", Worker: strings.Repeat("w", MaxWorkerBytes+1), LeaseDuration: time.Second},
		{Queue: "default", Worker: "worker", LeaseDuration: time.Second + time.Nanosecond},
	}
	for _, request := range invalidClaims {
		if err := validateClaim(request); !errors.Is(err, ErrInvalid) {
			t.Fatalf("validateClaim(%+v) = %v, want ErrInvalid", request, err)
		}
	}

	for _, failure := range []Failure{
		{Message: strings.Repeat("e", MaxFailureBytes), RetryAfter: MaxRetryDelay},
		{Message: "permanent", Permanent: true},
	} {
		if err := validateFailure(failure); err != nil {
			t.Fatalf("validateFailure(limit) = %v", err)
		}
	}
	for _, failure := range []Failure{
		{Message: strings.Repeat("e", MaxFailureBytes+1)},
		{Message: "bad\x00error"},
		{RetryAfter: -1},
		{RetryAfter: MaxRetryDelay + 1},
		{RetryAfter: time.Nanosecond},
		{Permanent: true, RetryAfter: time.Second},
	} {
		if err := validateFailure(failure); !errors.Is(err, ErrInvalid) {
			t.Fatalf("validateFailure(%+v) = %v, want ErrInvalid", failure, err)
		}
	}
}

func TestOpaqueIDUsesExactEntropy(t *testing.T) {
	source := bytes.NewReader(bytes.Repeat([]byte{0xab}, 32))
	id, err := newOpaqueID(source, 16)
	if err != nil || id != strings.Repeat("ab", 16) {
		t.Fatalf("newOpaqueID() = (%q, %v)", id, err)
	}
	if _, err := newOpaqueID(bytes.NewReader(nil), 16); err == nil {
		t.Fatal("newOpaqueID(short source) returned nil error")
	}
}

func TestRetryPolicyDelaySaturates(t *testing.T) {
	policy := RetryPolicy{Initial: time.Second, Maximum: 10 * time.Second}
	wants := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 10 * time.Second, 10 * time.Second}
	for index, want := range wants {
		got, err := policy.Delay(index + 1)
		if err != nil || got != want {
			t.Fatalf("Delay(%d) = (%s, %v), want %s", index+1, got, err, want)
		}
	}
	if _, err := policy.Delay(0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Delay(0) = %v, want ErrInvalid", err)
	}
	if _, err := (RetryPolicy{Initial: -1, Maximum: time.Second}).Delay(1); !errors.Is(err, ErrInvalid) {
		t.Fatalf("negative policy = %v, want ErrInvalid", err)
	}
	if _, err := (RetryPolicy{Initial: time.Second, Maximum: MaxRetryDelay + 1}).Delay(1); !errors.Is(err, ErrInvalid) {
		t.Fatalf("oversize policy = %v, want ErrInvalid", err)
	}
	if got, err := (RetryPolicy{Maximum: MaxRetryDelay}).Delay(int(^uint(0) >> 1)); err != nil || got != 0 {
		t.Fatalf("zero initial delay = (%s, %v), want (0, nil)", got, err)
	}
}

func TestPermanentPreservesErrorTraversal(t *testing.T) {
	cause := errors.New("do not retry")
	wrapped := Permanent(cause)
	if !IsPermanent(wrapped) || !errors.Is(wrapped, cause) {
		t.Fatalf("Permanent() did not preserve classification and cause: %v", wrapped)
	}
	if Permanent(nil) != nil || IsPermanent(cause) {
		t.Fatal("nil or ordinary error permanent classification is wrong")
	}
}

func TestRequestFingerprintBindsEverySemanticField(t *testing.T) {
	base := EnqueueRequest{Queue: "default", Kind: "send", Payload: []byte("payload"), IdempotencyKey: "same", MaxAttempts: 3, AvailableAt: time.Unix(1_900_000_000, 123_000).UTC()}
	want := requestFingerprint(base)
	if got := requestFingerprint(base); got != want {
		t.Fatal("request fingerprint is nondeterministic")
	}

	mutations := []func(*EnqueueRequest){
		func(r *EnqueueRequest) { r.Queue = "other" },
		func(r *EnqueueRequest) { r.Kind = "other" },
		func(r *EnqueueRequest) { r.Payload = []byte("other") },
		func(r *EnqueueRequest) { r.MaxAttempts++ },
		func(r *EnqueueRequest) { r.AvailableAt = r.AvailableAt.Add(time.Microsecond) },
	}
	for index, mutate := range mutations {
		request := cloneEnqueue(base)
		mutate(&request)
		if requestFingerprint(request) == want {
			t.Fatalf("mutation %d did not alter fingerprint", index)
		}
	}
	request := cloneEnqueue(base)
	request.IdempotencyKey = "different"
	if requestFingerprint(request) != want {
		t.Fatal("idempotency key must not be part of request semantics")
	}
}

func TestRequestFingerprintDoesNotUseOverflowingUnixNanoseconds(t *testing.T) {
	first := EnqueueRequest{Queue: "default", Kind: "send", MaxAttempts: 1, AvailableAt: time.Unix(0, 0).UTC()}
	second := first
	second.AvailableAt = time.Unix(18_446_744_073, 709_551_616).UTC()
	if first.AvailableAt.UnixNano() != second.AvailableAt.UnixNano() {
		t.Fatal("test fixture no longer demonstrates UnixNano wraparound")
	}
	if requestFingerprint(first) == requestFingerprint(second) {
		t.Fatal("distinct schedule times have the same request fingerprint")
	}
}

func TestEnqueueAvailabilityPostgreSQLRange(t *testing.T) {
	minimum := time.Date(-4713, time.November, 24, 0, 0, 0, 0, time.UTC)
	maximum := time.Date(294276, time.December, 31, 23, 59, 59, 999999000, time.UTC)
	overflowToY2K := time.Unix(18_447_690_758_509, 551_616_000).UTC()
	tests := []struct {
		name      string
		available time.Time
		valid     bool
	}{
		{name: "minimum", available: minimum, valid: true},
		{name: "minimum plus one microsecond", available: minimum.Add(time.Microsecond), valid: true},
		{name: "maximum minus one microsecond", available: maximum.Add(-time.Microsecond), valid: true},
		{name: "maximum", available: maximum, valid: true},
		{name: "below minimum", available: minimum.Add(-time.Microsecond)},
		{name: "above maximum", available: maximum.Add(time.Microsecond)},
		{name: "pgx codec wraps to Y2K", available: overflowToY2K},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := EnqueueRequest{
				Queue: "default", Kind: "send", MaxAttempts: 1,
				AvailableAt: test.available,
			}
			err := validateEnqueue(request)
			if test.valid && err != nil {
				t.Fatalf("validateEnqueue(%v) = %v", test.available, err)
			}
			if !test.valid && !errors.Is(err, ErrInvalid) {
				t.Fatalf("validateEnqueue(%v) = %v, want ErrInvalid", test.available, err)
			}
		})
	}
}

func TestEveryDocumentedEnvelopeBoundary(t *testing.T) {
	for _, size := range []int{MaxQueueBytes - 1, MaxQueueBytes} {
		request := EnqueueRequest{Queue: strings.Repeat("q", size), Kind: "kind", MaxAttempts: 1}
		if err := validateEnqueue(request); err != nil {
			t.Fatalf("queue size %d rejected: %v", size, err)
		}
	}
	for _, size := range []int{MaxKindBytes - 1, MaxKindBytes} {
		request := EnqueueRequest{Queue: "queue", Kind: strings.Repeat("k", size), MaxAttempts: 1}
		if err := validateEnqueue(request); err != nil {
			t.Fatalf("kind size %d rejected: %v", size, err)
		}
	}
	for _, size := range []int{MaxIdempotencyKeyBytes - 1, MaxIdempotencyKeyBytes} {
		request := EnqueueRequest{Queue: "queue", Kind: "kind", IdempotencyKey: strings.Repeat("i", size), MaxAttempts: 1}
		if err := validateEnqueue(request); err != nil {
			t.Fatalf("idempotency-key size %d rejected: %v", size, err)
		}
	}
	for _, attempts := range []int{MaxAttempts - 1, MaxAttempts} {
		request := EnqueueRequest{Queue: "queue", Kind: "kind", MaxAttempts: attempts}
		if err := validateEnqueue(request); err != nil {
			t.Fatalf("attempt count %d rejected: %v", attempts, err)
		}
	}
}
