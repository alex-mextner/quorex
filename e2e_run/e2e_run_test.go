//go:build e2e

// Package e2e_run tests the quorex run subcommand through the actual binary.
// Each test builds a real git repo, writes fake AI agent scripts, and invokes
// the binary as a subprocess. No mocks — real git operations, real process execution.
package e2e_run

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var quorexBin string

func TestMain(m *testing.M) {
	bin, err := buildBinary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build quorex binary: %v\n", err)
		os.Exit(1)
	}
	quorexBin = bin
	code := m.Run()
	os.Remove(bin) //nolint:errcheck // best-effort cleanup of temp binary
	os.Exit(code)
}

func buildBinary() (string, error) {
	// e2e_run/ is a direct child of the project root.
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	projectRoot := filepath.Dir(cwd)

	f, err := os.CreateTemp("", "quorex-e2e-*")
	if err != nil {
		return "", fmt.Errorf("temp file: %w", err)
	}
	f.Close()
	os.Remove(f.Name()) //nolint:errcheck // path is reused without the empty file

	binPath := f.Name()
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/quorex")
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go build: %w", err)
	}
	return binPath, nil
}

// runIn runs the quorex binary with the given args and cwd.
// Returns combined stdout+stderr output and exit code.
func runIn(dir string, args ...string) (string, int) {
	cmd := exec.Command(quorexBin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	}
	return string(out), code
}

// initRepo creates a temp git repo with an initial commit and returns its path.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	git("init", "-b", "main")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0o600))
	git("add", "README.md")
	git("commit", "-m", "init")
	return dir
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}

// writeExec writes an executable shell script and returns its path.
func writeExec(t *testing.T, dir, name, content string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o755)) //nolint:gosec // test scripts must be executable
	return p
}

// validPlan is a minimal plan that passes planval.Validate.
const validPlan = "# Plan\n\n## Task: write-file\n\n- [ ] Write output file\n"

// TestRunCmd_NoArgs verifies that "quorex run" with no plan file exits non-zero
// and prints usage to stderr.
func TestRunCmd_NoArgs(t *testing.T) {
	dir := initRepo(t)
	out, code := runIn(dir, "run")
	assert.NotEqual(t, 0, code, "exit code must be non-zero")
	assert.Contains(t, out, "usage")
}

// TestRunCmd_MissingPlan verifies that a non-existent plan file is reported.
func TestRunCmd_MissingPlan(t *testing.T) {
	dir := initRepo(t)
	out, code := runIn(dir, "run", filepath.Join(dir, "nonexistent.md"))
	assert.NotEqual(t, 0, code)
	assert.Contains(t, out, "nonexistent.md")
}

// TestRunCmd_InvalidPlan verifies that a plan with a bad ## heading (not "## Task: <name>")
// is rejected with a validation error that names the file.
func TestRunCmd_InvalidPlan(t *testing.T) {
	dir := initRepo(t)
	planPath := filepath.Join(dir, "bad-plan.md")
	writeFile(t, dir, "bad-plan.md", "# Feature\n\n## Overview\n\nSome text\n")

	out, code := runIn(dir, "run", planPath)
	assert.NotEqual(t, 0, code)
	assert.Contains(t, out, "bad-plan.md")
}

// TestRunCmd_InvalidToml verifies that a corrupt quorex.toml in the working directory
// causes an early error (file absent is OK, file present but invalid is not).
func TestRunCmd_InvalidToml(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "quorex.toml", "[executor\n") // unclosed TOML table header
	writeFile(t, dir, "plan.md", validPlan)

	out, code := runIn(dir, "run", filepath.Join(dir, "plan.md"))
	assert.NotEqual(t, 0, code)
	assert.Contains(t, out, "quorex.toml")
}

// TestRunCmd_SingleAgent_AppliesDiff runs a single fake AI agent that writes a
// uniquely-named file, then verifies quorex applies the diff to the main checkout.
func TestRunCmd_SingleAgent_AppliesDiff(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}
	dir := initRepo(t)
	scriptsDir := t.TempDir()

	agentScript := writeExec(t, scriptsDir, "agent.sh",
		"#!/bin/sh\nprintf 'AGENT_MARKER_XYZ' > agent_output.txt\n")

	writeFile(t, dir, "quorex.toml", fmt.Sprintf(
		"[executor.myagent]\ncommand = %q\nrole    = \"task\"\n\n[pool]\ntask_executors = \"myagent\"\ntimeout        = \"30s\"\n",
		agentScript,
	))
	writeFile(t, dir, "plan.md", validPlan)

	out, code := runIn(dir, "run", filepath.Join(dir, "plan.md"))
	require.Equal(t, 0, code, "quorex run must exit 0\noutput:\n%s", out)

	// The agent's output file must appear in the main repo's working tree after apply.
	content, err := os.ReadFile(filepath.Join(dir, "agent_output.txt")) //nolint:gosec // test reads from known temp dir
	require.NoError(t, err, "agent_output.txt must exist in main repo after apply")
	assert.Contains(t, string(content), "AGENT_MARKER_XYZ")
}

// TestRunCmd_AgentNoChanges verifies that when an agent produces no diff
// (exits 0 but doesn't touch any files), quorex run still exits 0.
func TestRunCmd_AgentNoChanges(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}
	dir := initRepo(t)
	scriptsDir := t.TempDir()

	noopScript := writeExec(t, scriptsDir, "noop.sh", "#!/bin/sh\nexit 0\n")

	writeFile(t, dir, "quorex.toml", fmt.Sprintf(
		"[executor.noop]\ncommand = %q\nrole    = \"task\"\n\n[pool]\ntask_executors = \"noop\"\ntimeout        = \"30s\"\n",
		noopScript,
	))
	writeFile(t, dir, "plan.md", validPlan)

	_, code := runIn(dir, "run", filepath.Join(dir, "plan.md"))
	assert.Equal(t, 0, code, "no-op agent must still exit 0")
}
