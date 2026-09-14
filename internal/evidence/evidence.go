// Package evidence decides which rows of a plan's Evidence ledger are admitted as observed. A row is
// admitted only when the single command its `Admit` cell declares actually ran and the output it
// produced matches the digest the row pins. The decision is deterministic and dependency-injected: the
// same ledger and the same observed output give the same verdicts. Nothing here starts a process of its
// own or writes a file.
package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/alesierraalta/rdd-plus/internal/plan"
)

// Options is the run's shape: what to execute, where, under what bound, and what to narrow or record.
type Options struct {
	// Execute runs the admitted commands. When false the run is a dry run: every runnable row is
	// reported as WOULD RUN and no command reaches the runner.
	Execute bool
	// Dir is the working directory every command runs in.
	Dir string
	// Timeout bounds one command. Zero leaves the command unbounded rather than inventing a deadline.
	Timeout time.Duration
	// Mode names where this run observes the command: "" and "host" mean this machine, "sandbox" means a
	// container. It is compared against the mode the row pinned, because the same command digests
	// differently in the two places and a pin that does not say where it was taken cannot be checked.
	Mode string
	// Only, when non-empty, admits only the rows carrying these ids. Every other row is left out of the
	// result entirely, because a narrowed run must not report rows it was never asked about.
	Only []string
	// Record names the rows whose freshly observed digest is returned as the row's digest, over any
	// pinned value. It writes nothing: the caller owns the file, so a recording run can be read before
	// its digests land in the ledger.
	Record []string
}

// Deps is the injected runner. Unit tests pass a fake, so no unit test starts a process.
type Deps struct {
	Run func(ctx context.Context, dir, command string) (output string, err error)
	// Replay applies a row's mutation, runs the command against the mutated tree and then against the tree the
	// edit was undone in, and returns both observations. It is nil when the caller has no tree it owns, which is
	// every mode but a sandbox; the admission then refuses a row that declares a mutation rather than admitting it
	// with its falsifiability claim unchecked. This package judges red and green; the caller owns how the tree was
	// staged, how it was put back, and the deadline on each of the two runs it makes.
	Replay func(mutation plan.Mutation, dir, command string) ReplayResult
}

// ReplayResult is the two observations a replay owes: the command against the mutated tree, which must go red,
// and the same command against the tree the edit was undone in, which must go green.
type ReplayResult struct {
	MutatedOutput  string
	MutatedErr     error
	RestoredOutput string
	RestoredErr    error
}

// Verdict is what a row earned.
type Verdict string

const (
	// VerdictAdmitted means the command ran and its output is the observation the row claims.
	VerdictAdmitted Verdict = "ADMITTED"
	// VerdictWouldRun means the row is runnable and this run did not run it.
	VerdictWouldRun Verdict = "WOULD RUN"
	// VerdictRefused means the row may not be admitted, and Reason says why.
	VerdictRefused Verdict = "REFUSED"
)

// The reason codes a refusal carries. They are part of the machine surface and a caller may branch on
// them; the human sentence beside them is free to change.
const (
	ReasonLabelNotObservado = "label-not-observado"
	ReasonNoAdmitCommand    = "no-admit-command"
	ReasonMultipleCommands  = "admit-multiple-commands"
	ReasonHasMarkup         = "admit-has-markup"
	ReasonHasSubstitution   = "admit-has-substitution"
	ReasonHasRedirection    = "admit-has-redirection"
	ReasonHasPlaceholder    = "admit-has-placeholder"
	ReasonMalformedRow      = "malformed-row"
	ReasonNormalizeInvalid  = "normalize-invalid"
	ReasonUnstableOutput    = "unstable-output"
	ReasonCommandFailed     = "command-failed"
	ReasonTimeout           = "timeout"
	ReasonEmptyOutput       = "empty-output"
	ReasonDigestMismatch    = "digest-mismatch"
	ReasonDigestMissing     = "digest-missing"
	ReasonModeMismatch      = "mode-mismatch"
	ReasonSandboxReadOnly   = "sandbox-read-only"
	ReasonMisconfigured     = "sandbox-misconfigured"
	ReasonNoNetwork         = "network-unavailable"
	ReasonMutationNotReplay = "mutation-not-replayed"
	ReasonMutationNotRed    = "mutation-not-red"
	ReasonMutationNotGreen  = "mutation-not-green"
)

// Refusal lets a runner say why a command could not run, when the answer is about the runner's own
// environment rather than about the command: a sandbox that refused a write, a container that never
// started, a network the mode disables. The reason code travels with the error so this package keeps owning
// the machine surface while the runner decides what it actually knows.
type Refusal struct {
	Reason string
	Detail string
}

func (r Refusal) Error() string { return r.Detail }

// ModeHost and ModeSandbox name the two places a row's command can be observed. The mode is part of what a
// pin means, because the same command digests differently in a container than on this machine.
const (
	ModeHost    = "host"
	ModeSandbox = "sandbox"
)

// modeOf names the mode a run or a row is in. An empty cell means the host, which is where every pin taken
// before the mode existed was taken, so an old row keeps the meaning it always had.
func modeOf(mode string) string {
	if mode == "" {
		return ModeHost
	}
	return mode
}

// RowResult is one row's outcome: a machine-stable reason for a refusal and a human sentence for stderr.
type RowResult struct {
	ID      string
	Verdict Verdict
	Command string // the resolved command, when the row declares one
	Reason  string // machine-stable reason code; empty when the row was not refused
	Detail  string // human sentence, for stderr
	Digest  string // "sha256:<hex>" of the freshly observed output, when the command produced one
	Lines   int    // lines in the normalized observed output, when the command produced one
}

// observadoLabel is the only label a row is admitted under, spelled exactly as the ledger spells it: a
// `razonado` row is a hypothesis and belongs under Hypotheses, never in the ledger.
const observadoLabel = "observado"

// absentCommand is the whole cell a row carries when it declares no command yet: the empty cell and the
// placeholder tokens a plan author leaves behind while the row is still a hypothesis. The match is the
// whole cell, so a command that merely contains one of these words still runs.
var absentCommand = map[string]bool{
	"": true, "-": true, "—": true, "--": true,
	"n/a": true, "na": true, "none": true, "tbd": true, "todo": true,
}

// commandMarkers is the one explicit table of shell metacharacters the resolved command may not carry
// as shell syntax, each mapped to the reason it earns. The table's order is the check order: a cell
// that carries several of them is refused for the first one listed. Sequencing and grouping come
// first, because a cell that chains, pipelines, backgrounds, or groups is more than one command;
// command substitution next, because it starts a second process inside the cell; redirection last,
// because it writes or reads a file the pinned digest cannot cover. Redirection is the `late` entry,
// checked after the placeholder rule, so a cell that still holds an author's `<file>` names that
// placeholder instead of the redirection. A marker inside single quotes (or, for everything but `$(`,
// inside double quotes) is literal text the shell never reads as syntax, so `awk '{print $1}'` and
// `grep -E 'a|b'` still run. Extend this table, never a heuristic.
var commandMarkers = []struct {
	marker string
	reason string
	late   bool // checked after the placeholder rule
}{
	{"\n", ReasonMultipleCommands, false},
	{";", ReasonMultipleCommands, false},
	{"&", ReasonMultipleCommands, false},
	{"|", ReasonMultipleCommands, false},
	{"$(", ReasonHasSubstitution, false},
	{"(", ReasonMultipleCommands, false},
	{")", ReasonMultipleCommands, false},
	{"{", ReasonMultipleCommands, false},
	{"}", ReasonMultipleCommands, false},
	{"<", ReasonHasRedirection, true},
	{">", ReasonHasRedirection, true},
}

// A placeholder is a value only the row's author can supply: an angle span like `<tmp>`, or a variable
// expansion like `$HOME` or `${VAR}`. The variable must start with an identifier character, so a
// positional parameter such as `awk '{print $1}'` is not mistaken for one.
var (
	anglePlaceholder = regexp.MustCompile(`<[^<>\n]*>`)
	varPlaceholder   = regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z0-9_]*\}|\$[A-Za-z_][A-Za-z0-9_]*`)
)

// Admit decides every row named by opts.Only, or the whole ledger when Only is empty, in document
// order. Truncated output in RowResult is the fresh one, produced by this run.
func Admit(rows []plan.LedgerRow, opts Options, deps Deps) []RowResult {
	only := idSet(opts.Only)
	record := idSet(opts.Record)
	results := make([]RowResult, 0, len(rows))
	for _, row := range rows {
		if len(only) > 0 && !only[row.ID] {
			continue
		}
		results = append(results, admitRow(row, opts, deps, record[row.ID]))
	}
	return results
}

// admitRow is the whole decision for one row. The checks are owed in exactly this order and the
// earliest reason wins: that the row's cells line up with the header at all (malformed), what the row
// claims (label), what command its cell resolves to (absent), that the command is one command and not
// a sequence, a pipeline, a group, or a substitution (metacharacter), that the cell is a bare command
// and not prose (markup), that the command holds no unexpanded value (placeholder), that it does not
// redirect I/O (redirection), and only then what happened when it ran.
func admitRow(row plan.LedgerRow, opts Options, deps Deps, record bool) RowResult {
	result := RowResult{ID: row.ID}

	// A row whose cells do not line up with its header was cut by an unescaped `|`, so every column to
	// the right of the cut is read from the wrong cell — including the Label, the last one. Nothing the
	// row says can be trusted, so it is refused before its content is believed.
	if row.HeaderCells > 0 && row.Cells != row.HeaderCells {
		return refused(result, ReasonMalformedRow, fmt.Sprintf(
			"evidence %s has %d cells but its header has %d: an unescaped `|` in the row splits a cell, so a command that carries a pipe must be written as `\\|` or, better, moved into a script; as written the row declares one thing and runs another",
			row.ID, row.Cells, row.HeaderCells))
	}

	if row.Label != observadoLabel {
		return refused(result, ReasonLabelNotObservado, fmt.Sprintf(
			"evidence %s is labelled %q: a row the ledger does not label %s is not an observation, so nothing may be admitted from it",
			row.ID, row.Label, observadoLabel))
	}

	// Resolve the cell first: the live ledger writes its commands inside backticks, so one whole
	// wrapping span is the ordinary spelling of a command and is stripped. Resolution is not a check,
	// so the reasons below still fire in the order the contract owes them.
	cell := strings.TrimSpace(row.Admit)
	command, _ := admitSpan(cell)
	if absentCommand[strings.ToLower(command)] {
		return refused(result, ReasonNoAdmitCommand, fmt.Sprintf(
			"evidence %s declares no command in its Admit cell (%q is a placeholder, not a command)", row.ID, command))
	}

	// One command whose output is the observation: anything that chains, pipelines, backgrounds,
	// groups, or substitutes a command is refused, because the tool cannot tell what a second process
	// will do with the stream. Redirection is the tail of the table and runs after the placeholder rule.
	if marker, reason, ok := commandMarker(command, false); ok {
		return refused(result, reason, markerDetail(row.ID, command, marker, reason))
	}
	if strings.Contains(command, "`") {
		return refused(result, ReasonHasMarkup, fmt.Sprintf(
			"evidence %s carries backticks in its Admit cell (%q) that are not one wrapping span: Admit carries one bare command, unlike Executed, which is prose a human reads and may quote with backticks",
			row.ID, cell))
	}
	if placeholder := commandPlaceholder(command); placeholder != "" {
		return refused(result, ReasonHasPlaceholder, fmt.Sprintf(
			"evidence %s admits %q, which still holds the unexpanded placeholder %q: the row runs only once its author fills it in",
			row.ID, command, placeholder))
	}
	if marker, reason, ok := commandMarker(command, true); ok {
		return refused(result, reason, markerDetail(row.ID, command, marker, reason))
	}

	// A row whose Normalize expression does not compile is a defect in the row, not in the command, so it
	// is reported before anything is spawned: discovering it after a run would spend the run to learn
	// something the row already said.
	if err := compileNormalize(row.Normalize); err != nil {
		return refused(result, ReasonNormalizeInvalid, fmt.Sprintf(
			"evidence %s declares Normalize %q, which does not compile: %v; a Normalize expression is a Go regular expression whose every match becomes X before hashing",
			row.ID, row.Normalize, err))
	}

	// The mode a pin was taken in is part of what the pin means, because the same command digests differently
	// in a container than on this machine. A row pinned in one mode and checked in the other would report a
	// digest mismatch that says nothing about why, so the mismatch is named before anything is spawned.
	// Recording is exempt, because recording is how a row's mode is set in the first place.
	if !record && modeOf(row.Mode) != modeOf(opts.Mode) {
		return refused(result, ReasonModeMismatch, fmt.Sprintf(
			"evidence %s was pinned in %s mode but this run observes in %s mode: a pin is only comparable inside the mode it was taken in, so rerun with --record %s to pin this one, or in %s mode",
			row.ID, modeOf(row.Mode), modeOf(opts.Mode), row.ID, modeOf(row.Mode)))
	}

	// A row that declares a mutation is claiming its own command is falsifiable. The claim is checked where the
	// tree is, and checked before anything is spawned: a cell that does not parse, a file that is not there, a line
	// that does not exist, text that is absent or ambiguous, and an edit that changes nothing are defects in the
	// row rather than in the command. A replay applies an edit and undoes it, so it needs a tree it owns, and a
	// claim that cannot be checked is refused rather than admitted unchecked.
	var mutation *plan.Mutation
	if strings.TrimSpace(row.Mutate) != "" {
		parsed, err := plan.ParseMutation(row.Mutate)
		if err != nil {
			return refused(result, mutationReason(err), fmt.Sprintf("evidence %s declares Mutate %q: %v", row.ID, row.Mutate, err))
		}
		if err := plan.ValidateMutation(opts.Dir, parsed); err != nil {
			return refused(result, mutationReason(err), fmt.Sprintf("evidence %s declares Mutate %q: %v", row.ID, row.Mutate, err))
		}
		mutation = &parsed
		if deps.Replay == nil {
			return refused(result, ReasonMutationNotReplay, fmt.Sprintf(
				"evidence %s declares a mutation and this run cannot replay it: a replay applies the edit and undoes it, so it needs a tree it owns, which only a sandbox provides; a row that claims its own command is falsifiable is not admitted with that claim unchecked",
				row.ID))
		}
	}

	result.Command = command
	if !opts.Execute {
		result.Verdict = VerdictWouldRun
		result.Detail = fmt.Sprintf("evidence %s was not executed: this is a dry run, so the row stays unadmitted", row.ID)
		return result
	}

	// The command runs twice under --execute. The second run is the stability probe, and it is the reason a
	// pin means anything: a pin recorded over output that moves is not a pin, it is a trap for the next
	// honest run. Both runs get the full timeout, so --timeout still bounds one command as documented.
	runOnce := func() (string, error) {
		ctx := context.Background()
		if opts.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
			defer cancel()
		}
		return deps.Run(ctx, opts.Dir, command)
	}
	runFailure := func(err error, suffix string) RowResult {
		// A runner that knows the failure is about its own environment rather than about the command says so
		// with a Refusal, and its reason code is reported as it stands: a sandbox that refused a write is not
		// a failing test, and calling it one would send the reader looking in the wrong place.
		var refusal Refusal
		if errors.As(err, &refusal) {
			return refused(result, refusal.Reason, fmt.Sprintf("evidence %s did not run%s: %s", row.ID, suffix, refusal.Detail))
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return refused(result, ReasonTimeout, fmt.Sprintf(
				"evidence %s hit the %s timeout%s: %v", row.ID, opts.Timeout, suffix, err))
		}
		return refused(result, ReasonCommandFailed, fmt.Sprintf("evidence %s failed to run%s: %v", row.ID, suffix, err))
	}

	output, err := runOnce()
	if err != nil {
		return runFailure(err, "")
	}

	// The command produced output, so the fresh observation is carried whether or not the row is
	// admitted: a refusal that says what was observed is worth more than one that says nothing.
	if strings.TrimSpace(output) == "" {
		return refused(result, ReasonEmptyOutput, fmt.Sprintf(
			"evidence %s produced no output, so there is nothing to observe", row.ID))
	}
	fresh, err := Digest(output, row.Normalize)
	if err != nil {
		return refused(result, ReasonNormalizeInvalid, fmt.Sprintf(
			"evidence %s declares Normalize %q, which does not compile: %v", row.ID, row.Normalize, err))
	}

	// The second observation is either another run of the same command or the restored half of a replay. A row
	// that declares a mutation has already earned its second run this way, so the probe it would have asked for
	// costs nothing extra, and the two greens it compares are stronger evidence than two runs of an unedited tree:
	// the edit was applied and undone between them.
	var again string
	if mutation != nil {
		replayed := deps.Replay(*mutation, opts.Dir, command)
		if replayed.MutatedErr == nil {
			return refused(result, ReasonMutationNotRed, fmt.Sprintf(
				"evidence %s declares the edit `%s` => `%s` at %s:%d and the command stayed green under it: a mutation its own row survives proves nothing about that row, so the claim is refused rather than recorded",
				row.ID, mutation.Old, mutation.New, mutation.Path, mutation.Line))
		}
		if replayed.RestoredErr != nil {
			return refused(result, ReasonMutationNotGreen, fmt.Sprintf(
				"evidence %s declares a mutation and the command failed after the edit was undone: %v; a replay that cannot put the tree back is not evidence of anything",
				row.ID, replayed.RestoredErr))
		}
		again = replayed.RestoredOutput
	} else {
		again, err = runOnce()
		if err != nil {
			return runFailure(err, " on its second run")
		}
	}
	second, err := Digest(again, row.Normalize)
	if err != nil {
		return refused(result, ReasonNormalizeInvalid, fmt.Sprintf(
			"evidence %s declares Normalize %q, which does not compile: %v", row.ID, row.Normalize, err))
	}
	if second != fresh {
		return refused(result, ReasonUnstableOutput, fmt.Sprintf(
			"evidence %s produced two different digests (%s then %s): the output moves between two identical runs, so the row cannot be pinned as it stands; declare a Normalize expression for the part that moves, or make the command deterministic",
			row.ID, fresh, second))
	}

	result.Digest, result.Lines = fresh, lineCount(output, row.Normalize)

	switch {
	case record:
		// Recording returns the fresh digest and writes nothing: pinning it in the ledger is the
		// caller's edit, so what was observed can be read before it becomes the pinned value.
		result.Verdict = VerdictAdmitted
		return result
	case row.Digest == "":
		return refused(result, ReasonDigestMissing, fmt.Sprintf(
			"evidence %s pins no digest, so the output it just produced is unpinned; rerun with --record %s to pin %s",
			row.ID, row.ID, fresh))
	case row.Digest != fresh:
		return refused(result, ReasonDigestMismatch, fmt.Sprintf(
			"evidence %s pins %s but the observed output digests to %s", row.ID, row.Digest, fresh))
	}
	result.Verdict = VerdictAdmitted
	return result
}

// refused marks a result refused without clearing what the row already resolved to.
func refused(result RowResult, reason, detail string) RowResult {
	result.Verdict = VerdictRefused
	result.Reason = reason
	result.Detail = detail
	return result
}

// mutationReason is the reason code a mutation defect carries. The grammar and its reasons belong to the package
// that owns the grammar, so this only names the fallback for an error that is not one of them.
func mutationReason(err error) string {
	var bad plan.MutationError
	if errors.As(err, &bad) {
		return bad.Reason
	}
	return plan.ReasonMutationMalformed
}

// admitSpan resolves an Admit cell to the command it declares and reports whether the whole cell was
// exactly one backtick-wrapped span. The live ledger writes every command inside backticks, so that
// one shape is the ordinary spelling of a command and its wrapper is stripped; every other use of a
// backtick is prose in a cell that owes a bare command, and is refused rather than guessed at.
func admitSpan(cell string) (command string, wrapped bool) {
	if len(cell) < 2 || !strings.HasPrefix(cell, "`") || !strings.HasSuffix(cell, "`") {
		return cell, false
	}
	inner := cell[1 : len(cell)-1]
	if strings.Contains(inner, "`") {
		return cell, false
	}
	return strings.TrimSpace(inner), true
}

// commandMarker returns the first marker in commandMarkers that the shell would read as syntax in
// command and that the given pass owns, together with the reason it earns. late selects the
// redirection pass, which runs after the placeholder rule; the other pass runs before it. The table's
// order is the check order, so `$(` is seen before the `(` it contains and a substitution is never
// reported as grouping.
func commandMarker(command string, late bool) (marker, reason string, ok bool) {
	active := activeMarkers(command)
	for _, m := range commandMarkers {
		if m.late == late && active[m.marker] {
			return m.marker, m.reason, true
		}
	}
	return "", "", false
}

// activeMarkers walks the command once and returns the markers the shell would read as syntax in it.
// Quoting is the shell's own: every byte inside a single-quoted span is literal, and inside a
// double-quoted span everything but a command substitution (`$(`) is literal — so a quoted pipe or
// brace is text, not a second process. A backslash escapes the next byte outside single quotes. The
// walk and the table beside it are the whole rule.
func activeMarkers(command string) map[string]bool {
	active := map[string]bool{}
	const (
		unquoted = 0
		single   = '\''
		double   = '"'
	)
	quote := byte(unquoted)
	for i := 0; i < len(command); i++ {
		c := command[i]
		switch quote {
		case single:
			if c == '\'' {
				quote = unquoted
			}
			continue
		case double:
			switch {
			case c == '\\':
				i++
			case c == '"':
				quote = unquoted
			case c == '$' && i+1 < len(command) && command[i+1] == '(':
				active["$("] = true
			case c == '$' && i+1 < len(command) && command[i+1] == '{':
				i = skipExpansion(command, i)
			}
			continue
		}
		switch {
		case c == '\\':
			i++
		case c == '\'':
			quote = single
		case c == '"':
			quote = double
		case c == '$' && i+1 < len(command) && command[i+1] == '(':
			active["$("] = true
		case c == '$' && i+1 < len(command) && command[i+1] == '{':
			i = skipExpansion(command, i)
		case strings.IndexByte("\n;&|(){}<>", c) >= 0:
			active[string(c)] = true
		}
	}
	return active
}

// skipExpansion returns the index of the `}` that closes the `${` starting at i, or i when the
// expansion is unterminated. The braces of a parameter expansion `${VAR}` are not shell grouping, so
// the parameter is skipped here and the placeholder rule owns it, rather than this rule reporting the
// braces as a brace group.
func skipExpansion(command string, i int) int {
	if end := strings.IndexByte(command[i+2:], '}'); end >= 0 {
		return i + 2 + end
	}
	return i
}

// markerDetail is the human sentence a metacharacter refusal carries: it names the row, the command,
// and the marker, then says what that marker does that one pinned observation cannot cover.
func markerDetail(id, command, marker, reason string) string {
	switch reason {
	case ReasonHasSubstitution:
		return fmt.Sprintf("evidence %s admits %q, which carries the command substitution %q: a second process runs inside the cell, so the output is not this command's own observation", id, command, marker)
	case ReasonHasRedirection:
		return fmt.Sprintf("evidence %s admits %q, which redirects I/O with %q: the file the shell writes or reads is a side effect the pinned digest does not cover, so the row is refused before the shell sees it", id, command, marker)
	default:
		return fmt.Sprintf("evidence %s admits %q, which carries the shell metacharacter %q: the row owes one command whose output is one stream, so chained, piped, grouped, sequenced, or backgrounded commands are refused", id, command, marker)
	}
}

// commandPlaceholder returns the first unexpanded placeholder in a command, or "" when there is none.
func commandPlaceholder(command string) string {
	if found := anglePlaceholder.FindString(command); found != "" {
		return found
	}
	return varPlaceholder.FindString(command)
}

// Digest returns "sha256:<hex>" over the observed output after two passes, so a run is never refused for a
// formatting accident, a digest pinned on one platform stays comparable on another, and a row whose output
// moves in a declared way can still be pinned:
//
//  1. the structural pass, which forgives formatting and nothing else
//     - split on "\n"
//     - strip a trailing "\r" and trailing spaces and tabs from every line
//     - drop trailing empty lines
//     - join with "\n"
//  2. the row's own expression, when its Normalize cell declares one: every match becomes a single "X"
//
// The digest covers exactly what those two passes leave. The structural pass cannot hide a changed
// observation, because only trailing whitespace and trailing blank lines are forgiven. The row's own
// expression can hide one, and that is the row author's promise to keep: a pattern broad enough to swallow
// the whole output turns the pin into decoration, and no reader of the plan can tell by looking. The
// stability probe is what keeps the promise honest, because a pin that would only hide a moving output is
// refused instead of written.
func Digest(output, normalize string) (string, error) {
	text, err := canonical(output, normalize)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(text))
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// compileNormalize reports whether a row's expression can be used at all. It exists so the row can be
// refused before its command is spawned rather than after a run has been spent on a row defect.
func compileNormalize(normalize string) error {
	if normalize == "" {
		return nil
	}
	_, err := regexp.Compile(normalize)
	return err
}

// canonical is the text Digest hashes and lineCount counts, so a result's line count and its digest always
// describe the same observation. The structural pass runs first and the row's expression runs over its
// result, so a pattern sees the text the digest is actually taken over.
func canonical(output, normalize string) (string, error) {
	text := normalizeOutput(output)
	if normalize == "" {
		return text, nil
	}
	expr, err := regexp.Compile(normalize)
	if err != nil {
		return "", err
	}
	return expr.ReplaceAllString(text, "X"), nil
}

// normalizeOutput is the structural pass: the byte string the row's expression then rewrites.
func normalizeOutput(output string) string {
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, "\r \t")
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// lineCount counts the lines of the text the digest covers, so the two always describe one observation.
func lineCount(output, normalize string) int {
	text, err := canonical(output, normalize)
	if err != nil || text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

// idSet turns an id list into a lookup, keeping the empty list empty so it never filters.
func idSet(ids []string) map[string]bool {
	if len(ids) == 0 {
		return nil
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}
