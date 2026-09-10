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
  let mills;
  try {
    mills = toMills(amount);
  } catch {
    throw new TypeError("amount must be numeric");
  }
  const abs = Math.abs(mills);
  const cents = Math.floor(abs / 10) + (abs % 10 >= 5 ? 1 : 0);
  return `${mills < 0 ? "-" : ""}${Math.floor(cents / 100)}.${String(cents % 100).padStart(2, "0")}`;
}
