package jobs

import (
	"fmt"
	"time"
)

// Delay returns the saturating base-2 delay for a one-based attempt number.
//
// Complexity: for attempt a, time O(min(a,64)), Omega(1), with no single tight
// bound across all inputs; auxiliary space O(1), Omega(1), tight Theta(1).
func (policy RetryPolicy) Delay(attempt int) (time.Duration, error) {
	if attempt < 1 || policy.Initial < 0 || policy.Maximum < 0 || policy.Maximum > MaxRetryDelay || policy.Initial > policy.Maximum {
		return 0, fmt.Errorf("%w: retry policy is invalid", ErrInvalid)
	}
	if policy.Initial == 0 {
		return 0, nil
	}
	delay := policy.Initial
	for current := 1; current < attempt && delay < policy.Maximum; current++ {
		if delay > policy.Maximum/2 {
			return policy.Maximum, nil
		}
		delay *= 2
	}
	return min(delay, policy.Maximum), nil
}
