// Records of one upstream batch; integer ids are kept as BigInt so 64-bit values survive parsing.
export function parseRecords(json) {
  const doc = JSON.parse(json, (key, value, context) =>
    key === "id" && typeof value === "number" && Number.isInteger(value) && context?.source ? BigInt(context.source) : value,
  );
  if (!doc || !Array.isArray(doc.records)) throw new TypeError("batch must contain a records array");
  return doc.records;
}

// First record of every distinct id, in arrival order.
export function dedupeById(records) {
  const seen = new Map();
  for (const record of records) {
    if (record.id == null) throw new TypeError("record without id");
    if (!seen.has(record.id)) seen.set(record.id, record);
  }
  return [...seen.values()];
}
