# ledger

Running balance for customer accounts.

- Amounts arrive as decimal strings with at most three decimals, e.g. `"12.50"` or `"0.125"`.
- `post(id, amount)` records an entry and returns the new balance; `balance()` is the exact sum
  of every posted amount, so two ledgers fed the same entries always agree to the cent.
- `formatCents(amount)` renders an amount with exactly two decimals, rounding half up
  (`"1.005"` → `"1.01"`).
