// Active when the day falls inside [start, end], both inclusive.
export function isActive(sub, onDate) {
  const start = new Date(sub.start);
  const end = new Date(sub.end);
  const day = onDate instanceof Date ? onDate : new Date(onDate);
  if (Number.isNaN(day.getTime())) throw new TypeError("onDate must be a valid date");
  return day >= start && day <= end;
}

// Same calendar day one year later.
export function nextRenewal(dateStr) {
  const d = new Date(dateStr);
  if (Number.isNaN(d.getTime())) throw new TypeError("dateStr must be YYYY-MM-DD");
  d.setFullYear(d.getFullYear() + 1);
  return d.toISOString().slice(0, 10);
}
