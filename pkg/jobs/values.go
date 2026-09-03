package jobs

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// requestFingerprint binds every semantic request field other than the
// idempotency key using unambiguous length-prefix framing.
//
// Complexity: for q queue, k kind, and p payload bytes, time O(q+k+p),
// Omega(q+k+p), tight Theta(q+k+p); auxiliary space O(1), Omega(1), tight
// Theta(1) beyond the fixed hash state.
func requestFingerprint(request EnqueueRequest) [sha256.Size]byte {
	hash := sha256.New()
	writeFingerprintField(hash, []byte(request.Queue))
	writeFingerprintField(hash, []byte(request.Kind))
	writeFingerprintField(hash, request.Payload)
	var scalar [8]byte
	binary.BigEndian.PutUint64(scalar[:], uint64(request.MaxAttempts))
	_, _ = hash.Write(scalar[:])
	if request.AvailableAt.IsZero() {
		_, _ = hash.Write([]byte{0})
	} else {
		_, _ = hash.Write([]byte{1})
		binary.BigEndian.PutUint64(scalar[:], uint64(request.AvailableAt.UnixNano()))
		_, _ = hash.Write(scalar[:])
	}
	var result [sha256.Size]byte
	copy(result[:], hash.Sum(nil))
	return result
}

// writeFingerprintField adds one length-prefixed byte field to hash.
//
// Complexity: for n field bytes and delegated hash write cost H(n), time
// O(n)+H(n), Omega(n), tight Theta(n)+H(n); auxiliary space O(1), Omega(1),
// tight Theta(1).
func writeFingerprintField(hash io.Writer, field []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(field)))
	_, _ = hash.Write(size[:])
	_, _ = hash.Write(field)
}

// validateEnqueue rejects requests that cannot be represented by the public
// and database contracts.
//
// Complexity: for q queue bytes, k kind bytes, p payload bytes, and i key
// bytes, time O(q+k+p+i), Omega(q+k), tight Theta(q+k+p+i) because UTF-8 and
// NUL validation scans present variable fields; auxiliary space O(1),
// Omega(1), tight Theta(1).
func validateEnqueue(request EnqueueRequest) error {
	if err := validateName("queue", request.Queue, MaxQueueBytes); err != nil {
		return err
	}
	if err := validateName("kind", request.Kind, MaxKindBytes); err != nil {
		return err
	}
	if len(request.Payload) > MaxPayloadBytes {
		return fmt.Errorf("%w: payload exceeds %d bytes", ErrInvalid, MaxPayloadBytes)
	}
	if len(request.IdempotencyKey) > MaxIdempotencyKeyBytes || strings.IndexByte(request.IdempotencyKey, 0) >= 0 || !utf8.ValidString(request.IdempotencyKey) {
		return fmt.Errorf("%w: idempotency key is invalid", ErrInvalid)
	}
	if request.MaxAttempts < 1 || request.MaxAttempts > MaxAttempts {
		return fmt.Errorf("%w: max attempts must be between 1 and %d", ErrInvalid, MaxAttempts)
	}
	if !request.AvailableAt.IsZero() && request.AvailableAt.Location() != time.UTC {
		return fmt.Errorf("%w: availability must use UTC", ErrInvalid)
	}
	return nil
}

// cloneEnqueue isolates caller-owned payload memory from subsequent mutation.
//
// Complexity: for p payload bytes, time O(p), Omega(p), tight Theta(p);
// auxiliary space O(p), Omega(p), tight Theta(p).
func cloneEnqueue(request EnqueueRequest) EnqueueRequest {
	payload := make([]byte, len(request.Payload))
	copy(payload, request.Payload)
	request.Payload = payload
	return request
}

// validateClaim rejects invalid queue, worker, and lease bounds.
//
// Complexity: for q queue bytes and w worker bytes, time O(q+w), Omega(q),
// tight Theta(q+w); auxiliary space O(1), Omega(1), tight Theta(1).
func validateClaim(request ClaimRequest) error {
	if err := validateName("queue", request.Queue, MaxQueueBytes); err != nil {
		return err
	}
	if len(request.Worker) == 0 || len(request.Worker) > MaxWorkerBytes || strings.IndexByte(request.Worker, 0) >= 0 || !utf8.ValidString(request.Worker) {
		return fmt.Errorf("%w: worker is invalid", ErrInvalid)
	}
	if request.LeaseDuration < MinLeaseDuration || request.LeaseDuration > MaxLeaseDuration {
		return fmt.Errorf("%w: lease must be between %s and %s", ErrInvalid, MinLeaseDuration, MaxLeaseDuration)
	}
	return nil
}

// validateFailure enforces the bounded diagnostic and retry delay contract.
//
// Complexity: for m message bytes, time O(m), Omega(1), tight Theta(m) when a
// message is present; auxiliary space O(1), Omega(1), tight Theta(1).
func validateFailure(failure Failure) error {
	if len(failure.Message) > MaxFailureBytes || strings.IndexByte(failure.Message, 0) >= 0 || !utf8.ValidString(failure.Message) {
		return fmt.Errorf("%w: failure message is invalid", ErrInvalid)
	}
	if failure.RetryAfter < 0 || failure.RetryAfter > MaxRetryDelay {
		return fmt.Errorf("%w: retry delay must be between zero and %s", ErrInvalid, MaxRetryDelay)
	}
	if failure.Permanent && failure.RetryAfter != 0 {
		return fmt.Errorf("%w: permanent failure cannot carry a retry delay", ErrInvalid)
	}
	return nil
}

// validateName accepts a deliberately small ASCII identifier alphabet. These
// values are data, never SQL identifiers.
//
// Complexity: for n bytes, time O(n), Omega(1), tight Theta(n) for valid
// input; auxiliary space O(1), Omega(1), tight Theta(1).
func validateName(label, value string, maximum int) error {
	if len(value) == 0 || len(value) > maximum {
		return fmt.Errorf("%w: %s length is invalid", ErrInvalid, label)
	}
	for index := range len(value) {
		character := value[index]
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || strings.ContainsRune("._:/-", rune(character)) {
			continue
		}
		return fmt.Errorf("%w: %s contains an invalid byte", ErrInvalid, label)
	}
	return nil
}

// newOpaqueID reads size cryptographic bytes and returns their lowercase hex
// encoding.
//
// Complexity: for size s and delegated reader cost R(s), time O(s)+R(s),
// Omega(s), tight Theta(s)+R(s); auxiliary space O(s), Omega(s), tight
// Theta(s).
func newOpaqueID(source io.Reader, size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := io.ReadFull(source, buffer); err != nil {
		return "", fmt.Errorf("generate opaque identifier: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

// newJobID returns a 128-bit random job identifier.
//
// Complexity: delegated entropy cost R(16) dominates; local time and space are
// tight Theta(1) because the byte count is fixed.
func newJobID() (string, error) {
	return newOpaqueID(rand.Reader, 16)
}

// newLeaseToken returns a 256-bit random fencing token.
//
// Complexity: delegated entropy cost R(32) dominates; local time and space are
// tight Theta(1) because the byte count is fixed.
func newLeaseToken() (string, error) {
	return newOpaqueID(rand.Reader, 32)
}
