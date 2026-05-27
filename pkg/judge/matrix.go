// Package judge implements the two-pass judge for quorex.
package judge

import (
	"fmt"
	"strings"
)

// Matrix holds the scoring results from Pass 1 evaluation.
type Matrix struct {
	Providers  []string            // executor names in order
	Categories []string            // evaluated categories
	Scores     map[string]map[string]bool // category → provider → winner
}

// Render returns a human-readable ASCII table of the matrix.
func (m Matrix) Render() string {
	var b strings.Builder

	fmt.Fprintf(&b, "%-20s", "")
	for _, p := range m.Providers {
		fmt.Fprintf(&b, "  %-10s", p)
	}
	b.WriteString("\n")

	for _, cat := range m.Categories {
		fmt.Fprintf(&b, "%-20s", cat)
		for _, p := range m.Providers {
			if m.Scores[cat][p] {
				fmt.Fprintf(&b, "  %-10s", "★")
			} else {
				fmt.Fprintf(&b, "  %-10s", "—")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// DefaultCategories is the default evaluation set used when none configured.
var DefaultCategories = []string{
	"Architecture",
	"Correctness",
	"Error handling",
	"Code style",
	"Tests",
	"Performance",
	"Security",
}
