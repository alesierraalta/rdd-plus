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

// Parses one CSV line; a quoted field may contain separators, and a doubled quote stands for a literal quote.
export function parseLine(line) {
  if (typeof line !== "string") throw new TypeError("line must be a string");
  if (line === "") return [];
  const fields = [];
  let i = 0;
  for (;;) {
    while (line[i] === " ") i++;
    if (line[i] === '"') {
      let value = "";
      i++;
      for (;;) {
        if (i >= line.length) throw new SyntaxError("unterminated quoted field");
        if (line[i] === '"') {
          if (line[i + 1] === '"') {
            value += '"';
            i += 2;
            continue;
          }
          i++;
          break;
        }
        value += line[i++];
      }
      while (line[i] === " ") i++;
      fields.push(value);
    } else {
      const end = line.indexOf(",", i);
      const raw = end === -1 ? line.slice(i) : line.slice(i, end);
      fields.push(raw.trim());
      i = end === -1 ? line.length : end;
    }
    if (i >= line.length) return fields;
    if (line[i] !== ",") throw new SyntaxError(`unexpected character at ${i}`);
    i++;
    if (i === line.length) {
      fields.push("");
      return fields;
    }
  }
}
