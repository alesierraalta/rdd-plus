// Parses a decimal amount into whole thousandths, the finest unit the contract admits, rounding half up beyond it.
function toMills(amount) {
  const text = typeof amount === "number" ? String(amount) : amount;
  const m = typeof text === "string" && /^\s*([+-]?)(\d+)(?:\.(\d+))?\s*$/.exec(text);
  if (!m) throw new TypeError("amount must be a decimal string");
  const [, sign, whole, frac = ""] = m;
  const digits = (frac + "0000").slice(0, 4);
  let mills = Number(whole) * 1000 + Number(digits.slice(0, 3));
  if (digits[3] >= "5") mills += 1;
  return sign === "-" ? -mills : mills;
}

// Keeps the entries and the running balance of one account, in whole thousandths.
export function createLedger() {
  let balance = 0;
  const entries = [];
  return {
    post(id, amount) {
      const mills = toMills(amount);
      if (entries.some((e) => e.id === id)) return balance / 1000;
      entries.push({ id, value: mills / 1000 });
      balance += mills;
      return balance / 1000;
    },
    balance() {
      return balance / 1000;
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
