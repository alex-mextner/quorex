package judge

import (
	"fmt"
	"strings"
)

// BuildEvalPrompt constructs the Pass 1 prompt.
// The judge executor must return one "category: provider" line per category.
func BuildEvalPrompt(diffs map[string]string, categories []string) string {
	var b strings.Builder
	b.WriteString("You are evaluating code changes from multiple AI providers.\n\n")
	b.WriteString("## Provider Diffs\n\n")
	for provider, diff := range diffs {
		fmt.Fprintf(&b, "### %s\n\n```diff\n%s\n```\n\n", provider, diff)
	}
	b.WriteString("## Task\n\n")
	b.WriteString("Produce a scoring matrix. For each category, identify which provider ")
	b.WriteString("had the strongest approach. Mark exactly one winner per category or none.\n\n")
	b.WriteString("Categories: " + strings.Join(categories, ", ") + "\n\n")
	b.WriteString("Output format (one line per category):\n")
	b.WriteString("```\n<category>: <provider_name>\n```\n")
	b.WriteString("Use exact provider names. Output ONLY the matrix lines.\n")
	return b.String()
}

// BuildSynthesisPrompt constructs the Pass 2 prompt.
// The judge executor receives the matrix as context for a ralphex iteration.
func BuildSynthesisPrompt(taskDescription string, matrix Matrix, diffs map[string]string) string {
	var b strings.Builder
	b.WriteString("You are synthesizing the best solution from multiple AI provider outputs.\n\n")
	b.WriteString("## Original Task\n\n")
	b.WriteString(taskDescription + "\n\n")
	b.WriteString("## Scoring Matrix\n\n```\n")
	b.WriteString(matrix.Render())
	b.WriteString("```\n\n")
	b.WriteString("## Provider Outputs\n\n")
	for provider, diff := range diffs {
		fmt.Fprintf(&b, "### %s\n\n```diff\n%s\n```\n\n", provider, diff)
	}
	b.WriteString("## Instructions\n\n")
	b.WriteString("Using the matrix as a guide for which provider excelled in each area, ")
	b.WriteString("produce a unified implementation combining the strongest elements. ")
	b.WriteString("Apply changes to the working tree.\n")
	return b.String()
}

// ParseMatrix parses the judge's Pass 1 output into a Matrix.
func ParseMatrix(output string, providers []string) Matrix {
	scores := make(map[string]map[string]bool)
	var categories []string

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "```") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		cat := strings.TrimSpace(parts[0])
		winner := strings.TrimSpace(parts[1])
		if cat == "" {
			continue
		}
		categories = append(categories, cat)
		if winner == "" || winner == "none" {
			scores[cat] = map[string]bool{}
		} else {
			scores[cat] = map[string]bool{winner: true}
		}
	}

	return Matrix{
		Providers:  providers,
		Categories: categories,
		Scores:     scores,
	}
}
