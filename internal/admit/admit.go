package admit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/alesierraalta/rdd-plus/internal/evidence"
	"github.com/alesierraalta/rdd-plus/internal/plan"
)

// Runner runs one command in dir and returns its combined output.
type Runner func(ctx context.Context, dir, command string) (string, error)

// Request is the admission run's shape: what to read, whether to execute, and how to narrow or record it.
type Request struct {
	Path    string
	Execute bool
	Timeout time.Duration
	Only    []string
	Record  []string
	Sandbox bool
	Image   string
	Dir     string
}

// Deps are the process boundaries injected by the caller. Out and Err must be non-nil; Run must be non-nil
// whenever a row can execute, and SandboxRunner and Replay are only reached when the request asks for the
// sandbox, so a host-mode caller may leave both empty.
type Deps struct {
	Run           Runner
	SandboxRunner func(image, mount string) Runner
	Replay        func(run Runner, timeout time.Duration) func(plan.Mutation, string, string) evidence.ReplayResult
	Out           io.Writer
	Err           io.Writer
}

// Run admits the requested Evidence rows and returns the process exit code: 0 when no row was refused, 1 when
// a row was refused or the plan could not be read or written, and 2 when a flag precondition or a named id is
// wrong. It writes the row lines and the summary to deps.Out and every refusal to deps.Err.
func Run(req Request, deps Deps) int {
	onlyIDs := admitIDs(req.Only)
	recordIDs := admitIDs(req.Record)
	if len(recordIDs) > 0 && !req.Execute {
		fmt.Fprintln(deps.Err, "plan admit: --record requires --execute: recording pins the observation this run makes, and a dry run makes none")
		return 2
	}
	if req.Sandbox && !req.Execute {
		fmt.Fprintln(deps.Err, "plan admit: --sandbox requires --execute: a dry run executes nothing, so there is nothing to confine")
		return 2
	}
	mode := evidence.ModeHost
	if req.Sandbox {
		mode = evidence.ModeSandbox
	}
	raw, err := os.ReadFile(req.Path)
	if err != nil {
		fmt.Fprintln(deps.Err, "plan admit:", err)
		return 1
	}
	ledgerRows := plan.Ledger(string(raw))
	// A named row that does not exist is a mistake the user must see: without this, `--record ZZZ` is a
	// silent no-op whose exit code depends only on the other rows, and `--only ZZZ` silently narrows to
	// nothing. Both flags are checked against the ids the ledger actually carries.
	for _, flag := range []struct {
		name string
		ids  []string
	}{{"--only", onlyIDs}, {"--record", recordIDs}} {
		if id := unknownID(flag.ids, ledgerRows); id != "" {
			return ledgerIDError(deps.Err, flag.name, id, ledgerRows)
		}
	}
	runner := deps.Run
	if req.Sandbox {
		runner = deps.SandboxRunner(req.Image, SandboxReadOnly)
	}
	evidenceDeps := evidence.Deps{Run: runner}
	// The replay is wired exactly where a tree the tool owns exists: a sandbox stages a copy of the tree git knows,
	// edits the copy, runs against it and puts the file back, so a row that claims its own command is falsifiable
	// has that claim checked instead of admitted unchecked. The host mode has no such copy and leaves Replay nil,
	// which refuses such a row.
	if req.Sandbox {
		evidenceDeps.Replay = deps.Replay(deps.SandboxRunner(req.Image, SandboxWritable), req.Timeout)
	}
	results := evidence.Admit(ledgerRows, evidence.Options{
		Execute: req.Execute,
		Dir:     req.Dir,
		Timeout: req.Timeout,
		Mode:    mode,
		Only:    onlyIDs,
		Record:  recordIDs,
	}, evidenceDeps)

	admitted, wouldRun, refused := 0, 0, 0
	for _, r := range results {
		switch r.Verdict {
		case evidence.VerdictAdmitted:
			admitted++
			fmt.Fprintf(deps.Out, "%s  %s  %s  %s  %d lines\n", r.ID, r.Verdict, r.Command, r.Digest, r.Lines)
		case evidence.VerdictWouldRun:
			wouldRun++
			fmt.Fprintf(deps.Out, "%s  %s  %s\n", r.ID, r.Verdict, r.Command)
		default:
			// A refusal prints the human sentence beside its machine reason, so a run that stops here
			// still says what to change and what a caller can branch on.
			refused++
			fmt.Fprintf(deps.Out, "%s  %s  %s  [%s]\n", r.ID, r.Verdict, r.Detail, r.Reason)
		}
	}

	// Recording applies every edit to the document in memory and writes the file once, so a run that
	// records three rows leaves one write. A splice that refuses aborts the whole write rather than
	// skipping the row: a half-recorded ledger is the state this feature exists to prevent.
	recording := map[string]bool{}
	for _, id := range recordIDs {
		recording[id] = true
	}
	doc, recorded := string(raw), 0
	var recordedLines []string
	for _, r := range results {
		if r.Verdict != evidence.VerdictAdmitted || !recording[r.ID] {
			continue
		}
		updated, err := plan.RecordDigest(doc, r.ID, r.Digest)
		if err != nil {
			fmt.Fprintln(deps.Err, "plan admit:", err)
			return 1
		}
		// The mode is recorded beside the digest, because a digest without the mode it was taken in is not
		// checkable. A plan written before the Mode column existed cannot carry one; an empty cell there
		// already means the host, so a host recording is still true and a sandbox recording is refused rather
		// than written as a claim the plan cannot hold.
		withMode, err := plan.RecordMode(updated, r.ID, mode)
		if err != nil {
			if mode != evidence.ModeHost || !errors.Is(err, plan.ErrNoColumn) {
				fmt.Fprintln(deps.Err, "plan admit:", err)
				return 1
			}
		} else {
			updated = withMode
		}
		doc = updated
		recorded++
		recordedLines = append(recordedLines, fmt.Sprintf("%s  RECORDED  %s  (%s mode)\n", r.ID, r.Digest, mode))
	}
	if recorded > 0 {
		// The document was read before the rows ran, and a row can take minutes. The digests this run observed
		// belong to the plan it read, so writing them over a file another writer has changed since would erase
		// that edit and pin a claim against a plan that no longer exists. The run refuses instead of
		// overwriting a document it did not read.
		now, err := os.ReadFile(req.Path)
		if err != nil {
			fmt.Fprintln(deps.Err, "plan admit:", err)
			return 1
		}
		if string(now) != string(raw) {
			fmt.Fprintf(deps.Err, "plan admit: %s changed while the rows ran; nothing was written (the digests this run observed belong to the plan it read, not to the one on disk now)\n", req.Path)
			return 1
		}
		info, err := os.Stat(req.Path)
		if err != nil {
			fmt.Fprintln(deps.Err, "plan admit:", err)
			return 1
		}
		if err := os.WriteFile(req.Path, []byte(doc), info.Mode().Perm()); err != nil {
			fmt.Fprintln(deps.Err, "plan admit:", err)
			return 1
		}
	}
	for _, line := range recordedLines {
		fmt.Fprint(deps.Out, line)
	}
	fmt.Fprintf(deps.Out, "%d rows: %d admitted, %d would run, %d refused, %d recorded\n", len(results), admitted, wouldRun, refused, recorded)
	if refused > 0 {
		return 1
	}
	return 0
}

// unknownID returns the first id in ids that names no ledger row, or "" when every id names one. An
// empty list names nothing and is never a mistake.
func unknownID(ids []string, rows []plan.LedgerRow) string {
	if len(ids) == 0 {
		return ""
	}
	known := make(map[string]bool, len(rows))
	for _, row := range rows {
		known[row.ID] = true
	}
	for _, id := range ids {
		if !known[id] {
			return id
		}
	}
	return ""
}

// ledgerIDError reports an id a flag named that the ledger does not carry, and lists every id it does
// carry, so a typo is a message on stderr and exit 2 instead of a silent narrowing.
func ledgerIDError(w io.Writer, flag, id string, rows []plan.LedgerRow) int {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	available := "(none)"
	if len(ids) > 0 {
		available = strings.Join(ids, ", ")
	}
	fmt.Fprintf(w, "plan admit: %s names %q, which is not a row in the Evidence ledger; available ids: %s\n", flag, id, available)
	return 2
}

// admitIDs turns the comma-separated flag values into the ids Admit narrows to. An empty list narrows
// nothing, which is why the empty string and a list of blanks both come back empty.
func admitIDs(parts []string) []string {
	var ids []string
	for _, part := range parts {
		if id := strings.TrimSpace(part); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}
