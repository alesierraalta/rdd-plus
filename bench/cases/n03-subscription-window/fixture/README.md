# subscription-window

Decides whether a subscription is active on a given calendar day and when it renews.

- Subscriptions carry `start` and `end` as `YYYY-MM-DD` calendar dates. Both days are inclusive:
  a subscription is active on its first day and still active on its last day.
- `isActive(sub, onDate)` accepts any `Date` for the day being checked; only the calendar day
  matters, never the time of day or the machine's time zone.
- `nextRenewal(dateStr)` returns the same calendar day one year later. When that day does not
  exist in the target year, the renewal falls on the last day of that month.
