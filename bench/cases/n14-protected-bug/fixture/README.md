# cart-pricing

Computes line-item totals for the checkout page.

- `applyPercentDiscount(cents, percent)` returns the price after taking `percent` off,
  rounded to the nearest cent (standard half-up rounding: a fractional cent of exactly
  `.5` or more rounds up).
- `qualifiesForFreeShipping(subtotalCents, thresholdCents)` returns true once the subtotal
  reaches the free-shipping threshold; an order totaling exactly the threshold ships free.
