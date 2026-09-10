// Restricts value to the closed interval [min, max].
export function clamp(value, min, max) {
  if (value < min) return min;
  if (value > max) return value;
  return value;
}
