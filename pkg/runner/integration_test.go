package runner_test

// Integration tests with simulated AI agents.
// Each "agent" is a shell script written to a temp directory that makes real file
// changes inside the git worktree.  The review agent outputs a scoring matrix in the
// exact format ParseMatrix expects.  No mocks, no stubs — real git operations, real
// process execution.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/alex-mextner/quorex/pkg/quorexcfg"
	"github.com/alex-mextner/quorex/pkg/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeReviewScript outputs a fixed scoring matrix.
// It reads stdin (the eval prompt) and ignores it — we just want predictable output.
const fakeReviewScript = `#!/bin/sh
cat <<'MATRIX'
Architecture: alpha
Correctness: beta
Code style: alpha
Tests: beta
MATRIX
`

// failingPostHook is a post-hook script that always fails.
const failingPostHook = `#!/bin/sh
exit 1
`

// writeScript writes content as an executable shell script and returns its path.
func writeScript(t *testing.T, dir, name, content string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o755)) //nolint:gosec // test scripts must be executable
	return p
}

// initGitRepoFull creates a git repo with an initial commit and a placeholder file.
func initGitRepoFull(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	// initial file so the repo has content
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o600))
	run("add", ".")
	run("commit", "-m", "init")
}

// TestRunner_TwoAgents_MatrixPopulated runs two fake AI agents, then verifies that
// the judge eval captures both diffs and produces a Matrix with expected winners.
func TestRunner_TwoAgents_MatrixPopulated(t *testing.T) {
	repoDir := t.TempDir()
	scriptsDir := t.TempDir()
	initGitRepoFull(t, repoDir)

	alphaScript := writeScript(t, scriptsDir, "alpha.sh",
		"#!/bin/sh\nprintf 'package main\\n\\nfunc Alpha() string { return \"alpha\" }\\n' > agent_alpha.go\n")
	betaScript := writeScript(t, scriptsDir, "beta.sh",
		"#!/bin/sh\nprintf 'package main\\n\\nfunc Beta() string { return \"beta\" }\\n' > agent_beta.go\n")
	reviewScript := writeScript(t, scriptsDir, "review.sh", fakeReviewScript)

	cfg := configWithAgents(t, alphaScript, betaScript, reviewScript)
	r := runner.New(cfg, repoDir)

	result, err := r.Run(context.Background(), "implement a feature")
	require.NoError(t, err)
	defer r.Cleanup(result.PoolResults)

	// Both diffs must be captured.
	assert.Len(t, result.Diffs, 2)
	_, hasAlpha := result.Diffs["alpha"]
	_, hasBeta := result.Diffs["beta"]
	assert.True(t, hasAlpha, "alpha diff must be captured")
	assert.True(t, hasBeta, "beta diff must be captured")

	// Matrix must have categories and winners.
	assert.NotEmpty(t, result.Matrix.Categories)
	assert.NotEmpty(t, result.Matrix.Scores)

	// Verify specific winners from fakeReviewScript.
	archWinner := firstWinner(result.Matrix.Scores["Architecture"])
	correctWinner := firstWinner(result.Matrix.Scores["Correctness"])
	assert.Equal(t, "alpha", archWinner, "Architecture winner must be alpha")
	assert.Equal(t, "beta", correctWinner, "Correctness winner must be beta")
}

// TestRunner_TwoAgents_SynthesisPromptBuilt verifies the synthesis prompt
// contains the diff content and matrix for both agents.
func TestRunner_TwoAgents_SynthesisPromptBuilt(t *testing.T) {
	repoDir := t.TempDir()
	scriptsDir := t.TempDir()
	initGitRepoFull(t, repoDir)

	alphaScript := writeScript(t, scriptsDir, "alpha.sh",
		"#!/bin/sh\necho 'alpha was here' > alpha_output.txt\n")
	betaScript := writeScript(t, scriptsDir, "beta.sh",
		"#!/bin/sh\necho 'beta was here' > beta_output.txt\n")
	reviewScript := writeScript(t, scriptsDir, "review.sh", fakeReviewScript)

	cfg := configWithAgents(t, alphaScript, betaScript, reviewScript)
	r := runner.New(cfg, repoDir)

	result, err := r.Run(context.Background(), "the original task description")
	require.NoError(t, err)
	defer r.Cleanup(result.PoolResults)

	require.NotEmpty(t, result.SynthesisPrompt, "synthesis prompt must be built for multi-provider run")
	assert.Contains(t, result.SynthesisPrompt, "original task description", "synthesis prompt must contain original task")
	assert.Contains(t, result.SynthesisPrompt, "Architecture", "synthesis prompt must include matrix categories")
	assert.Contains(t, result.SynthesisPrompt, "alpha", "synthesis prompt must reference provider names")
	assert.Contains(t, result.SynthesisPrompt, "beta", "synthesis prompt must reference provider names")
}

// TestRunner_PostHookFails_ExcludesOneAgent runs two agents but the post-hook for
// "beta" fails (e.g., tests failed in that worktree). Only "alpha" proceeds to judging.
// With a single passing provider the degenerate path is taken (no synthesis prompt).
func TestRunner_PostHookFails_ExcludesOneAgent(t *testing.T) {
	repoDir := t.TempDir()
	scriptsDir := t.TempDir()
	initGitRepoFull(t, repoDir)

	alphaScript := writeScript(t, scriptsDir, "alpha.sh",
		"#!/bin/sh\necho 'alpha' > alpha.txt\n")
	betaScript := writeScript(t, scriptsDir, "beta.sh",
		"#!/bin/sh\necho 'beta' > beta.txt\n")
	// Post-hook that fails only for the "beta" worktree.
	// We simulate this with a hook that checks if "beta.txt" exists and exits 1.
	postHookScript := writeScript(t, scriptsDir, "post.sh",
		"#!/bin/sh\n[ -f beta.txt ] && exit 1 || exit 0\n")

	cfg := configWithAgentsAndPostHook(t, alphaScript, betaScript, "", postHookScript)
	r := runner.New(cfg, repoDir)

	result, err := r.Run(context.Background(), "task")
	require.NoError(t, err)
	defer r.Cleanup(result.PoolResults)

	// Only alpha passed post-hook → degenerate single-provider path.
	assert.Empty(t, result.SynthesisPrompt, "degenerate path: no synthesis needed")
	assert.Len(t, result.Diffs, 1)
	_, hasAlpha := result.Diffs["alpha"]
	assert.True(t, hasAlpha, "alpha must be the sole passing provider")
}

// TestRunner_BothPostHooksFail_ReturnsError verifies that when all providers
// are excluded by post-hooks, Run returns an error instead of an empty result.
func TestRunner_BothPostHooksFail_ReturnsError(t *testing.T) {
	repoDir := t.TempDir()
	scriptsDir := t.TempDir()
	initGitRepoFull(t, repoDir)

	noop := writeScript(t, scriptsDir, "noop.sh", "#!/bin/sh\nexit 0\n")
	failHook := writeScript(t, scriptsDir, "fail.sh", failingPostHook)

	cfg := configWithAgentsAndPostHook(t, noop, noop, "", failHook)
	r := runner.New(cfg, repoDir)

	_, err := r.Run(context.Background(), "task")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "excluded by post-hooks")
}

// TestRunner_DiffsContainActualChanges verifies that the captured diffs include
// the file content written by each fake agent, not empty strings.
func TestRunner_DiffsContainActualChanges(t *testing.T) {
	repoDir := t.TempDir()
	scriptsDir := t.TempDir()
	initGitRepoFull(t, repoDir)

	alphaScript := writeScript(t, scriptsDir, "alpha.sh",
		"#!/bin/sh\nprintf 'ALPHA_MARKER_XYZ' > alpha_unique.txt\n")
	betaScript := writeScript(t, scriptsDir, "beta.sh",
		"#!/bin/sh\nprintf 'BETA_MARKER_XYZ' > beta_unique.txt\n")
	reviewScript := writeScript(t, scriptsDir, "review.sh", fakeReviewScript)

	cfg := configWithAgents(t, alphaScript, betaScript, reviewScript)
	r := runner.New(cfg, repoDir)

	result, err := r.Run(context.Background(), "task")
	require.NoError(t, err)
	defer r.Cleanup(result.PoolResults)

	assert.Contains(t, result.Diffs["alpha"], "ALPHA_MARKER_XYZ",
		"alpha diff must contain the file content written by the alpha agent")
	assert.Contains(t, result.Diffs["beta"], "BETA_MARKER_XYZ",
		"beta diff must contain the file content written by the beta agent")
}

// TestRunner_Cleanup_AfterMultiProvider verifies that cleanup removes all worktrees
// created during a multi-provider run.
func TestRunner_Cleanup_AfterMultiProvider(t *testing.T) {
	repoDir := t.TempDir()
	scriptsDir := t.TempDir()
	initGitRepoFull(t, repoDir)

	alphaScript := writeScript(t, scriptsDir, "alpha.sh", "#!/bin/sh\nexec true\n")
	betaScript := writeScript(t, scriptsDir, "beta.sh", "#!/bin/sh\nexec true\n")
	reviewScript := writeScript(t, scriptsDir, "review.sh", fakeReviewScript)

	cfg := configWithAgents(t, alphaScript, betaScript, reviewScript)
	r := runner.New(cfg, repoDir)

	result, err := r.Run(context.Background(), "task")
	require.NoError(t, err)

	var wtPaths []string
	for _, res := range result.PoolResults {
		if res.Worktree != nil {
			wtPaths = append(wtPaths, res.Worktree.Path)
		}
	}
	require.Len(t, wtPaths, 2, "two worktrees should have been created")

	r.Cleanup(result.PoolResults)

	for _, p := range wtPaths {
		_, statErr := os.Stat(p)
		assert.True(t, os.IsNotExist(statErr), "worktree %s must be removed after cleanup", p)
	}
}

// --- helpers ---

func configWithAgents(t *testing.T, alphaCmd, betaCmd, reviewCmd string) *quorexcfg.Config {
	t.Helper()
	toml := `
[executor.alpha]
command = "` + alphaCmd + `"
role    = "task"

[executor.beta]
command = "` + betaCmd + `"
role    = "task"

[executor.review]
command  = "` + reviewCmd + `"
role     = "review"

[pool]
task_executors   = "alpha,beta"
task_parallel    = 2
review_executors = "review"
timeout          = "30s"
`
	cfg, err := quorexcfg.Parse([]byte(toml))
	require.NoError(t, err)
	return cfg
}

func configWithAgentsAndPostHook(t *testing.T, alphaCmd, betaCmd, reviewCmd, postHookCmd string) *quorexcfg.Config {
	t.Helper()
	reviewSection := ""
	reviewExecutors := ""
	if reviewCmd != "" {
		reviewSection = `
[executor.review]
command  = "` + reviewCmd + `"
role     = "review"
`
		reviewExecutors = "review"
	}

	toml := `
[executor.alpha]
command = "` + alphaCmd + `"
role    = "task"

[executor.beta]
command = "` + betaCmd + `"
role    = "task"
` + reviewSection + `
[pool]
task_executors   = "alpha,beta"
task_parallel    = 2
review_executors = "` + reviewExecutors + `"
timeout          = "30s"

[[hook]]
name    = "post-check"
command = "` + postHookCmd + `"
phase   = "post"
`
	cfg, err := quorexcfg.Parse([]byte(toml))
	require.NoError(t, err)
	return cfg
}

// firstWinner returns the name of the first true winner from a score map.
func firstWinner(scores map[string]bool) string {
	for k, v := range scores {
		if v {
			return k
		}
	}
	return ""
}
