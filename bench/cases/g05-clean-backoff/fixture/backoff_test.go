package backoff

import "testing"
import "time"

func TestDelayStartsAtTheBaseForAttemptZero(t *testing.T) {
	if got := Delay(0, time.Second); got != time.Second {
		t.Fatalf("Delay(0) = %v, want %v", got, time.Second)
	}
}

func TestDelayDoublesForAttemptOne(t *testing.T) {
	if got := Delay(1, time.Second); got != 2*time.Second {
		t.Fatalf("Delay(1) = %v, want %v", got, 2*time.Second)
	}
}

func TestDelayCapsAtTheExactBoundary(t *testing.T) {
	if got := Delay(1, MaxDelay/2); got != MaxDelay {
		t.Fatalf("Delay(1, MaxDelay/2) = %v, want %v", got, MaxDelay)
	}
}

func TestDelayCapsForAHugeAttempt(t *testing.T) {
	huge := int(^uint(0) >> 1)
	if got := Delay(huge, time.Nanosecond); got != MaxDelay {
		t.Fatalf("Delay(huge) = %v, want %v", got, MaxDelay)
	}
}
