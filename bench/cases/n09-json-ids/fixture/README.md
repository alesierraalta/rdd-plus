# json-ids

Ingests record batches from the upstream event service.

- Batches arrive as JSON documents: `{ "records": [ { "id": <64-bit integer>, ... } ] }`.
  Upstream ids are 64-bit integers assigned by a sequence, so consecutive records have
  consecutive ids.
- `parseRecords(json)` returns the records of a batch; `dedupeById(records)` keeps the first
  record of every distinct id, in arrival order.
- Two records with different upstream ids are never merged.
