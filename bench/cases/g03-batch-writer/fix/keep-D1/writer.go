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

// WriteAll sends records to the sink in batches.
func WriteAll(sink Sink, records []Record) (Result, error) {
	written := 0
	for i := 0; i+BatchSize <= len(records); i += BatchSize {
		if err := sink.Write(records[i : i+BatchSize]); err != nil {
			return Result{Written: written}, err
		}
		written += BatchSize
	}
	return Result{Written: written}, nil
}
