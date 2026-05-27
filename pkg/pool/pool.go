package pool

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alex-mextner/quorex/pkg/quorexcfg"
)

// Result holds the output of one executor run.
type Result struct {
	ExecutorName string
	Output       string
	Diff         string
	Worktree     *Worktree
	Err          error
	IsQuotaError bool
}

// Config controls pool dispatcher behavior.
type Config struct {
	RepoRoot        string
	Executors       []*quorexcfg.ExecutorDef
	Parallel        int
	Timeout         time.Duration
	ProviderRetries int
}

// Pool runs N executors in parallel, each in an isolated worktree.
type Pool struct {
	cfg Config
	wm  *WorktreeManager
}

// New creates a Pool.
func New(cfg Config) *Pool {
	if cfg.Parallel <= 0 {
		cfg.Parallel = len(cfg.Executors)
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Minute
	}
	return &Pool{cfg: cfg, wm: NewWorktreeManager(cfg.RepoRoot)}
}

// Run dispatches all executors in parallel and collects results.
// Each executor runs in its own isolated git worktree.
// On transient failure retries up to cfg.ProviderRetries times.
// On quota/billing failure marks IsQuotaError and skips retries.
func (p *Pool) Run(ctx context.Context, prompt string) ([]Result, error) {
	runID := strconv.FormatInt(time.Now().UnixMilli(), 10)

	sem := make(chan struct{}, p.cfg.Parallel)
	resultCh := make(chan Result, len(p.cfg.Executors))

	var wg sync.WaitGroup
	for _, ex := range p.cfg.Executors {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			resultCh <- p.runOne(ctx, ex, runID, prompt)
		})
	}

	wg.Wait()
	close(resultCh)

	var results []Result
	for r := range resultCh {
		results = append(results, r)
	}
	return results, nil
}

func (p *Pool) runOne(ctx context.Context, ex *quorexcfg.ExecutorDef, runID, prompt string) Result {
	wt, err := p.wm.Create(ex.Name, runID)
	if err != nil {
		return Result{ExecutorName: ex.Name, Err: fmt.Errorf("create worktree: %w", err)}
	}

	var output string
	var runErr error

	for attempt := 0; attempt <= p.cfg.ProviderRetries; attempt++ {
		execCtx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
		output, runErr = p.execInWorktree(execCtx, ex, wt, prompt)
		cancel()

		if runErr == nil {
			break
		}
		if isQuotaError(runErr) || isQuotaOutput(output) {
			return Result{
				ExecutorName: ex.Name,
				Output:       output,
				Worktree:     wt,
				Err:          runErr,
				IsQuotaError: true,
			}
		}
	}

	diff, diffErr := p.wm.Diff(wt)
	if diffErr != nil && runErr == nil {
		runErr = diffErr
	}

	return Result{
		ExecutorName: ex.Name,
		Output:       output,
		Diff:         diff,
		Worktree:     wt,
		Err:          runErr,
	}
}

func (p *Pool) execInWorktree(ctx context.Context, ex *quorexcfg.ExecutorDef, wt *Worktree, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, ex.Command, ex.Args...)
	cmd.Dir = wt.Path
	if prompt != "" {
		cmd.Stdin = strings.NewReader(prompt)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// CleanupWorktrees removes all worktrees from a run.
func (p *Pool) CleanupWorktrees(results []Result) {
	for _, r := range results {
		if r.Worktree != nil {
			p.wm.Remove(r.Worktree) //nolint:errcheck // best-effort cleanup
		}
	}
}

var quotaErrorPatterns = []string{
	"rate limit",
	"quota exceeded",
	"insufficient credits",
	"billing",
	"too many requests",
	"429",
}

func isQuotaError(err error) bool {
	if err == nil {
		return false
	}
	return containsQuotaPattern(err.Error())
}

func isQuotaOutput(output string) bool {
	return containsQuotaPattern(output)
}

func containsQuotaPattern(s string) bool {
	s = strings.ToLower(s)
	for _, pat := range quotaErrorPatterns {
		if strings.Contains(s, pat) {
			return true
		}
	}
	return false
}
