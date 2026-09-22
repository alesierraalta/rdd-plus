package backoff

import "time"

// MaxDelay is the largest delay returned by Delay.
const MaxDelay time.Duration = 30 * time.Second

// Delay returns the exponential retry delay for an attempt, capped at MaxDelay.
func Delay(attempt int, base time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}
	if attempt < 0 {
		attempt = 0
	}
	if base >= MaxDelay {
		return MaxDelay
	}

	delay := base
	for i := 0; i < attempt && delay < MaxDelay; i++ {
		if delay > MaxDelay/2 {
			return MaxDelay
		}
		delay *= 2
	}
	if delay > MaxDelay {
		return MaxDelay
	}
	return delay
}
