# batchwriter

Bulk export of records to a downstream sink.

- `WriteAll(sink, records)` sends every record to the sink in chunks of `BatchSize`, in order,
  and returns a `Result` whose `Written` is the number of records the sink accepted.
- Every record is delivered exactly once when `WriteAll` returns without error.
- When the sink fails, `WriteAll` returns the sink's error and `Written` reports how many
  records had been accepted before the failure, so the caller can resume from there.
