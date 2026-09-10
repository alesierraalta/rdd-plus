package bench

import (
	"sort"
	"strings"
	"sync"
	"testing"
)

// Running cases side by side must not change what the run reports: the aggregate is ordered by
// case, not by whichever finished first.
func TestScheduleKeepsCaseOrderWhateverTheCompletionOrder(t *testing.T) {
	names := []string{"a", "b", "c", "d", "e"}
	for _, workers := range []int{1, 2, 5, 9} {
		var mu sync.Mutex
		var started int
		var peak int
		got := schedule(names, workers, func(i int, name string) Result {
			mu.Lock()
			started++
			if started > peak {
				peak = started
			}
			mu.Unlock()
			defer func() { mu.Lock(); started--; mu.Unlock() }()
			return Result{Case: name, Run: i}
		})
		if len(got) != len(names) {
			t.Fatalf("workers %d: %d results", workers, len(got))
		}
		for i, r := range got {
			if r.Case != names[i] || r.Run != i {
				t.Fatalf("workers %d: result %d is %+v", workers, i, r)
			}
		}
		if want := min(workers, len(names)); peak > want {
			t.Fatalf("workers %d: %d ran at once", workers, peak)
		}
	}
}

// One case blowing up must not take the others down or leave a hole in the aggregate: it is
// recorded as a failed case, exactly like an agent that never ran.
func TestScheduleSurvivesACaseThatPanics(t *testing.T) {
	got := schedule([]string{"a", "b", "c"}, 3, func(i int, name string) Result {
		if name == "b" {
			panic("boom")
		}
		return Result{Case: name, Found: 1}
	})
	if len(got) != 3 {
		t.Fatalf("%d results", len(got))
	}
	if got[0].Found != 1 || got[2].Found != 1 {
		t.Fatalf("the other cases must survive: %+v", got)
	}
	if !got[1].Failed || got[1].Case != "b" {
		t.Fatalf("the panicking case must be recorded as failed: %+v", got[1])
	}
	if !strings.Contains(got[1].FailReason, "boom") {
		t.Fatalf("the reason must say what happened: %q", got[1].FailReason)
	}
}

func TestWorkersFor(t *testing.T) {
	cases := map[int]int{0: 1, -3: 1, 1: 1, 4: 4}
	keys := make([]int, 0, len(cases))
	for k := range cases {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, in := range keys {
		if got := workersFor(in); got != cases[in] {
			t.Errorf("workersFor(%d) = %d, want %d", in, got, cases[in])
		}
	}
}
