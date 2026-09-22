package tui

import (
	"fmt"
	"strings"
)

const (
	helpRoot     = "up/down move | enter select | q quit"
	helpStatus   = "esc back | q quit"
	helpFeatures = "space toggle | enter preview | esc back | q quit"
	helpPlan     = "esc back | q quit"
)

var menuItems = []string{"Status", "Features", "Sync plan", "Quit"}

// joinSections builds one frame from blank-line-separated sections; empty sections are dropped
// so a view with no detail block or no rows does not grow a double blank line.
func joinSections(sections ...string) string {
	kept := make([]string, 0, len(sections))
	for _, section := range sections {
		if section != "" {
			kept = append(kept, section)
		}
	}
	return strings.Join(kept, "\n\n")
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func stateLabel(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func renderMenu(sel int) string {
	var items strings.Builder
	for i, name := range menuItems {
		if i > 0 {
			items.WriteByte('\n')
		}
		if i == sel {
			items.WriteString("> ")
		} else {
			items.WriteString("  ")
		}
		items.WriteString(name)
	}
	return joinSections("rdd-plus tui", items.String(), helpRoot)
}

func renderStatus(sv StatusView, err error) string {
	const title = "rdd-plus tui: status"
	if err != nil {
		return joinSections(title, "error: "+err.Error(), helpStatus)
	}
	var body strings.Builder
	fmt.Fprintf(&body, "State root: %s\n", sv.StateRoot)
	fmt.Fprintf(&body, "State exists: %s\n", yesNo(sv.StateExists))
	fmt.Fprintf(&body, "Installed version: %s\n", sv.InstalledVersion)
	fmt.Fprintf(&body, "Available version: %s", sv.AvailableVersion)
	body.WriteString("\n\nID\tTitle\tState")
	for _, row := range sv.Features {
		fmt.Fprintf(&body, "\n%s\t%s\t%s", row.ID, row.Title, stateLabel(row.Enabled))
	}
	return joinSections(title, body.String(), helpStatus)
}

func renderFeatures(rows []FeatureRow, sel int, detail string) string {
	const title = "rdd-plus tui: features"
	var list strings.Builder
	for i, row := range rows {
		if i > 0 {
			list.WriteByte('\n')
		}
		if i == sel {
			list.WriteString("> ")
		} else {
			list.WriteString("  ")
		}
		fmt.Fprintf(&list, "%s\t%s\t%s", row.ID, row.Title, stateLabel(row.Enabled))
	}
	return joinSections(title, list.String(), strings.TrimRight(detail, "\n"), helpFeatures)
}

func renderPlan(report string, err error) string {
	const title = "rdd-plus tui: sync plan (dry-run; writes nothing)"
	body := report
	if err != nil {
		body = "error: " + err.Error()
	}
	return joinSections(title, strings.TrimRight(body, "\n"), helpPlan)
}
