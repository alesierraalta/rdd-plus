package session

import "time"

// Expired reports whether a session issued at issuedAt has expired by now, given ttl.
func Expired(issuedAt, now time.Time, ttl time.Duration) bool {
	return now.Sub(issuedAt) > ttl
}
