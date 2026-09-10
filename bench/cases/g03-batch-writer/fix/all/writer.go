package batchwriter

// BatchSize is the number of records sent per sink call.
const BatchSize = 100

// Record is one exported row.
type Record struct {
	ID   int
	Body string
}

// Sink receives batches in order.
type Sink interface {
	Write(batch []Record) error
}

// Result reports how many records the sink accepted.
type Result struct {
	Written int
}

// WriteAll sends records to the sink in batches; the last batch may be shorter than BatchSize.
func WriteAll(sink Sink, records []Record) (Result, error) {
	written := 0
	for i := 0; i < len(records); i += BatchSize {
		end := min(i+BatchSize, len(records))
		if err := sink.Write(records[i:end]); err != nil {
			return Result{Written: written}, err
		}
		written += end - i
	}
	return Result{Written: written}, nil
}
