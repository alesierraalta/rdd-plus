// Parses a decimal amount into whole cents, rounding half up on the third decimal.
function toCents(amount) {
  const text = typeof amount === "number" ? String(amount) : amount;
  const m = typeof text === "string" && /^\s*([+-]?)(\d+)(?:\.(\d+))?\s*$/.exec(text);
  if (!m) throw new TypeError("amount must be a decimal string");
  const [, sign, whole, frac = ""] = m;
  const digits = (frac + "000").slice(0, 3);
  let cents = Number(whole) * 100 + Number(digits.slice(0, 2));
  if (digits[2] >= "5") cents += 1;
  return sign === "-" ? -cents : cents;
}

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
  let cents;
  try {
    cents = toCents(amount);
  } catch {
    throw new TypeError("amount must be numeric");
  }
  const abs = Math.abs(cents);
  return `${cents < 0 ? "-" : ""}${Math.floor(abs / 100)}.${String(abs % 100).padStart(2, "0")}`;
}
