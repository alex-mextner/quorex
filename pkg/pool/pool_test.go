package pool_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/alex-mextner/quorex/pkg/pool"
	"github.com/alex-mextner/quorex/pkg/quorexcfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPool_RunParallel_AllSucceed(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	executors := []*quorexcfg.ExecutorDef{
		{Name: "echo-a", Command: "sh", Args: []string{"-c", "echo done_a"}, Role: quorexcfg.RoleTask, Mode: quorexcfg.ModeEdit, Enabled: true},
		{Name: "echo-b", Command: "sh", Args: []string{"-c", "echo done_b"}, Role: quorexcfg.RoleTask, Mode: quorexcfg.ModeEdit, Enabled: true},
	}

	p := pool.New(pool.Config{
		RepoRoot:        dir,
		Executors:       executors,
		Parallel:        2,
		ProviderRetries: 0,
	})

	results, err := p.Run(context.Background(), "test prompt")
	require.NoError(t, err)
	assert.Len(t, results, 2)
	for _, r := range results {
		require.NoError(t, r.Err)
		assert.NotEmpty(t, r.Output)
	}
	p.CleanupWorktrees(results)
}

func TestPool_RunParallel_OneFailsRetrySucceeds(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// script fails on first call, succeeds on second
	scriptPath := filepath.Join(dir, "flaky.sh")
	script := "#!/bin/sh\ncounter_file=\"$(dirname \"$0\")/count\"\ncount=0\n" +
		"[ -f \"$counter_file\" ] && count=$(cat \"$counter_file\")\ncount=$((count+1))\n" +
		"printf '%d' $count > \"$counter_file\"\nif [ \"$count\" -eq 1 ]; then exit 1; fi\n" +
		"echo \"success on attempt $count\"\n"
	err := os.WriteFile(scriptPath, []byte(script), 0o600)
	require.NoError(t, err)
	require.NoError(t, os.Chmod(scriptPath, 0o755)) //nolint:gosec // test script must be executable
	require.NoError(t, err)

	executors := []*quorexcfg.ExecutorDef{
		{Name: "flaky", Command: "sh", Args: []string{scriptPath}, Role: quorexcfg.RoleTask, Mode: quorexcfg.ModeEdit, Enabled: true},
	}

	p := pool.New(pool.Config{
		RepoRoot:        dir,
		Executors:       executors,
		Parallel:        1,
		ProviderRetries: 1,
	})

	results, err := p.Run(context.Background(), "prompt")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NoError(t, results[0].Err)
	assert.Contains(t, results[0].Output, "success on attempt 2")
	p.CleanupWorktrees(results)
}

func TestPool_QuotaError_NoRetry(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	executors := []*quorexcfg.ExecutorDef{
		{Name: "quota-fail", Command: "sh", Args: []string{"-c", "echo 'rate limit exceeded'; exit 1"}, Role: quorexcfg.RoleTask, Mode: quorexcfg.ModeEdit, Enabled: true},
	}

	p := pool.New(pool.Config{
		RepoRoot:        dir,
		Executors:       executors,
		Parallel:        1,
		ProviderRetries: 3, // should not retry quota errors
	})

	results, err := p.Run(context.Background(), "prompt")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Error(t, results[0].Err)
	assert.True(t, results[0].IsQuotaError)
	p.CleanupWorktrees(results)
}
