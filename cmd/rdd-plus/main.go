// rdd-plus is a deterministic companion for the testing discipline: it installs the skills,
// wires the Stop hook that keeps them invoked, and reports what the environment can do.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/alesierraalta/rdd-plus/internal/doctor"
	"github.com/alesierraalta/rdd-plus/internal/gate"
	"github.com/alesierraalta/rdd-plus/internal/sync"
)

// Version is overridable at build time: -ldflags "-X main.Version=v1.2.3".
var Version = "dev"

const usage = `usage: rdd-plus <command> [flags]

commands:
  gate     Stop hook: read the hook payload on stdin, decide, log, emit feedback
  sync     install the embedded skills and wire the gate into settings.json
  doctor   report installed skills, the hook wiring, and optional capabilities
  version  print the version

flags shared by gate, sync, doctor:
  --config-dir <dir>   Claude config directory (default: ~/.claude)
`

func defaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude"
	}
	return filepath.Join(home, ".claude")
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "gate":
		os.Exit(runGate(os.Args[2:]))
	case "sync":
		os.Exit(runSync(os.Args[2:]))
	case "doctor":
		os.Exit(runDoctor(os.Args[2:]))
	case "version":
		fmt.Println(Version)
		os.Exit(0)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}

func runGate(args []string) int {
	fs := flag.NewFlagSet("gate", flag.ContinueOnError)
	configDir := fs.String("config-dir", "", "Claude config directory")
	if err := fs.Parse(args); err != nil {
		return 0 // a hook must never break the turn, even on a bad flag
	}
	return gate.Run(os.Stdin, os.Stdout, gate.DefaultLogPath(*configDir), time.Now())
}

func runSync(args []string) int {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	configDir := fs.String("config-dir", defaultConfigDir(), "Claude config directory")
	dryRun := fs.Bool("dry-run", false, "print the plan and write nothing")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	bin, err := os.Executable()
	if err == nil {
		bin, _ = filepath.Abs(bin)
	}
	report, err := sync.Sync(*configDir, bin, sync.Options{DryRun: *dryRun})
	fmt.Print(report.String())
	if err != nil {
		fmt.Fprintln(os.Stderr, "sync:", err)
		return 1
	}
	return 0
}

func runDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	configDir := fs.String("config-dir", defaultConfigDir(), "Claude config directory")
	asJSON := fs.Bool("json", false, "machine-readable output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	report := doctor.Run(*configDir, exec.LookPath)
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	} else {
		fmt.Print(report.String())
	}
	if report.Healthy {
		return 0
	}
	return 1
}
