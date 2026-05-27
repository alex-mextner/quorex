// Package pool provides the parallel executor pool for quorex.
package pool

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree represents an isolated git worktree for one executor run.
type Worktree struct {
	Path         string
	Branch       string
	ExecutorName string
	RunID        string
}

// WorktreeManager creates and removes per-executor worktrees.
type WorktreeManager struct {
	repoRoot string
}

// NewWorktreeManager creates a manager rooted at repoRoot.
func NewWorktreeManager(repoRoot string) *WorktreeManager {
	return &WorktreeManager{repoRoot: repoRoot}
}

// Create creates an isolated worktree for the given executor and run ID.
// Path: <repoRoot>/.quorex/worktrees/<executor>/<runID>
func (m *WorktreeManager) Create(executorName, runID string) (*Worktree, error) {
	wtPath := filepath.Join(m.repoRoot, ".quorex", "worktrees", executorName, runID)
	branch := fmt.Sprintf("quorex/%s/%s", executorName, runID)

	cmd := exec.Command("git", "worktree", "add", "-b", branch, wtPath, "HEAD") //nolint:noctx // lifecycle op, no context needed
	cmd.Dir = m.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git worktree add for %s/%s: %w\n%s", executorName, runID, err, out)
	}

	return &Worktree{
		Path:         wtPath,
		Branch:       branch,
		ExecutorName: executorName,
		RunID:        runID,
	}, nil
}

// Diff returns the unified diff of changes in the worktree vs HEAD (staged+unstaged).
func (m *WorktreeManager) Diff(wt *Worktree) (string, error) {
	addCmd := exec.Command("git", "add", "-A") //nolint:noctx // lifecycle op
	addCmd.Dir = wt.Path
	if out, err := addCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add in worktree %s: %w\n%s", wt.ExecutorName, err, out)
	}

	diffCmd := exec.Command("git", "diff", "--cached", "HEAD") //nolint:noctx // lifecycle op
	diffCmd.Dir = wt.Path
	out, err := diffCmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff in worktree %s: %w", wt.ExecutorName, err)
	}
	return string(out), nil
}

// Apply applies a worktree's staged diff to the repo root via patch.
func (m *WorktreeManager) Apply(wt *Worktree) error {
	diff, err := m.Diff(wt)
	if err != nil {
		return err
	}
	if strings.TrimSpace(diff) == "" {
		return nil
	}

	cmd := exec.Command("git", "apply", "--index", "-") //nolint:noctx // lifecycle op
	cmd.Dir = m.repoRoot
	cmd.Stdin = strings.NewReader(diff)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git apply from worktree %s: %w\n%s", wt.ExecutorName, err, out)
	}
	return nil
}

// Remove deletes the worktree directory and its associated branch.
func (m *WorktreeManager) Remove(wt *Worktree) error {
	rmCmd := exec.Command("git", "worktree", "remove", "--force", wt.Path) //nolint:noctx // lifecycle op
	rmCmd.Dir = m.repoRoot
	if out, err := rmCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("remove worktree %s: %w\n%s", wt.ExecutorName, err, out)
	}
	exec.Command("git", "branch", "-D", wt.Branch).Run() //nolint:errcheck,noctx // best-effort cleanup
	return nil
}
