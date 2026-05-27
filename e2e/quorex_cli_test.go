package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// binaryPath resolves the path to the quorex binary.
// Falls back to locally built binary; skips if not found.
func binaryPath(t *testing.T) string {
	t.Helper()
	// prefer binary in project root
	root := projectRoot(t)
	local := filepath.Join(root, "quorex")
	if _, err := os.Stat(local); err == nil {
		return local
	}
	if bin, err := exec.LookPath("quorex"); err == nil {
		return bin
	}
	t.Skip("quorex binary not found; run: go build ./cmd/quorex/")
	return ""
}

func projectRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (go.mod)")
		}
		dir = parent
	}
}

// runQuorex runs the quorex binary with the given args and returns stdout, stderr, exit code.
func runQuorex(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	bin := binaryPath(t)
	cmd := exec.Command(bin, args...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("exec quorex: %v", err)
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// runQuorexEnv runs quorex with a custom environment (e.g. modified PATH).
func runQuorexEnv(t *testing.T, env []string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	bin := binaryPath(t)
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("exec quorex: %v", err)
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// writePlan writes plan content to a temp file and returns the path.
func writePlan(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// fakeBinDir creates a temp dir with a fake binary named `name` that exits 0 with `output`.
func fakeBinDir(t *testing.T, name, output string) string {
	t.Helper()
	dir := t.TempDir()
	script := fmt.Sprintf("#!/bin/sh\necho %q\n", output)
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755)) //nolint:gosec // test helper
	return dir
}

// TestCLI_Version verifies the binary prints "quorex" in version output.
func TestCLI_Version(t *testing.T) {
	stdout, _, code := runQuorex(t, "--version")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "quorex")
}

// TestCLI_PlansFixValidPlan verifies valid plans pass with exit 0.
func TestCLI_PlansFixValidPlan(t *testing.T) {
	path := writePlan(t, "# My Feature\n\n## Task: implement foo\n\ncode here\n")
	stdout, _, code := runQuorex(t, "plans", "fix", path)
	assert.Equal(t, 0, code, "valid plan should exit 0")
	assert.Contains(t, stdout, "valid")
}

// TestCLI_PlansFixInvalidPlan_MissingTaskPrefix verifies bad ## headings are flagged.
func TestCLI_PlansFixInvalidPlan_MissingTaskPrefix(t *testing.T) {
	path := writePlan(t, "# My Feature\n\n## implement foo\n\ncode here\n")
	stdout, _, code := runQuorex(t, "plans", "fix", path)
	assert.NotEqual(t, 0, code, "invalid plan should exit non-zero")
	assert.Contains(t, stdout, "## Task:")
	assert.Contains(t, stdout, "Line 3")
	assert.Contains(t, stdout, "quorex plans fix")
}

// TestCLI_PlansFixInvalidPlan_UnclosedFence verifies unclosed fences are detected.
func TestCLI_PlansFixInvalidPlan_UnclosedFence(t *testing.T) {
	path := writePlan(t, "# My Feature\n\n## Task: foo\n\n```go\nfunc x() {}\n")
	stdout, _, code := runQuorex(t, "plans", "fix", path)
	assert.NotEqual(t, 0, code)
	assert.Contains(t, stdout, "fence block not closed")
}

// TestCLI_PlansFixInvalidPlan_MultipleErrors verifies multiple errors are all reported.
func TestCLI_PlansFixInvalidPlan_MultipleErrors(t *testing.T) {
	path := writePlan(t, "# My Feature\n\n## implement foo\n\n```go\nfunc x() {}\n")
	stdout, _, code := runQuorex(t, "plans", "fix", path)
	assert.NotEqual(t, 0, code)
	assert.Contains(t, stdout, "Line 3")
	assert.Contains(t, stdout, "fence block not closed")
}

// TestCLI_PlansFixMissingFile verifies error when file doesn't exist.
func TestCLI_PlansFixMissingFile(t *testing.T) {
	_, stderr, code := runQuorex(t, "plans", "fix", "/nonexistent/plan.md")
	assert.NotEqual(t, 0, code)
	assert.Contains(t, stderr, "read plan")
}

// TestCLI_PlansNoArgs verifies usage error when no args.
func TestCLI_PlansNoArgs(t *testing.T) {
	_, stderr, code := runQuorex(t, "plans")
	assert.NotEqual(t, 0, code)
	assert.Contains(t, stderr, "usage")
}

// TestCLI_PlansFixFenceInsideSection_NotFlagged verifies headings inside fences are ignored.
func TestCLI_PlansFixFenceInsideSection_NotFlagged(t *testing.T) {
	path := writePlan(t, "# Feature\n\n## Task: foo\n\n```md\n## task bar\n```\n")
	stdout, _, code := runQuorex(t, "plans", "fix", path)
	assert.Equal(t, 0, code, "headings inside fences must not be flagged")
	assert.Contains(t, stdout, "valid")
}

// TestCLI_PlansFixAuto_InvokesHarness verifies --auto invokes the detected harness.
func TestCLI_PlansFixAuto_InvokesHarness(t *testing.T) {
	path := writePlan(t, "# My Feature\n\n## implement foo\n\ncode here\n")
	fakeDir := fakeBinDir(t, "claude", "auto-fix done")

	// inject fake claude into PATH
	pathEnv := "PATH=" + fakeDir + ":" + os.Getenv("PATH")
	stdout, _, _ := runQuorexEnv(t, append(os.Environ(), pathEnv), "plans", "fix", "--auto", path)
	// should mention auto-fixing and include fake binary output
	assert.Contains(t, stdout, "auto-fix done")
}

// TestCLI_InitNoHarness verifies init prints install instructions when no harness found.
func TestCLI_InitNoHarness(t *testing.T) {
	bin := binaryPath(t)
	cmd := exec.Command(bin, "init")
	// override PATH so no harness is found
	cmd.Env = append(os.Environ(), "PATH=/nonexistent")
	var outBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf
	_ = cmd.Run()
	out := outBuf.String()
	assert.Contains(t, out, "claude")
	assert.Contains(t, out, "codex")
	assert.Contains(t, out, "opencode")
}

// TestCLI_InitDetectsHarness verifies init detects a harness in PATH.
func TestCLI_InitDetectsHarness(t *testing.T) {
	fakeDir := fakeBinDir(t, "claude", "")
	pathEnv := "PATH=" + fakeDir + ":" + os.Getenv("PATH")

	bin := binaryPath(t)
	cmd := exec.Command(bin, "init")
	cmd.Env = append(os.Environ(), pathEnv)
	var outBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf
	err := cmd.Run()
	assert.NoError(t, err)
	assert.Contains(t, outBuf.String(), "claude")
}
