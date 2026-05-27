package pool_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/alex-mextner/quorex/pkg/pool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorktreeManager_CreateAndRemove(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	wm := pool.NewWorktreeManager(dir)
	wt, err := wm.Create("claude", "run-001")
	require.NoError(t, err)
	assert.DirExists(t, wt.Path)
	assert.Equal(t, "claude", wt.ExecutorName)
	assert.Equal(t, "run-001", wt.RunID)

	err = wm.Remove(wt)
	require.NoError(t, err)
	assert.NoDirExists(t, wt.Path)
}

func TestWorktreeManager_Diff(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	wm := pool.NewWorktreeManager(dir)
	wt, err := wm.Create("claude", "run-002")
	require.NoError(t, err)
	defer wm.Remove(wt) //nolint:errcheck // test cleanup

	err = os.WriteFile(filepath.Join(wt.Path, "hello.txt"), []byte("hello\n"), 0o600)
	require.NoError(t, err)

	diff, err := wm.Diff(wt)
	require.NoError(t, err)
	assert.Contains(t, diff, "hello.txt")
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		require.NoError(t, cmd.Run(), "cmd: %v", args)
	}
}
