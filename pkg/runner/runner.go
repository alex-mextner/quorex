// Package runner orchestrates the quorex execution pipeline.
package runner

import (
	"context"
	"errors"
	"fmt"

	"github.com/alex-mextner/quorex/pkg/executor"
	"github.com/alex-mextner/quorex/pkg/hooks"
	"github.com/alex-mextner/quorex/pkg/judge"
	"github.com/alex-mextner/quorex/pkg/pool"
	"github.com/alex-mextner/quorex/pkg/quorexcfg"
)

// Result is the output of the quorex pipeline through judge eval (Pass 1).
type Result struct {
	// Matrix is the scoring matrix from judge eval. Empty when only one provider succeeded.
	Matrix judge.Matrix
	// Diffs maps executor name to its git diff (passing providers only).
	Diffs map[string]string
	// PoolResults contains all pool results, including failures, for cleanup.
	PoolResults []pool.Result
	// SynthesisPrompt is ready for Pass 2 via processor.Runner.
	// Empty when only one provider succeeded (degenerate case — no synthesis needed).
	SynthesisPrompt string
	// ReviewExecutor is the executor that ran eval and should run synthesis.
	ReviewExecutor *quorexcfg.ExecutorDef
}

// Runner orchestrates pre-hooks → pool → post-hooks → judge eval (Pass 1).
type Runner struct {
	cfg     *quorexcfg.Config
	repoDir string
	p       *pool.Pool
}

// New creates a Runner.
func New(cfg *quorexcfg.Config, repoDir string) *Runner {
	return &Runner{cfg: cfg, repoDir: repoDir}
}

// Run executes the pipeline through judge eval.
// Returns a Result with the eval matrix and synthesis prompt for Pass 2.
// Call Cleanup with the returned PoolResults when done.
func (r *Runner) Run(ctx context.Context, planPrompt string) (*Result, error) {
	if err := r.runPreHooks(ctx); err != nil {
		return nil, fmt.Errorf("pre-hooks: %w", err)
	}

	taskExecs := r.cfg.ResolveTaskExecutors()
	if len(taskExecs) == 0 {
		return nil, errors.New("no task executors configured")
	}

	r.p = pool.New(pool.Config{
		RepoRoot:        r.repoDir,
		Executors:       taskExecs,
		Parallel:        r.cfg.Pool.TaskParallel,
		Timeout:         r.cfg.Pool.Timeout,
		ProviderRetries: r.cfg.Pool.ProviderRetries,
	})

	poolResults, err := r.p.Run(ctx, planPrompt)
	if err != nil {
		r.p.CleanupWorktrees(poolResults)
		return nil, fmt.Errorf("pool: %w", err)
	}

	passing := r.runPostHooks(ctx, poolResults)
	if len(passing) == 0 {
		r.p.CleanupWorktrees(poolResults)
		return nil, errors.New("all executors failed or were excluded by post-hooks")
	}

	diffs := make(map[string]string, len(passing))
	providers := make([]string, 0, len(passing))
	for _, res := range passing {
		diffs[res.ExecutorName] = res.Diff
		providers = append(providers, res.ExecutorName)
	}

	reviewExec := r.cfg.ResolveReviewExecutor()

	// Degenerate case: single provider — no judging required.
	if len(passing) == 1 {
		return &Result{
			Diffs:          diffs,
			PoolResults:    poolResults,
			ReviewExecutor: reviewExec,
		}, nil
	}

	categories := r.cfg.Judge.Categories
	if len(categories) == 0 {
		categories = judge.DefaultCategories
	}

	evalPrompt := judge.BuildEvalPrompt(diffs, categories)
	evalResult := executor.NewGenericExecutor(reviewExec.Command, reviewExec.Args).Run(ctx, evalPrompt)
	if evalResult.Error != nil {
		r.p.CleanupWorktrees(poolResults)
		return nil, fmt.Errorf("judge eval: %w", evalResult.Error)
	}

	matrix := judge.ParseMatrix(evalResult.Output, providers)

	return &Result{
		Matrix:          matrix,
		Diffs:           diffs,
		PoolResults:     poolResults,
		SynthesisPrompt: judge.BuildSynthesisPrompt(planPrompt, matrix, diffs),
		ReviewExecutor:  reviewExec,
	}, nil
}

// Cleanup removes all worktrees created during the run.
func (r *Runner) Cleanup(results []pool.Result) {
	if r.p != nil {
		r.p.CleanupWorktrees(results)
	}
}

func (r *Runner) runPreHooks(ctx context.Context) error {
	if err := hooks.NewRunner(r.cfg.HooksForPhase(quorexcfg.PhasePre), r.repoDir).RunPre(ctx); err != nil {
		return fmt.Errorf("pre-hook: %w", err)
	}
	return nil
}

// runPostHooks runs post-phase hooks per worktree.
// Returns only results whose worktrees passed all post-hooks (and had no pool errors).
func (r *Runner) runPostHooks(ctx context.Context, results []pool.Result) []pool.Result {
	hookDefs := r.cfg.HooksForPhase(quorexcfg.PhasePost)
	var passing []pool.Result
	for _, res := range results {
		if res.Err != nil || res.Worktree == nil {
			continue
		}
		if len(hookDefs) == 0 {
			passing = append(passing, res)
			continue
		}
		if err := hooks.NewRunner(hookDefs, r.repoDir).RunPost(ctx, res.Worktree.Path); err == nil {
			passing = append(passing, res)
		}
	}
	return passing
}
