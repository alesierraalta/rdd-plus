package gate

import "strings"

// BuildAuditReason is what the Stop says when the discipline ran and stopped halfway. Covering a
// diff and reporting as though the surface were covered is the failure this catches: the run
// looks complete because every step it did take succeeded.
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
		"Want the run graded? Ask for feedback on the testing itself: what was executed, what was",
		"skipped, and which findings would have been missed. This is a reminder, not an approval",
		"gate. Silence it for this repository with a .no-testing-gate file.",
	}, "\n")
}
