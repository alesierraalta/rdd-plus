const ISO_DAY = /^(\d{4})-(\d{2})-(\d{2})$/;

// Calendar day as YYYY-MM-DD; a Date is read in local time so the caller's midnight stays on its own day.
function dayKey(value) {
  if (typeof value === "string" && ISO_DAY.test(value)) return value;
  const d = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(d.getTime())) throw new TypeError("onDate must be a valid date");
  const pad = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

// Active when the day falls inside [start, end], both inclusive.
export function isActive(sub, onDate) {
  const day = dayKey(onDate);
  return day >= dayKey(sub.start) && day <= dayKey(sub.end);
}

// Same calendar day one year later.
export function nextRenewal(dateStr) {
  const d = new Date(dateStr);
  if (Number.isNaN(d.getTime())) throw new TypeError("dateStr must be YYYY-MM-DD");
  d.setFullYear(d.getFullYear() + 1);
  return d.toISOString().slice(0, 10);
}
