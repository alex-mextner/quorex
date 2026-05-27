// Package initcmd implements the "quorex init" command.
package initcmd

import "os/exec"

// priorityHarnesses defines detection order per spec.
var priorityHarnesses = []string{"claude", "codex", "opencode"}

// DetectHarness returns the first available AI harness found in PATH.
// Returns empty string if none found.
func DetectHarness() string {
	for _, h := range priorityHarnesses {
		if _, err := exec.LookPath(h); err == nil {
			return h
		}
	}
	return ""
}

// InstallInstructions returns install guidance when no harness is detected.
func InstallInstructions() string {
	return `No supported AI harness found. Install one of:
  claude   — https://claude.ai/code
  codex    — https://github.com/openai/codex
  opencode — https://opencode.ai

Then re-run: quorex init`
}
