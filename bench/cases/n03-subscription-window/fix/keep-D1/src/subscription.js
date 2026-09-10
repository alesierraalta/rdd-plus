const ISO_DAY = /^(\d{4})-(\d{2})-(\d{2})$/;

// Active when the day falls inside [start, end], both inclusive.
export function isActive(sub, onDate) {
  const start = new Date(sub.start);
  const end = new Date(sub.end);
  const day = onDate instanceof Date ? onDate : new Date(onDate);
  if (Number.isNaN(day.getTime())) throw new TypeError("onDate must be a valid date");
  return day >= start && day <= end;
}

// Same calendar day one year later; Feb 29 renews on Feb 28 when the next year is not leap.
export function nextRenewal(dateStr) {
  const m = typeof dateStr === "string" && ISO_DAY.exec(dateStr);
  if (!m || Number.isNaN(new Date(dateStr).getTime())) throw new TypeError("dateStr must be YYYY-MM-DD");
  const year = Number(m[1]) + 1;
  const month = Number(m[2]);
  const day = Math.min(Number(m[3]), new Date(Date.UTC(year, month, 0)).getUTCDate());
  const pad = (n) => String(n).padStart(2, "0");
  return `${year}-${pad(month)}-${pad(day)}`;
}
