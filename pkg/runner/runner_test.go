package runner_test

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/alex-mextner/quorex/pkg/quorexcfg"
	"github.com/alex-mextner/quorex/pkg/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initGitRepo initializes a throwaway git repo in dir with an empty initial commit.
func initGitRepo(t *testing.T, dir string) {
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
	run("commit", "--allow-empty", "-m", "init")
}

// configWithEcho returns a quorexcfg.Config whose single task executor is 'echo'.
// echo is a built-in that writes its args to stdout, so the pool will always succeed.
func configWithEcho(t *testing.T) *quorexcfg.Config {
	t.Helper()
	cfg, err := quorexcfg.Parse([]byte(`
[executor.echo]
command = "echo"
args    = ["hello"]
role    = "task"

[pool]
task_executors = "echo"
task_parallel  = 1
`))
	require.NoError(t, err)
	return cfg
}

func TestRunner_PreHookFails_Aborts(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	cfg, err := quorexcfg.Parse([]byte(`
[executor.echo]
command = "echo"
args    = ["hello"]
role    = "task"

[pool]
task_executors = "echo"

[[hook]]
name    = "failing-pre"
command = "false"
phase   = "pre"
`))
	require.NoError(t, err)

	r := runner.New(cfg, dir)
	_, err = r.Run(context.Background(), "test prompt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pre-hooks")
}

func TestRunner_NoTaskExecutors_Errors(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Disable all executors — ResolveTaskExecutors returns empty.
	cfg, err := quorexcfg.Parse([]byte(`
[executor.claude]
enabled = false

[executor.codex]
enabled = false

[pool]
task_executors = "*"
`))
	require.NoError(t, err)

	r := runner.New(cfg, dir)
	_, err = r.Run(context.Background(), "prompt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no task executors")
}

func TestRunner_SingleProvider_NoSynthesisPrompt(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Use 'true' as executor — succeeds immediately, produces no diff.
	cfg, err := quorexcfg.Parse([]byte(`
[executor.noop]
command = "true"
role    = "task"

[pool]
task_executors = "noop"
task_parallel  = 1
`))
	require.NoError(t, err)

	r := runner.New(cfg, dir)
	result, err := r.Run(context.Background(), "prompt")
	require.NoError(t, err)
	assert.Empty(t, result.SynthesisPrompt, "single provider must not produce a synthesis prompt")
	assert.NotNil(t, result.ReviewExecutor)
}

func TestRunner_PostHookFails_ExcludesProvider(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Only one executor; post-hook fails → all providers excluded → error.
	cfg, err := quorexcfg.Parse([]byte(`
[executor.noop]
command = "true"
role    = "task"

[pool]
task_executors = "noop"
task_parallel  = 1

[[hook]]
name    = "failing-post"
command = "false"
phase   = "post"
`))
	require.NoError(t, err)

	r := runner.New(cfg, dir)
	_, err = r.Run(context.Background(), "prompt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "excluded by post-hooks")
}

func TestRunner_Cleanup_RemovesWorktrees(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	cfg := configWithEcho(t)
	r := runner.New(cfg, dir)
	result, err := r.Run(context.Background(), "prompt")
	require.NoError(t, err)

	// Collect worktree paths before cleanup.
	var wtPaths []string
	for _, res := range result.PoolResults {
		if res.Worktree != nil {
			wtPaths = append(wtPaths, res.Worktree.Path)
		}
	}
	require.NotEmpty(t, wtPaths, "should have at least one worktree before cleanup")

	r.Cleanup(result.PoolResults)

	// Each specific worktree path should no longer exist.
	for _, p := range wtPaths {
		_, statErr := os.Stat(p)
		assert.True(t, os.IsNotExist(statErr), "worktree %s should be removed", p)
	}
}
