// Records of one upstream batch.
export function parseRecords(json) {
  const doc = JSON.parse(json);
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
