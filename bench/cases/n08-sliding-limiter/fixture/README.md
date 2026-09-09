# sliding-limiter

Per-identifier request limiter with a sliding window.

- `createLimiter({ limit, windowMs, now })` returns an object whose `allow(id)` is true for at
  most `limit` requests from the same identifier within any `windowMs` interval, and false for
  every request beyond that until the window slides.
- Identifiers are `user:<name>` or `token:<value>` and are case-insensitive: the same identifier
  written with different capitalization shares one budget.
- `now` is injectable for deterministic tests.
