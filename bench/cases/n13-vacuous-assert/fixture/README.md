# range-clamp

Bounds a numeric setting to its allowed range for the UI slider component.

- `clamp(value, min, max)` returns `value` restricted to the closed interval `[min, max]`:
  values below `min` return `min`, values above `max` return `max`, values inside the
  interval are returned unchanged.
