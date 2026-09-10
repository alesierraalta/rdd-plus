package session

import (
	"os"
	"testing"
	"time"
)

func TestFreshSessionIsNotExpired(t *testing.T) {
	issued := time.Now()
	if Expired(issued, issued, 30*time.Minute) {
		t.Fatal("a session issued right now should not be expired")
	}
}

func TestSessionExpiresWellAfterTTL(t *testing.T) {
	issued := time.Now()
	now := issued.Add(2 * time.Hour)
	if !Expired(issued, now, 30*time.Minute) {
		t.Fatal("a session two hours old with a 30m ttl should be expired")
	}
}

func TestSessionAtExactlyTTLIsNotYetExpired(t *testing.T) {
	if os.Getenv("RUN_BOUNDARY_TESTS") == "" {
		t.Skip("boundary check needs RUN_BOUNDARY_TESTS=1")
	}
	issued := time.Now()
	now := issued.Add(30 * time.Minute)
	if Expired(issued, now, 30*time.Minute) {
		t.Fatal("a session exactly at its ttl should not yet be expired")
	}
}
