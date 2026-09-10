// Price after taking percent off, rounded to the nearest cent.
export function applyPercentDiscount(cents, percent) {
  const discounted = (cents * (100 - percent)) / 100;
  return Math.round(discounted);
}

// True once the subtotal reaches the free-shipping threshold.
export function qualifiesForFreeShipping(subtotalCents, thresholdCents) {
  return subtotalCents > thresholdCents;
}
