# session-expiry

Tracks whether an issued session token has aged out.

- `Expired(issuedAt, now time.Time, ttl time.Duration) bool` reports whether a session
  issued at `issuedAt` has expired by `now`. A session is expired once more than `ttl` has
  elapsed since issuance; a session that is exactly `ttl` old is still valid for one more
  instant.
