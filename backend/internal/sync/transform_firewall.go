package sync

import (
	"context"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func init() {
	RegisterTransformer("networking", TransformerFunc(normalizeFirewallRules))
}

// normalizeFirewallRules rewrites the "Enabled" column of a "Firewall Rules"
// section's markdown table from raw "1"/"0" to "Yes"/"No", so pfSense and
// OPNsense (and any future firewall connector) share one normalization
// instead of each connector formatting it themselves.
func normalizeFirewallRules(_ context.Context, snap *connector.ServiceSnapshot) error {
	for i, sec := range snap.Sections {
		if sec.Title != "Firewall Rules" {
			continue
		}
		snap.Sections[i].Content = normalizeEnabledColumn(sec.Content)
	}
	return nil
}

// normalizeEnabledColumn rewrites the last "| 1 |" / "| 0 |" cell on each
// markdown table row to "| Yes |" / "| No |", leaving header/separator rows
// and every other column untouched.
func normalizeEnabledColumn(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") || strings.Contains(line, "---") {
			continue
		}
		cells := strings.Split(line, "|")
		last := len(cells) - 1
		for last >= 0 && strings.TrimSpace(cells[last]) == "" {
			last--
		}
		if last < 0 {
			continue
		}
		switch strings.TrimSpace(cells[last]) {
		case "1":
			cells[last] = " Yes "
		case "0":
			cells[last] = " No "
		default:
			continue
		}
		lines[i] = strings.Join(cells, "|")
	}
	return strings.Join(lines, "\n")
}
