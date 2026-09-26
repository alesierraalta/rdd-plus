package doctor

import (
	"os"
	"testing"
)

// TestMain points the installation state at a throwaway directory, so no test in this package
// can read or rewrite the state of the rdd-plus installed on the machine running the suite.
func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "rdd-plus-state-")
	if err != nil {
		panic(err)
	}
	os.Setenv("RDD_PLUS_HOME", root)
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}
