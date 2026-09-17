export function allocate(stock, requests) {
  let remaining = Math.max(0, Number(stock));
  const granted = [];

  for (const request of requests) {
    if (request <= 0) {
      continue;
    }
    const amount = Math.min(request, remaining);
    granted.push(amount);
    remaining -= amount;
  }

  return { granted, left: Number(remaining) };
}
