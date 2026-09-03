package jobs

import "time"

const (
	MaxQueueBytes          = 128
	MaxKindBytes           = 128
	MaxWorkerBytes         = 256
	MaxIdempotencyKeyBytes = 256
	MaxPayloadBytes        = 1 << 20
	MaxFailureBytes        = 4 << 10
	MaxAttempts            = 100
	MaxDeadPage            = 100
	MinLeaseDuration       = time.Second
	MaxLeaseDuration       = time.Hour
	MaxRetryDelay          = 24 * time.Hour
)

// State is the durable lifecycle state of one job.
type State string

const (
	StatePending   State = "pending"
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateDead      State = "dead"
	StateCanceled  State = "canceled"
)

// Job is a copied snapshot of one durable envelope. Payload never aliases the
// caller's enqueue slice or a database driver's reusable row buffer.
type Job struct {
	ID             string
	Queue          string
	Kind           string
	Payload        []byte
	IdempotencyKey string
	State          State
	Attempts       int
	MaxAttempts    int
	AvailableAt    time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Lease          Lease
	LeaseOwner     string
	LeaseUntil     time.Time
	LastError      string
	FinishedAt     time.Time
}

// Lease identifies one exact running attempt. Tokens are secrets for mutation
// authority and should not be logged or exposed to untrusted callers.
type Lease struct {
	JobID string
	Token string
}

// EnqueueRequest describes one product-owned payload and delivery budget.
type EnqueueRequest struct {
	Queue          string
	Kind           string
	Payload        []byte
	IdempotencyKey string
	MaxAttempts    int
	AvailableAt    time.Time
}

// ClaimRequest selects one queue and defines the new attempt's owner and
// database-clock lease duration.
type ClaimRequest struct {
	Queue         string
	Worker        string
	LeaseDuration time.Duration
}

// Failure tells the store whether an exact active attempt may retry and when.
type Failure struct {
	Message    string
	RetryAfter time.Duration
	Permanent  bool
}

// DeadCursor is the stable exclusive cursor returned from a dead Job's
// FinishedAt and ID fields.
type DeadCursor struct {
	FinishedAt time.Time
	ID         string
}

// Counts contains exact current row counts for one queue.
type Counts struct {
	Pending   int64
	Running   int64
	Succeeded int64
	Dead      int64
	Canceled  int64
}

// RetryPolicy computes a saturating base-2 delay from a one-based attempt.
type RetryPolicy struct {
	Initial time.Duration
	Maximum time.Duration
}
