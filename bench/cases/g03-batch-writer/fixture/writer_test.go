package batchwriter

import "testing"

type memSink struct {
	batches [][]Record
}

func (m *memSink) Write(batch []Record) error {
	copied := make([]Record, len(batch))
	copy(copied, batch)
	m.batches = append(m.batches, copied)
	return nil
}

func records(n int) []Record {
	out := make([]Record, n)
	for i := range out {
		out[i] = Record{ID: i + 1}
	}
	return out
}

func TestOneFullBatch(t *testing.T) {
	sink := &memSink{}
	res, err := WriteAll(sink, records(100))
	if err != nil {
		t.Fatal(err)
	}
	if res.Written != 100 || len(sink.batches) != 1 {
		t.Fatalf("written=%d batches=%d", res.Written, len(sink.batches))
	}
}

func TestTwoFullBatchesInOrder(t *testing.T) {
	sink := &memSink{}
	res, err := WriteAll(sink, records(200))
	if err != nil {
		t.Fatal(err)
	}
	if res.Written != 200 || len(sink.batches) != 2 {
		t.Fatalf("written=%d batches=%d", res.Written, len(sink.batches))
	}
	if sink.batches[0][0].ID != 1 || sink.batches[1][0].ID != 101 {
		t.Fatal("batches out of order")
	}
}

func TestEmptyInput(t *testing.T) {
	sink := &memSink{}
	res, err := WriteAll(sink, nil)
	if err != nil || res.Written != 0 || len(sink.batches) != 0 {
		t.Fatalf("empty input: written=%d batches=%d err=%v", res.Written, len(sink.batches), err)
	}
}
