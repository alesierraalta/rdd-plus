# retry-backoff

Provides bounded delays for retry attempts.

- `Delay(attempt, base)` treats zero or negative attempts as the first attempt, doubles the base delay for each later attempt, and caps the result at `MaxDelay`.
- The delay remains bounded even when the attempt number is very large.
