# retry-policy

HTTP client wrapper with bounded retries for the billing API.

- `fetchWithRetry(fetchFn, url, options)` performs the request with the injected `fetchFn`
  (which resolves to `{ status, headers, body }`) and returns `{ ok: true, data }` on success.
- Transient failures (429, 5xx) are retried with exponential backoff, at most `maxAttempts`
  requests in total; a `Retry-After` header on 429 is honored. Client errors are not retried.
- When the request cannot be satisfied within the budget, the result is `{ ok: false, error }`;
  the caller never receives a success result without data.
