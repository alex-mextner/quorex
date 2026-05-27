// Package hooks provides pre and post hook execution for quorex.
package hooks

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/alex-mextner/quorex/pkg/quorexcfg"
)

// Runner executes hooks for a given phase.
type Runner struct {
	defs    []quorexcfg.HookDef
	repoDir string
}

// NewRunner creates a hook runner.
func NewRunner(defs []quorexcfg.HookDef, repoDir string) *Runner {
	return &Runner{defs: defs, repoDir: repoDir}
}

// RunPre runs all pre-phase hooks in the repo directory.
// Returns on first failure.
func (r *Runner) RunPre(ctx context.Context) error {
	for _, h := range r.defs {
		if h.Phase != quorexcfg.PhasePre {
			continue
		}
		if err := r.run(ctx, h, r.repoDir); err != nil {
			return fmt.Errorf("pre-hook %q failed: %w", h.Name, err)
		}
	}
	return nil
}

// RunPost runs all post-phase hooks in the given worktree directory.
// Returns error if hook fails (caller decides whether to exclude provider).
func (r *Runner) RunPost(ctx context.Context, worktreeDir string) error {
	for _, h := range r.defs {
		if h.Phase != quorexcfg.PhasePost {
			continue
		}
		if err := r.run(ctx, h, worktreeDir); err != nil {
			return fmt.Errorf("post-hook %q failed: %w", h.Name, err)
		}
	}
	return nil
}

func (r *Runner) run(ctx context.Context, h quorexcfg.HookDef, dir string) error {
	cmd := exec.CommandContext(ctx, h.Command, h.Args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("exit error: %w\noutput: %s", err, out)
	}
	return nil
}
