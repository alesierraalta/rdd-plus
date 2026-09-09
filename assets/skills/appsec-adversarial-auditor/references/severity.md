# Severity classes (C/M/N)

- **C (Critical)**: RCE, unauthenticated BOLA/IDOR, direct SQLi, hardcoded master production secrets.
- **M (Major)**: Internal BFLA, SSRF to private IP ranges, missing rate limits on auth endpoints, loose deserialization.
- **N (Minor/Note)**: Missing security response headers, permissive CORS without credentials, informational SAST notices.

**Classify at the sink, not the source**: severity for SSRF, injection, traversal and deserialization is set by the component that dereferences or executes the tainted value. Accepting a loopback, link-local or `javascript:` URL that only a remote third-party provider ever fetches is CWE-20 input validation (N), not SSRF (M). Name the sink `file:line` before assigning the class; without a named sink the finding is a hypothesis, not a severity.
