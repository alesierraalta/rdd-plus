package plan

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Gaps is what a finished run still owes: the breadth half of the discipline, which is the half
// a session can skip while its report still reads as complete.
type Gaps struct {
	NoLayerMatrix  bool     `json:"no_layer_matrix"`          // breadth was never planned, not merely left undone
	UnsweptLayers  []string `json:"unswept_layers,omitempty"` // "Security (appsec-adversarial-auditor)"
	LayersDone     int      `json:"layers_done"`
	LayersTotal    int      `json:"layers_total"` // excluding layers the plan marked n/a
	TargetsDone    int      `json:"targets_done"`
	TargetsTotal   int      `json:"targets_total"`
	PendingTargets []string `json:"pending_targets,omitempty"`
}

// Any reports whether the run left breadth owed.
func (g Gaps) Any() bool {
	return g.NoLayerMatrix || len(g.UnsweptLayers) > 0 || g.TargetsDone < g.TargetsTotal
}

// Report renders the gaps as the lines a final message has to carry to be honest.
func (g Gaps) Report() string {
	var b strings.Builder
	if g.NoLayerMatrix {
		b.WriteString("the layer sweep was never planned: the plan has no layer matrix, so breadth was not skipped, it was never on the list\n")
	} else {
		fmt.Fprintf(&b, "layers swept: %d of %d\n", g.LayersDone, g.LayersTotal)
		for _, l := range g.UnsweptLayers {
			fmt.Fprintf(&b, "  assigned and never invoked: %s\n", l)
		}
	}
	fmt.Fprintf(&b, "ranked targets done: %d of %d\n", g.TargetsDone, g.TargetsTotal)
	for _, t := range g.PendingTargets {
		fmt.Fprintf(&b, "  still pending: %s\n", t)
	}
	return b.String()
}

var doneStatus = regexp.MustCompile(`(?i)^(done|fixed|closed)$`)
var naStatus = regexp.MustCompile(`(?i)^(n/?a|none|skipped)$`)

// GapsInFile reads a plan and reports what it still owes.
func GapsInFile(path string) (Gaps, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Gaps{}, err
	}
	return GapsIn(string(raw))
}

// GapsIn reports what a plan document still owes.
func GapsIn(doc string) (Gaps, error) {
	var g Gaps
	layers := section(doc, "Layer matrix")
	if strings.TrimSpace(layers) == "" {
		g.NoLayerMatrix = true
	} else {
		rows, header := table(layers)
		iSkill := columnIndex(header, "skill")
		iStatus := columnIndex(header, "status")
		for _, row := range rows {
			status := cell(row, iStatus)
			if naStatus.MatchString(status) {
				continue
			}
			g.LayersTotal++
			if doneStatus.MatchString(status) {
				g.LayersDone++
				continue
			}
			name := cell(row, 0)
			if owner := strings.Trim(cell(row, iSkill), "`"); owner != "" {
				name += " (" + owner + ")"
			}
			g.UnsweptLayers = append(g.UnsweptLayers, name)
		}
	}
	rows, header := table(section(doc, "Ranked targets"))
	iStatus := columnIndex(header, "status")
	for _, row := range rows {
		status := cell(row, iStatus)
		if naStatus.MatchString(status) {
			continue
		}
		g.TargetsTotal++
		if doneStatus.MatchString(status) {
			g.TargetsDone++
			continue
		}
		g.PendingTargets = append(g.PendingTargets, cell(row, 0))
	}
	return g, nil
}
