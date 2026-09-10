package gate

import "strings"

// BuildAuditReason is what the Stop says when the discipline ran. Covering a diff and reporting
// as though the surface were covered is the failure this catches: the run looks complete because
// every step it did take succeeded.
func BuildAuditReason(planPath, gaps string) string {
	return strings.Join([]string{
		"This session ran the testing discipline and left breadth owed. " + planPath + " says:",
		"",
		strings.TrimRight(gaps, "\n"),
		"",
		"A layer assigned and never invoked is an open gap, not a silence. Before reporting this",
		"done, either invoke the owners above, or say plainly in one line which surfaces went",
		"unexamined, so nobody reads depth on a diff as coverage of the whole.",
		"",
		feedbackOffer,
	}, "\n")
}

// BuildCompleteReason is what the Stop says when the plan owes nothing. A run with no gaps is
// still worth grading: the question is what it would have missed, not whether it finished.
func BuildCompleteReason(planPath string) string {
	return strings.Join([]string{
		"This session ran the testing discipline and " + planPath + " owes nothing: every layer",
		"the plan assigned was swept and every ranked target is done.",
		"",
		feedbackOffer,
	}, "\n")
}

const feedbackOffer = "Want the run graded? Offer the operator feedback on the testing itself, in " +
	"one short pass: what was executed, what was skipped, which findings a different order would " +
	"have surfaced first, and what you would still not know if the suite were green. This is a " +
	"reminder, not an approval gate. Silence it for this repository with a .no-testing-gate file."
