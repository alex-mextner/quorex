package executor

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// GenericExecutor runs any command and captures its combined stdout+stderr.
// Used for "plain" protocol executors (opencode, gemini --yolo, etc).
type GenericExecutor struct {
	command string
	args    []string
}

// NewGenericExecutor creates a new GenericExecutor.
func NewGenericExecutor(command string, args []string) *GenericExecutor {
	return &GenericExecutor{command: command, args: args}
}

// Run executes the command, passing the prompt via stdin.
func (e *GenericExecutor) Run(ctx context.Context, prompt string) Result {
	cmd := exec.CommandContext(ctx, e.command, e.args...) //nolint:gosec // user-configured
	if prompt != "" {
		cmd.Stdin = strings.NewReader(prompt)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Result{Output: string(out), Error: fmt.Errorf("%s exited: %w", e.command, err)}
	}
	return Result{Output: string(out)}
}
