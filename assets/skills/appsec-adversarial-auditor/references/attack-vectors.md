# AppSec Attack Vectors and Invariant Design

## 1. BOLA / IDOR (Broken Object Level Authorization)

BOLA occurs when controllers receive resource IDs and query data without scoping to the authenticated caller's identity:

```sql
-- VULNERABLE: Relies solely on client-supplied ID
SELECT * FROM orders WHERE id = :order_id;

-- SECURE: Enforces tenant/user scoping at database level
SELECT * FROM orders WHERE id = :order_id AND tenant_id = :auth_tenant_id;
```

### Dual-Persona Test Protocol:
1. Create `tenant_1` with resource `res_1`.
2. Create `tenant_2` with caller `user_2`.
3. With written authorization for this exact synthetic target, dispatch a request with `user_2` credentials targeting `res_1`.
4. The default expected response is `403 Forbidden` or `404 Not Found`; use the protocol's declared denial contract when it specifies another non-disclosing status.

---

## 2. SSRF (Server-Side Request Forgery)

### Common Bypass Techniques:
- **Cloud Metadata IPs**: `http://169.254.169.254/latest/meta-data/` (AWS), `http://metadata.google.internal/` (GCP).
- **Alternative Numeric IP Encodings**: `2130706433` (decimal for 127.0.0.1), `0x7f.1` (hex), `017700000001` (octal).
- **DNS Rebinding**: Domain returns public IP on validation, but returns `127.0.0.1` on subsequent socket dial.

### Defensive Pattern:
As the default SSRF defense, resolve the host before connection and enforce IP-boundary validation in the socket dialer, blocking RFC 1918 private ranges, loopback (`127.0.0.0/8`, `::1`), and link-local addresses. A protocol-specific allowlist or owner-approved test target may define a narrower exception; validate it at the dial boundary and document the declared target.

---

## 3. Formal Invariant Patterns for Security Fuzzing

| Invariant Pattern | Mathematical Property / Rule | Verification Goal |
| :--- | :--- | :--- |
| **Crash & Panic Freedom** | $\forall x \in \text{DeclaredBytes}: f(x) \to \text{Result}$ | The selected parser should not trigger unhandled exceptions, panics, or OOM loops within the declared input/resource bounds; protocol-defined rejection and unavailable fuzzing capability are reported exceptions. |
| **Round-Trip Symmetry** | $\text{decode}(\text{encode}(x)) == x$ | Where the protocol promises round trips, verify serialization fidelity; otherwise assert the declared canonicalization or loss policy. |
| **Differential Parity** | $\text{Parser}_A(x) == \text{Parser}_B(x)$ | When two parsers are security-relevant, compare their declared interpretations to detect request-smuggling differentials; do not assume parity where the protocol specifies different grammars. |
| **Conservation Laws** | $\sum \text{Balances}_{t0} == \sum \text{Balances}_{t1}$ | For state machines that declare conservation, verify balance totals across concurrent actions; record when the target intentionally permits fees, rounding, or other declared deltas. |
