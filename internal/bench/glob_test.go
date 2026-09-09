package bench

import (
	"path/filepath"
	"testing"
)

func TestResolveCasesGlob(t *testing.T) {
	cases := []struct{ name, benchDir, glob, want string }{
		{"bare name resolves under bench/cases", "bench", "n0*", filepath.Join("bench", "cases", "n0*")},
		{"star alone means every case", "bench", "*", filepath.Join("bench", "cases", "*")},
		{"a path is used as given", "bench", "bench/cases/n01-csv-rfc4180", "bench/cases/n01-csv-rfc4180"},
		{"relative path with separator is used as given", "other", "./x/*", "./x/*"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveCasesGlob(c.benchDir, c.glob); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}
