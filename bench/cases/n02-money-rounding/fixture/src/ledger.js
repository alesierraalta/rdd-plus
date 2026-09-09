// Keeps the entries and the running balance of one account.
export function createLedger() {
  let balance = 0;
  const entries = [];
  return {
    post(id, amount) {
      const value = parseFloat(amount);
      if (!Number.isFinite(value)) throw new TypeError("amount must be a decimal string");
      if (entries.some((e) => e.id === id)) return balance;
      entries.push({ id, value });
      balance += value;
      return balance;
    },
    balance() {
      return balance;
    },
    entries() {
      return entries.slice();
    },
  };
}

// Renders an amount with two decimals, rounding half up.
export function formatCents(amount) {
  const value = typeof amount === "number" ? amount : parseFloat(amount);
  if (!Number.isFinite(value)) throw new TypeError("amount must be numeric");
  return (Math.round(value * 100) / 100).toFixed(2);
}
