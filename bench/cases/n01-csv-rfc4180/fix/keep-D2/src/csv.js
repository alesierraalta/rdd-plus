// Renders one record as a CSV line; a field containing the separator, a quote, or a line break is quoted with inner quotes doubled.
export function renderLine(fields) {
  if (!Array.isArray(fields)) throw new TypeError("fields must be an array");
  return fields
    .map((field) => {
      const value = String(field);
      return /[",\r\n]/.test(value) ? `"${value.replace(/"/g, '""')}"` : value;
    })
    .join(",");
}

// Parses one CSV line; inside a quoted field a doubled quote stands for a literal quote.
export function parseLine(line) {
  if (typeof line !== "string") throw new TypeError("line must be a string");
  if (line === "") return [];
  return line.split(",").map((raw) => {
    const field = raw.trim();
    if (field.startsWith('"') && field.endsWith('"') && field.length >= 2) {
      return field.slice(1, -1).replace(/""/g, '"');
    }
    return field;
  });
}
