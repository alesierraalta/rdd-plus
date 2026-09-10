package bench

import (
	"fmt"
	"path/filepath"
	"sync"
)

// workersFor turns a requested concurrency into a usable one; anything below 1 means sequential.
func workersFor(requested int) int {
	if requested < 1 {
		return 1
	}
	return requested
}

// schedule runs one job per item with at most workers running at once, and returns the results in
// the order of the items. Wall time is what concurrency buys; the reported run must not depend on
// which case happened to finish first.
func schedule[T any](items []T, workers int, job func(i int, item T) Result) []Result {
	out := make([]Result, len(items))
	sem := make(chan struct{}, workersFor(workers))
	var wg sync.WaitGroup
	for i, item := range items {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			// One case must never take the run down: a crash is recorded like a failed agent.
			defer func() {
				if r := recover(); r != nil {
					out[i] = Result{Case: caseName(item), Failed: true, FailReason: fmt.Sprintf("panic: %v", r)}
				}
			}()
			out[i] = job(i, item)
		}()
	}
	wg.Wait()
	return out
}

// caseName is the best label available for an item the job never got to name.
func caseName(item any) string {
	switch v := item.(type) {
	case string:
		return filepath.Base(v)
	default:
		return fmt.Sprint(v)
	}
}
