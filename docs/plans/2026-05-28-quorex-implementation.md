# Quorex Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the ralphex fork into Quorex — a multi-provider parallel CLI that dispatches coding tasks to N AI executors simultaneously, judges their outputs, and synthesizes the best solution.

**Architecture:** Quorex keeps ralphex's plan/review/git/notify pipeline intact. It adds: (1) a TOML project config (`quorex.toml`) with named executor model, (2) a parallel pool dispatcher that runs each executor in an isolated git worktree, (3) a two-pass judge (eval matrix → synthesis as a ralphex iteration), (4) provider failure/retry/refill, (5) pre/post hooks, (6) plan format validation, (7) `quorex init`.

**Tech Stack:** Go 1.26, `github.com/BurntSushi/toml` (TOML config), `github.com/go-pkgz/notify` (Telegram/Slack/email, already in deps), existing `pkg/git` worktree API, `github.com/stretchr/testify` (tests).

---

## File Map

New files:
- `cmd/quorex/main.go` — entry point (replaces `cmd/ralphex/main.go`)
- `pkg/quorexcfg/config.go` — TOML project config (quorex.toml schema)
- `pkg/quorexcfg/config_test.go`
- `pkg/pool/pool.go` — parallel pool dispatcher
- `pkg/pool/pool_test.go`
- `pkg/pool/worktree.go` — per-executor worktree lifecycle
- `pkg/pool/worktree_test.go`
- `pkg/judge/judge.go` — two-pass judge (eval + synthesis)
- `pkg/judge/judge_test.go`
- `pkg/judge/matrix.go` — scoring matrix types and rendering
- `pkg/hooks/hooks.go` — pre/post hook runner
- `pkg/hooks/hooks_test.go`
- `pkg/planval/planval.go` — plan format validator
- `pkg/planval/planval_test.go`
- `pkg/initcmd/init.go` — `quorex init` command
- `AGENTS.md` — project agent instructions
- `.claude/settings.json` — Claude Code hooks (lint, test before commit)

Modified files:
- `go.mod` — rename module + add toml dep
- `go.sum` — updated after `go get`
- `Makefile` — rename targets, add quorex-specific commands

---

## Task 1: Module Rename + Binary Rename

**Files:**
- Modify: `go.mod`
- Rename: `cmd/ralphex/` → `cmd/quorex/`
- Modify: all `*.go` importing `github.com/umputun/ralphex`

- [ ] **Step 1: Rename module in go.mod**

```
# in /Users/ultra/xp/quorex
sed -i '' 's|github.com/umputun/ralphex|github.com/alex-mextner/quorex|g' go.mod
```

- [ ] **Step 2: Update all Go imports**

```bash
find . -name "*.go" | xargs sed -i '' 's|github.com/umputun/ralphex|github.com/alex-mextner/quorex|g'
```

- [ ] **Step 3: Rename cmd directory**

```bash
mv cmd/ralphex cmd/quorex
```

- [ ] **Step 4: Update binary name references in main.go**

In `cmd/quorex/main.go`, replace all occurrences of "ralphex" with "quorex" in user-facing strings (version output, help text).

- [ ] **Step 5: Update Makefile**

```makefile
# replace ralphex → quorex in all build/run targets
sed -i '' 's/ralphex/quorex/g' Makefile
```

- [ ] **Step 6: Update vendor directory**

```bash
go mod vendor
```

- [ ] **Step 7: Build to verify**

```bash
go build ./cmd/quorex/
```

Expected: binary `quorex` produced, no compilation errors.

- [ ] **Step 8: Run existing tests**

```bash
go test ./... 2>&1 | tail -30
```

Expected: all existing tests pass (some may skip due to missing `claude` binary).

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "feat: rename module and binary ralphex → quorex"
```

---

## Task 2: Project Setup (AGENTS.md, git hooks, Makefile)

**Files:**
- Create: `AGENTS.md`
- Create: `.claude/settings.json`
- Modify: `Makefile`

- [ ] **Step 1: Write AGENTS.md**

```markdown
# Quorex — Agent Instructions

Quorex is a Go CLI fork of ralphex that adds multi-provider parallel execution.

## Build & Test

```bash
go build ./cmd/quorex/           # build binary
go test ./...                    # run all tests
go test ./pkg/pool/... -v        # run specific package tests
make lint                        # golangci-lint
```

## Key Packages

- `pkg/quorexcfg/` — TOML project config (quorex.toml)
- `pkg/pool/` — parallel pool dispatcher + worktree lifecycle
- `pkg/judge/` — two-pass judge (eval matrix + synthesis)
- `pkg/hooks/` — pre/post hook runner
- `pkg/planval/` — plan format validator
- `pkg/initcmd/` — `quorex init` command
- `pkg/executor/` — base executor (ClaudeExecutor, CodexExecutor, GenericExecutor)
- `pkg/processor/` — ralphex main loop (task iterations, review pipeline)
- `pkg/git/` — git operations including worktree management

## Commit Rules

- Run `go build ./...` and `go test ./...` before committing
- Run `make lint` before committing
- Atomic commits: one logical change per commit
- Commit message: `feat:`, `fix:`, `test:`, `refactor:` prefix

## Architecture Invariant

Each task executor runs in an isolated `git worktree` at `quorex/<name>/<run-id>`.
Executors cannot see each other's writes. After judging, winning diff applied via `git apply`.
Worktrees are cleaned up on exit (even on error/signal).

## quorex.toml location

Searched in current directory, then parent directories up to git root.
Global config still lives in `~/.config/quorex/` (INI format, backward compat).
```

- [ ] **Step 2: Write `.claude/settings.json` hooks**

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "if echo \"$CLAUDE_TOOL_INPUT\" | grep -q 'git commit'; then cd /Users/ultra/xp/quorex && go build ./... && go test ./... && make lint; fi"
          }
        ]
      }
    ]
  }
}
```

- [ ] **Step 3: Add lint and test targets to Makefile**

Ensure Makefile has:

```makefile
lint:
	golangci-lint run ./...

test:
	go test ./... -race -count=1

build:
	go build -o quorex ./cmd/quorex/
```

- [ ] **Step 4: Commit**

```bash
git add AGENTS.md .claude/settings.json Makefile
git commit -m "chore: add AGENTS.md, Claude hooks, Makefile targets"
```

---

## Task 3: Add TOML Dependency + quorexcfg Package

**Files:**
- Create: `pkg/quorexcfg/config.go`
- Create: `pkg/quorexcfg/config_test.go`
- Modify: `go.mod`, `go.sum`, `vendor/`

- [ ] **Step 1: Add toml dependency**

```bash
go get github.com/BurntSushi/toml@latest
go mod vendor
```

- [ ] **Step 2: Write failing test for config loading**

`pkg/quorexcfg/config_test.go`:

```go
package quorexcfg_test

import (
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/alex-mextner/quorex/pkg/quorexcfg"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestLoad_MinimalConfig(t *testing.T) {
    dir := t.TempDir()
    f := filepath.Join(dir, "quorex.toml")
    err := os.WriteFile(f, []byte(`
[pool]
task_executors   = "claude,codex"
task_parallel    = 2
review_executors = "claude"
timeout          = "10m"
provider_retries = 1
`), 0o644)
    require.NoError(t, err)

    cfg, err := quorexcfg.LoadFile(f)
    require.NoError(t, err)
    assert.Equal(t, []string{"claude", "codex"}, cfg.Pool.TaskExecutors)
    assert.Equal(t, 2, cfg.Pool.TaskParallel)
    assert.Equal(t, []string{"claude"}, cfg.Pool.ReviewExecutors)
    assert.Equal(t, 10*time.Minute, cfg.Pool.Timeout)
    assert.Equal(t, 1, cfg.Pool.ProviderRetries)
}

func TestLoad_ExecutorDefaults(t *testing.T) {
    dir := t.TempDir()
    f := filepath.Join(dir, "quorex.toml")
    err := os.WriteFile(f, []byte(`
[executor.claude]
args     = ["--output-format", "stream-json", "--dangerously-skip-permissions"]
protocol = "claude-json-stream"

[executor.gemini]
command  = "gemini"
args     = ["--yolo"]
protocol = "plain"
`), 0o644)
    require.NoError(t, err)

    cfg, err := quorexcfg.LoadFile(f)
    require.NoError(t, err)
    
    claude := cfg.Executors["claude"]
    assert.Equal(t, "claude", claude.Command) // inferred for built-in
    assert.Equal(t, []string{"--output-format", "stream-json", "--dangerously-skip-permissions"}, claude.Args)
    assert.Equal(t, "claude-json-stream", claude.Protocol)
    assert.Equal(t, "task", string(claude.Role))
    assert.Equal(t, "edit", string(claude.Mode))
    assert.True(t, claude.Enabled)

    gemini := cfg.Executors["gemini"]
    assert.Equal(t, "gemini", gemini.Command)
    assert.Equal(t, []string{"--yolo"}, gemini.Args)
    assert.Equal(t, "plain", gemini.Protocol)
}

func TestLoad_Hooks(t *testing.T) {
    dir := t.TempDir()
    f := filepath.Join(dir, "quorex.toml")
    err := os.WriteFile(f, []byte(`
[[hook]]
name    = "typecheck"
command = "bun"
args    = ["tsc", "--noEmit"]
phase   = "pre"

[[hook]]
name    = "tests"
command = "bun"
args    = ["test", "--bail"]
phase   = "post"
`), 0o644)
    require.NoError(t, err)

    cfg, err := quorexcfg.LoadFile(f)
    require.NoError(t, err)
    require.Len(t, cfg.Hooks, 2)
    assert.Equal(t, "typecheck", cfg.Hooks[0].Name)
    assert.Equal(t, quorexcfg.PhasePre, cfg.Hooks[0].Phase)
    assert.Equal(t, "tests", cfg.Hooks[1].Name)
    assert.Equal(t, quorexcfg.PhasePost, cfg.Hooks[1].Phase)
}

func TestLoad_WildcardTaskExecutors(t *testing.T) {
    dir := t.TempDir()
    f := filepath.Join(dir, "quorex.toml")
    err := os.WriteFile(f, []byte(`
[executor.claude]
args = ["--dangerously-skip-permissions"]

[executor.codex]
args = ["--full-auto"]

[executor.disabled]
command = "something"
enabled = false

[pool]
task_executors = "*"
`), 0o644)
    require.NoError(t, err)

    cfg, err := quorexcfg.LoadFile(f)
    require.NoError(t, err)

    resolved := cfg.ResolveTaskExecutors()
    assert.Len(t, resolved, 2) // disabled excluded
    names := make([]string, len(resolved))
    for i, e := range resolved { names[i] = e.Name }
    assert.ElementsMatch(t, []string{"claude", "codex"}, names)
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd /Users/ultra/xp/quorex && go test ./pkg/quorexcfg/... 2>&1 | tail -10
```

Expected: FAIL — package doesn't exist yet.

- [ ] **Step 4: Implement quorexcfg package**

`pkg/quorexcfg/config.go`:

```go
// Package quorexcfg provides TOML project configuration for quorex.
package quorexcfg

import (
    "fmt"
    "os"
    "strings"
    "time"

    "github.com/BurntSushi/toml"
)

// Role is the executor's role in the pipeline.
type Role string

const (
    RoleTask   Role = "task"
    RoleReview Role = "review"
)

// Mode controls whether an executor can write files.
type Mode string

const (
    ModeEdit  Mode = "edit"
    ModeAudit Mode = "audit"
)

// Phase is the hook execution phase.
type Phase string

const (
    PhasePre  Phase = "pre"
    PhasePost Phase = "post"
)

// builtin executor names with their default commands.
var builtinCommands = map[string]string{
    "claude": "claude",
    "codex":  "codex",
}

// ExecutorDef defines a named executor.
type ExecutorDef struct {
    Name     string   // populated from map key during load
    Command  string   `toml:"command"`
    Args     []string `toml:"args"`
    Role     Role     `toml:"role"`
    Mode     Mode     `toml:"mode"`
    Protocol string   `toml:"protocol"`
    Enabled  bool     `toml:"enabled"`
}

// PoolConfig controls how executors are selected and run.
type PoolConfig struct {
    TaskExecutorsRaw  string        `toml:"task_executors"`   // "*" or "claude,codex"
    TaskParallel      int           `toml:"task_parallel"`
    ReviewExecutors   []string      `toml:"-"`                // parsed from review_executors
    ReviewExecutorsRaw string       `toml:"review_executors"` // "claude" or "claude,deepseek"
    Timeout           time.Duration `toml:"-"`                // parsed from timeout string
    TimeoutRaw        string        `toml:"timeout"`
    ProviderRetries   int           `toml:"provider_retries"`
}

// TaskExecutors returns the list or wildcard flag from raw config.
func (p *PoolConfig) TaskExecutors() []string {
    if p.TaskExecutorsRaw == "*" {
        return nil // wildcard — resolved via Config.ResolveTaskExecutors
    }
    return splitComma(p.TaskExecutorsRaw)
}

// JudgeConfig controls evaluation categories.
type JudgeConfig struct {
    Categories []string `toml:"categories"`
}

// HookDef defines a pre or post hook.
type HookDef struct {
    Name    string   `toml:"name"`
    Command string   `toml:"command"`
    Args    []string `toml:"args"`
    Phase   Phase    `toml:"phase"`
}

// rawConfig matches the TOML file structure exactly (executors as map).
type rawConfig struct {
    Executor map[string]ExecutorDef `toml:"executor"`
    Pool     PoolConfig             `toml:"pool"`
    Judge    JudgeConfig            `toml:"judge"`
    Hook     []HookDef              `toml:"hook"`
}

// Config is the parsed and resolved quorex project configuration.
type Config struct {
    Executors map[string]*ExecutorDef
    Pool      PoolConfig
    Judge     JudgeConfig
    Hooks     []HookDef
}

// LoadFile loads and parses a quorex.toml file.
func LoadFile(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read %s: %w", path, err)
    }
    return Parse(data)
}

// Parse parses TOML bytes into a Config.
func Parse(data []byte) (*Config, error) {
    var raw rawConfig
    if _, err := toml.Decode(string(data), &raw); err != nil {
        return nil, fmt.Errorf("parse toml: %w", err)
    }

    cfg := &Config{
        Executors: make(map[string]*ExecutorDef),
        Pool:      raw.Pool,
        Judge:     raw.Judge,
        Hooks:     raw.Hook,
    }

    // apply built-in defaults then overlays
    for name, cmd := range builtinCommands {
        e := &ExecutorDef{
            Name:    name,
            Command: cmd,
            Role:    RoleTask,
            Mode:    ModeEdit,
            Enabled: true,
        }
        if overlay, ok := raw.Executor[name]; ok {
            applyOverlay(e, overlay)
        }
        cfg.Executors[name] = e
    }

    // user-defined executors (not in builtins)
    for name, def := range raw.Executor {
        if _, isBuiltin := builtinCommands[name]; isBuiltin {
            continue // already handled above
        }
        d := def
        d.Name = name
        if d.Role == "" {
            d.Role = RoleTask
        }
        if d.Mode == "" {
            d.Mode = ModeEdit
        }
        if !d.Enabled && d.Command == "" {
            // default enabled=true only when command is set
            d.Enabled = true
        }
        // if enabled wasn't explicitly set in TOML, default to true
        // BurntSushi/toml zero-value for bool is false; we use Command presence as signal
        if d.Command != "" {
            d.Enabled = true
        }
        cfg.Executors[name] = &d
    }

    // fix enabled defaults: built-ins are always enabled unless explicitly set to false
    // parse pool timeout
    if raw.Pool.TimeoutRaw != "" {
        dur, err := time.ParseDuration(raw.Pool.TimeoutRaw)
        if err != nil {
            return nil, fmt.Errorf("pool.timeout: %w", err)
        }
        cfg.Pool.Timeout = dur
    }
    if cfg.Pool.Timeout == 0 {
        cfg.Pool.Timeout = 10 * time.Minute
    }

    // parse review executors
    cfg.Pool.ReviewExecutors = splitComma(raw.Pool.ReviewExecutorsRaw)

    // set hook phase defaults
    for i := range cfg.Hooks {
        if cfg.Hooks[i].Phase == "" {
            cfg.Hooks[i].Phase = PhasePre
        }
    }

    return cfg, nil
}

// ResolveTaskExecutors returns the list of enabled task executors.
// If task_executors = "*", returns all enabled task-role executors.
// Otherwise filters the named list.
func (c *Config) ResolveTaskExecutors() []*ExecutorDef {
    var result []*ExecutorDef
    raw := c.Pool.TaskExecutorsRaw

    if raw == "" || raw == "*" {
        for _, e := range c.Executors {
            if e.Enabled && e.Role == RoleTask {
                result = append(result, e)
            }
        }
        return result
    }

    for _, name := range splitComma(raw) {
        if e, ok := c.Executors[name]; ok && e.Enabled {
            result = append(result, e)
        }
    }
    return result
}

// HooksForPhase returns all hooks matching the given phase.
func (c *Config) HooksForPhase(phase Phase) []HookDef {
    var out []HookDef
    for _, h := range c.Hooks {
        if h.Phase == phase {
            out = append(out, h)
        }
    }
    return out
}

func applyOverlay(base *ExecutorDef, overlay ExecutorDef) {
    if overlay.Command != "" {
        base.Command = overlay.Command
    }
    if len(overlay.Args) > 0 {
        base.Args = overlay.Args
    }
    if overlay.Role != "" {
        base.Role = overlay.Role
    }
    if overlay.Mode != "" {
        base.Mode = overlay.Mode
    }
    if overlay.Protocol != "" {
        base.Protocol = overlay.Protocol
    }
    // explicit false disables a builtin
    base.Enabled = overlay.Enabled || overlay.Command != "" || len(overlay.Args) > 0
}

func splitComma(s string) []string {
    if s == "" {
        return nil
    }
    parts := strings.Split(s, ",")
    out := make([]string, 0, len(parts))
    for _, p := range parts {
        if t := strings.TrimSpace(p); t != "" {
            out = append(out, t)
        }
    }
    return out
}
```

- [ ] **Step 5: Run tests**

```bash
cd /Users/ultra/xp/quorex && go test ./pkg/quorexcfg/... -v 2>&1 | tail -20
```

Expected: PASS all tests.

- [ ] **Step 6: Build check**

```bash
go build ./...
```

- [ ] **Step 7: Commit**

```bash
git add pkg/quorexcfg/ go.mod go.sum vendor/
git commit -m "feat(quorexcfg): TOML project config with named executor model"
```

---

## Task 4: Pool Worktree Lifecycle

**Files:**
- Create: `pkg/pool/worktree.go`
- Create: `pkg/pool/worktree_test.go`

This wraps `pkg/git`'s existing worktree API.

- [ ] **Step 1: Write failing tests**

`pkg/pool/worktree_test.go`:

```go
package pool_test

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/alex-mextner/quorex/pkg/pool"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestWorktreeManager_CreateAndRemove(t *testing.T) {
    // requires a git repo in temp dir
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
    defer wm.Remove(wt) //nolint:errcheck

    // write a file in the worktree
    err = os.WriteFile(filepath.Join(wt.Path, "hello.txt"), []byte("hello\n"), 0o644)
    require.NoError(t, err)

    diff, err := wm.Diff(wt)
    require.NoError(t, err)
    assert.Contains(t, diff, "hello.txt")
}

// initGitRepo creates a minimal git repo for testing.
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
```

- [ ] **Step 2: Run failing test**

```bash
go test ./pkg/pool/... 2>&1 | head -10
```

Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Implement worktree.go**

`pkg/pool/worktree.go`:

```go
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
// The worktree is created at <repoRoot>/quorex/<executor>/<runID>.
func (m *WorktreeManager) Create(executorName, runID string) (*Worktree, error) {
    wtPath := filepath.Join(m.repoRoot, "quorex", executorName, runID)
    branch := fmt.Sprintf("quorex/%s/%s", executorName, runID)

    cmd := exec.Command("git", "worktree", "add", "-b", branch, wtPath, "HEAD")
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

// Diff returns the unified diff of changes in the worktree vs HEAD.
func (m *WorktreeManager) Diff(wt *Worktree) (string, error) {
    // stage all changes to capture untracked files in diff
    addCmd := exec.Command("git", "add", "-A")
    addCmd.Dir = wt.Path
    if out, err := addCmd.CombinedOutput(); err != nil {
        return "", fmt.Errorf("git add in worktree %s: %w\n%s", wt.ExecutorName, err, out)
    }

    diffCmd := exec.Command("git", "diff", "--cached", "HEAD")
    diffCmd.Dir = wt.Path
    out, err := diffCmd.Output()
    if err != nil {
        return "", fmt.Errorf("git diff in worktree %s: %w", wt.ExecutorName, err)
    }
    return string(out), nil
}

// Apply applies a worktree's diff to the repo root.
func (m *WorktreeManager) Apply(wt *Worktree) error {
    diff, err := m.Diff(wt)
    if err != nil {
        return err
    }
    if strings.TrimSpace(diff) == "" {
        return nil // no changes
    }

    cmd := exec.Command("git", "apply", "--index", "-")
    cmd.Dir = m.repoRoot
    cmd.Stdin = strings.NewReader(diff)
    if out, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("git apply from worktree %s: %w\n%s", wt.ExecutorName, err, out)
    }
    return nil
}

// Remove deletes the worktree and its branch.
func (m *WorktreeManager) Remove(wt *Worktree) error {
    rmCmd := exec.Command("git", "worktree", "remove", "--force", wt.Path)
    rmCmd.Dir = m.repoRoot
    if out, err := rmCmd.CombinedOutput(); err != nil {
        return fmt.Errorf("remove worktree %s: %w\n%s", wt.ExecutorName, err, out)
    }

    branchCmd := exec.Command("git", "branch", "-D", wt.Branch)
    branchCmd.Dir = m.repoRoot
    branchCmd.CombinedOutput() //nolint:errcheck // best-effort cleanup

    return nil
}

// RemoveAll removes all worktrees created by this manager for a given run ID.
func (m *WorktreeManager) RemoveAll(runID string) error {
    pruneCmd := exec.Command("git", "worktree", "prune")
    pruneCmd.Dir = m.repoRoot
    pruneCmd.CombinedOutput() //nolint:errcheck // best-effort
    return nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./pkg/pool/... -v -run TestWorktree 2>&1 | tail -20
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/pool/
git commit -m "feat(pool): per-executor git worktree lifecycle manager"
```

---

## Task 5: Hooks Runner

**Files:**
- Create: `pkg/hooks/hooks.go`
- Create: `pkg/hooks/hooks_test.go`

- [ ] **Step 1: Write failing tests**

`pkg/hooks/hooks_test.go`:

```go
package hooks_test

import (
    "context"
    "testing"

    "github.com/alex-mextner/quorex/pkg/hooks"
    "github.com/alex-mextner/quorex/pkg/quorexcfg"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestRunner_PreHooks_Success(t *testing.T) {
    defs := []quorexcfg.HookDef{
        {Name: "ok", Command: "true", Phase: quorexcfg.PhasePre},
    }
    r := hooks.NewRunner(defs, ".")
    err := r.RunPre(context.Background())
    require.NoError(t, err)
}

func TestRunner_PreHooks_Failure(t *testing.T) {
    defs := []quorexcfg.HookDef{
        {Name: "fail", Command: "false", Phase: quorexcfg.PhasePre},
    }
    r := hooks.NewRunner(defs, ".")
    err := r.RunPre(context.Background())
    require.Error(t, err)
    assert.Contains(t, err.Error(), "fail")
}

func TestRunner_PostHooks_WorktreeDir(t *testing.T) {
    dir := t.TempDir()
    defs := []quorexcfg.HookDef{
        {Name: "check", Command: "sh", Args: []string{"-c", "test -d ."}, Phase: quorexcfg.PhasePost},
    }
    r := hooks.NewRunner(defs, ".")
    err := r.RunPost(context.Background(), dir)
    require.NoError(t, err)
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./pkg/hooks/... 2>&1 | head -5
```

- [ ] **Step 3: Implement hooks.go**

`pkg/hooks/hooks.go`:

```go
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
    cmd := exec.CommandContext(ctx, h.Command, h.Args...) //nolint:gosec // user-configured hooks
    cmd.Dir = dir
    if out, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("exit error: %w\noutput: %s", err, out)
    }
    return nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./pkg/hooks/... -v 2>&1 | tail -15
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/hooks/
git commit -m "feat(hooks): pre/post hook runner"
```

---

## Task 6: Plan Format Validator

**Files:**
- Create: `pkg/planval/planval.go`
- Create: `pkg/planval/planval_test.go`

- [ ] **Step 1: Write failing tests**

`pkg/planval/planval_test.go`:

```go
package planval_test

import (
    "testing"

    "github.com/alex-mextner/quorex/pkg/planval"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

const validPlan = `# My Feature

## Task: implement foo

Add the foo function.

## Task: add tests

Write tests for foo.
`

const missingTaskHeader = `# My Feature

## implement foo

Add the foo function.
`

const unclosedFence = "# My Feature\n\n## Task: implement foo\n\n```go\nfunc foo() {}\n"

func TestValidate_ValidPlan(t *testing.T) {
    errs := planval.Validate([]byte(validPlan))
    assert.Empty(t, errs)
}

func TestValidate_MissingTaskHeader(t *testing.T) {
    errs := planval.Validate([]byte(missingTaskHeader))
    require.Len(t, errs, 1)
    assert.Contains(t, errs[0].Message, "Task:")
    assert.Equal(t, 3, errs[0].Line)
}

func TestValidate_UnclosedFence(t *testing.T) {
    errs := planval.Validate([]byte(unclosedFence))
    require.Len(t, errs, 1)
    assert.Contains(t, errs[0].Message, "fence block not closed")
}

func TestFormatError(t *testing.T) {
    errs := planval.Validate([]byte(missingTaskHeader))
    msg := planval.FormatErrors("plans/test.md", errs)
    assert.Contains(t, msg, `Error: invalid plan file "plans/test.md"`)
    assert.Contains(t, msg, "quorex plans fix")
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./pkg/planval/... 2>&1 | head -5
```

- [ ] **Step 3: Implement planval.go**

`pkg/planval/planval.go`:

```go
// Package planval validates ralphex/quorex plan file format.
package planval

import (
    "bufio"
    "bytes"
    "fmt"
    "strings"
)

// ValidationError is a single format problem in a plan file.
type ValidationError struct {
    Line    int
    Message string
}

// Validate checks a plan file and returns all format errors found.
func Validate(data []byte) []ValidationError {
    var errs []ValidationError
    scanner := bufio.NewScanner(bytes.NewReader(data))

    var lineNum int
    var fenceOpen int    // line where open fence started (0 = none)
    var fenceLang string // language of open fence
    var hasTaskSection bool

    for scanner.Scan() {
        lineNum++
        line := scanner.Text()

        // track code fences
        if strings.HasPrefix(line, "```") {
            if fenceOpen == 0 {
                fenceOpen = lineNum
                fenceLang = strings.TrimPrefix(line, "```")
            } else {
                fenceOpen = 0
                fenceLang = ""
            }
            continue
        }

        if fenceOpen > 0 {
            continue // inside fence, skip section header checks
        }

        // check ## headings: task sections must use "## Task: <name>"
        if strings.HasPrefix(line, "## ") {
            content := strings.TrimPrefix(line, "## ")
            if strings.HasPrefix(strings.ToLower(content), "task") && !strings.HasPrefix(content, "Task: ") {
                errs = append(errs, ValidationError{
                    Line:    lineNum,
                    Message: fmt.Sprintf("task section missing required header format\n  Expected: ## Task: <name>\n  Found:    ## %s", content),
                })
            }
            if strings.HasPrefix(content, "Task: ") {
                hasTaskSection = true
            }
        }
    }

    // check for unclosed fence
    if fenceOpen > 0 {
        errs = append(errs, ValidationError{
            Line:    fenceOpen,
            Message: fmt.Sprintf("fence block not closed\n  Opened at line %d (```%s), never closed before end of file", fenceOpen, fenceLang),
        })
    }

    // warn if no task sections found
    _ = hasTaskSection // future: could warn on empty plans

    return errs
}

// FormatErrors produces the user-facing error message with fix suggestion.
func FormatErrors(filename string, errs []ValidationError) string {
    if len(errs) == 0 {
        return ""
    }
    var b strings.Builder
    fmt.Fprintf(&b, "Error: invalid plan file %q\n\n", filename)
    for _, e := range errs {
        fmt.Fprintf(&b, "  Line %d: %s\n\n", e.Line, e.Message)
    }
    fmt.Fprintf(&b, "Fix manually or run:\n  quorex plans fix %s\n", filename)
    return b.String()
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./pkg/planval/... -v 2>&1 | tail -15
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/planval/
git commit -m "feat(planval): plan format validator with fix suggestion"
```

---

## Task 7: Generic Executor Abstraction

**Files:**
- Create: `pkg/executor/generic.go`
- Create: `pkg/executor/generic_test.go`

The existing `ClaudeExecutor` handles `claude-json-stream` protocol. We need a `GenericExecutor` for `plain` protocol (other tools that just write stdout).

- [ ] **Step 1: Write failing test**

`pkg/executor/generic_test.go`:

```go
package executor_test

import (
    "context"
    "testing"

    "github.com/alex-mextner/quorex/pkg/executor"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestGenericExecutor_Run_Success(t *testing.T) {
    ex := executor.NewGenericExecutor("echo", []string{"hello world"})
    result := ex.Run(context.Background(), "")
    require.NoError(t, result.Error)
    assert.Contains(t, result.Output, "hello world")
}

func TestGenericExecutor_Run_Failure(t *testing.T) {
    ex := executor.NewGenericExecutor("false", nil)
    result := ex.Run(context.Background(), "")
    require.Error(t, result.Error)
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./pkg/executor/... -run TestGenericExecutor 2>&1 | head -10
```

- [ ] **Step 3: Implement generic.go**

`pkg/executor/generic.go`:

```go
package executor

import (
    "context"
    "fmt"
    "os/exec"
    "strings"
)

// GenericExecutor runs any command and captures its combined stdout+stderr as output.
// Used for "plain" protocol executors (gemini --yolo, opencode, etc).
type GenericExecutor struct {
    Command string
    Args    []string
}

// NewGenericExecutor creates a new GenericExecutor.
func NewGenericExecutor(command string, args []string) *GenericExecutor {
    return &GenericExecutor{Command: command, Args: args}
}

// Run executes the command, passing the prompt via stdin.
// The command's combined stdout+stderr is returned as Output.
func (e *GenericExecutor) Run(ctx context.Context, prompt string) Result {
    cmd := exec.CommandContext(ctx, e.Command, e.Args...) //nolint:gosec // user-configured command
    if prompt != "" {
        cmd.Stdin = strings.NewReader(prompt)
    }
    out, err := cmd.CombinedOutput()
    if err != nil {
        return Result{Output: string(out), Error: fmt.Errorf("%s exited: %w", e.Command, err)}
    }
    return Result{Output: string(out)}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./pkg/executor/... -run TestGenericExecutor -v 2>&1 | tail -10
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/executor/generic.go pkg/executor/generic_test.go
git commit -m "feat(executor): GenericExecutor for plain-protocol tools"
```

---

## Task 8: Judge — Two-Pass Model

**Files:**
- Create: `pkg/judge/matrix.go`
- Create: `pkg/judge/judge.go`
- Create: `pkg/judge/judge_test.go`

- [ ] **Step 1: Write failing tests**

`pkg/judge/judge_test.go`:

```go
package judge_test

import (
    "context"
    "testing"

    "github.com/alex-mextner/quorex/pkg/judge"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMatrix_Render(t *testing.T) {
    m := judge.Matrix{
        Providers: []string{"claude", "codex"},
        Scores: map[string]map[string]bool{
            "Architecture": {"claude": true},
            "Correctness":  {"codex": true},
        },
    }
    rendered := m.Render()
    assert.Contains(t, rendered, "claude")
    assert.Contains(t, rendered, "codex")
    assert.Contains(t, rendered, "★")
    assert.Contains(t, rendered, "—")
}

func TestBuildEvalPrompt(t *testing.T) {
    diffs := map[string]string{
        "claude": "diff --git a/foo.go\n+func foo() {}",
        "codex":  "diff --git a/foo.go\n+func foo() { return 1 }",
    }
    prompt := judge.BuildEvalPrompt(diffs, []string{"Correctness", "Code style"})
    assert.Contains(t, prompt, "claude")
    assert.Contains(t, prompt, "codex")
    assert.Contains(t, prompt, "Correctness")
    assert.Contains(t, prompt, "scoring matrix")
}

func TestBuildSynthesisPrompt(t *testing.T) {
    matrix := judge.Matrix{
        Providers: []string{"claude", "codex"},
        Scores:    map[string]map[string]bool{"Correctness": {"codex": true}},
    }
    diffs := map[string]string{
        "claude": "diff claude",
        "codex":  "diff codex",
    }
    task := "implement foo function"
    prompt := judge.BuildSynthesisPrompt(task, matrix, diffs)
    assert.Contains(t, prompt, "matrix")
    assert.Contains(t, prompt, "implement foo")
    assert.Contains(t, prompt, "codex")
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./pkg/judge/... 2>&1 | head -5
```

- [ ] **Step 3: Implement matrix.go**

`pkg/judge/matrix.go`:

```go
// Package judge implements the two-pass judge for quorex.
package judge

import (
    "fmt"
    "strings"
)

// Matrix holds the scoring results from Pass 1 evaluation.
type Matrix struct {
    Providers  []string            // executor names in order
    Categories []string            // evaluated categories
    Scores     map[string]map[string]bool // category → provider → winner
}

// Render returns a human-readable ASCII matrix.
func (m *Matrix) Render() string {
    var b strings.Builder

    // header
    b.WriteString(fmt.Sprintf("%-20s", ""))
    for _, p := range m.Providers {
        b.WriteString(fmt.Sprintf("  %-10s", p))
    }
    b.WriteString("\n")

    // rows
    for _, cat := range m.Categories {
        b.WriteString(fmt.Sprintf("%-20s", cat))
        for _, p := range m.Providers {
            if m.Scores[cat][p] {
                b.WriteString(fmt.Sprintf("  %-10s", "★"))
            } else {
                b.WriteString(fmt.Sprintf("  %-10s", "—"))
            }
        }
        b.WriteString("\n")
    }
    return b.String()
}

// DefaultCategories returns the default evaluation category set.
var DefaultCategories = []string{
    "Architecture",
    "Correctness",
    "Error handling",
    "Code style",
    "Tests",
    "Performance",
    "Security",
    "UX",
    "Accessibility",
    "Documentation",
}
```

- [ ] **Step 4: Implement judge.go**

`pkg/judge/judge.go`:

```go
package judge

import (
    "fmt"
    "strings"
)

// BuildEvalPrompt constructs the Pass 1 evaluation prompt.
// The judge receives all provider diffs and must produce a scoring matrix.
func BuildEvalPrompt(diffs map[string]string, categories []string) string {
    var b strings.Builder
    b.WriteString("You are evaluating code changes from multiple AI providers.\n\n")
    b.WriteString("## Provider Diffs\n\n")
    for provider, diff := range diffs {
        fmt.Fprintf(&b, "### %s\n\n```diff\n%s\n```\n\n", provider, diff)
    }
    b.WriteString("## Task\n\n")
    b.WriteString("Produce a scoring matrix. For each category below, identify which provider ")
    b.WriteString("had the strongest approach. Mark exactly one winner per category (★) or none ")
    b.WriteString("if all approaches are equivalent (—).\n\n")
    b.WriteString("Categories: " + strings.Join(categories, ", ") + "\n\n")
    b.WriteString("Output format:\n")
    b.WriteString("```\n")
    b.WriteString("<category>: <provider_name>\n")
    b.WriteString("```\n")
    b.WriteString("One line per category. Use the exact provider names. ")
    b.WriteString("Output ONLY the matrix lines, nothing else.\n")
    return b.String()
}

// BuildSynthesisPrompt constructs the Pass 2 synthesis prompt.
// This is passed to the judge executor as a ralphex task iteration.
func BuildSynthesisPrompt(taskDescription string, matrix Matrix, diffs map[string]string) string {
    var b strings.Builder
    b.WriteString("You are synthesizing the best solution from multiple AI provider outputs.\n\n")
    b.WriteString("## Original Task\n\n")
    b.WriteString(taskDescription + "\n\n")
    b.WriteString("## Scoring Matrix\n\n")
    b.WriteString("```\n")
    b.WriteString(matrix.Render())
    b.WriteString("```\n\n")
    b.WriteString("## Provider Outputs\n\n")
    for provider, diff := range diffs {
        fmt.Fprintf(&b, "### %s\n\n```diff\n%s\n```\n\n", provider, diff)
    }
    b.WriteString("## Instructions\n\n")
    b.WriteString("Using the matrix as a guide for which provider excelled in each area, ")
    b.WriteString("produce a unified implementation that combines the strongest elements. ")
    b.WriteString("Apply the changes to the working tree. ")
    b.WriteString("Emit <<<QUOREX:COMPLETED>>> when done.\n")
    return b.String()
}

// ParseMatrix parses the judge's Pass 1 output into a Matrix.
func ParseMatrix(output string, providers []string) Matrix {
    scores := make(map[string]map[string]bool)
    var categories []string

    for _, line := range strings.Split(output, "\n") {
        line = strings.TrimSpace(line)
        if line == "" || strings.HasPrefix(line, "```") {
            continue
        }
        parts := strings.SplitN(line, ":", 2)
        if len(parts) != 2 {
            continue
        }
        cat := strings.TrimSpace(parts[0])
        winner := strings.TrimSpace(parts[1])
        if cat == "" || winner == "" || winner == "none" {
            categories = append(categories, cat)
            scores[cat] = map[string]bool{}
            continue
        }
        categories = append(categories, cat)
        scores[cat] = map[string]bool{winner: true}
    }

    return Matrix{
        Providers:  providers,
        Categories: categories,
        Scores:     scores,
    }
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./pkg/judge/... -v 2>&1 | tail -15
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/judge/
git commit -m "feat(judge): two-pass evaluation matrix and synthesis prompt builder"
```

---

## Task 9: Pool Dispatcher

**Files:**
- Create: `pkg/pool/pool.go`
- Create: `pkg/pool/pool_test.go`

This is the core of Quorex. Runs N executors in parallel, each in its own worktree.

- [ ] **Step 1: Write failing tests**

`pkg/pool/pool_test.go`:

```go
package pool_test

import (
    "context"
    "os/exec"
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
        assert.NoError(t, r.Err)
        assert.NotEmpty(t, r.Output)
    }
}

func TestPool_RunParallel_OneFailsRetrySucceeds(t *testing.T) {
    dir := t.TempDir()
    initGitRepo(t, dir)

    // first call fails, retry succeeds — simulate with a counter file
    scriptPath := filepath.Join(dir, "flaky.sh")
    err := os.WriteFile(scriptPath, []byte(`#!/bin/sh
counter_file="$(dirname "$0")/count"
count=0
[ -f "$counter_file" ] && count=$(cat "$counter_file")
count=$((count+1))
echo $count > "$counter_file"
if [ $count -eq 1 ]; then exit 1; fi
echo "success on attempt $count"
`), 0o755)
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
    assert.NoError(t, results[0].Err)
    assert.Contains(t, results[0].Output, "success on attempt 2")
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./pkg/pool/... -run TestPool 2>&1 | head -5
```

- [ ] **Step 3: Implement pool.go**

`pkg/pool/pool.go`:

```go
package pool

import (
    "context"
    "fmt"
    "os/exec"
    "strings"
    "sync"
    "time"

    "github.com/alex-mextner/quorex/pkg/quorexcfg"
)

// Result holds the output of one executor run.
type Result struct {
    ExecutorName string
    Output       string
    Diff         string  // git diff of the worktree vs HEAD
    Worktree     *Worktree
    Err          error
    IsQuotaError bool // true when err is a quota/billing/rate-limit failure
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
// Each executor runs in an isolated git worktree.
// On transient failure, retries up to cfg.ProviderRetries times.
// On quota failure, marks IsQuotaError and skips retries.
func (p *Pool) Run(ctx context.Context, prompt string) ([]Result, error) {
    runID := fmt.Sprintf("%d", time.Now().UnixMilli())

    sem := make(chan struct{}, p.cfg.Parallel)
    resultCh := make(chan Result, len(p.cfg.Executors))

    var wg sync.WaitGroup
    for _, ex := range p.cfg.Executors {
        wg.Add(1)
        ex := ex
        go func() {
            defer wg.Done()
            sem <- struct{}{}
            defer func() { <-sem }()

            result := p.runOne(ctx, ex, runID, prompt)
            resultCh <- result
        }()
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
            break // success
        }
        if isQuotaError(runErr) {
            return Result{
                ExecutorName: ex.Name,
                Worktree:     wt,
                Err:          runErr,
                IsQuotaError: true,
            }
        }
        // transient: retry
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
    args := append(ex.Args, "--print") //nolint:gocritic // intentional append to new slice
    cmd := exec.CommandContext(ctx, ex.Command, args...)  //nolint:gosec
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

// quotaErrorPatterns are known rate-limit/billing error strings.
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
    msg := strings.ToLower(err.Error())
    for _, p := range quotaErrorPatterns {
        if strings.Contains(msg, p) {
            return true
        }
    }
    return false
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./pkg/pool/... -v 2>&1 | tail -20
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/pool/
git commit -m "feat(pool): parallel executor pool with worktree isolation and retry"
```

---

## Task 10: quorex init Command

**Files:**
- Create: `pkg/initcmd/init.go`
- Create: `pkg/initcmd/init_test.go`

- [ ] **Step 1: Write failing test**

`pkg/initcmd/init_test.go`:

```go
package initcmd_test

import (
    "testing"

    "github.com/alex-mextner/quorex/pkg/initcmd"
    "github.com/stretchr/testify/assert"
)

func TestDetectHarness_Claude(t *testing.T) {
    // inject PATH with a fake "claude" binary
    dir := t.TempDir()
    createFakeBin(t, dir, "claude")
    t.Setenv("PATH", dir)

    harness := initcmd.DetectHarness()
    assert.Equal(t, "claude", harness)
}

func TestDetectHarness_FallbackToCodex(t *testing.T) {
    dir := t.TempDir()
    createFakeBin(t, dir, "codex")
    t.Setenv("PATH", dir)

    harness := initcmd.DetectHarness()
    assert.Equal(t, "codex", harness)
}

func TestDetectHarness_None(t *testing.T) {
    t.Setenv("PATH", t.TempDir())
    harness := initcmd.DetectHarness()
    assert.Equal(t, "", harness)
}

func TestInstallMessage(t *testing.T) {
    msg := initcmd.InstallInstructions()
    assert.Contains(t, msg, "claude")
    assert.Contains(t, msg, "codex")
    assert.Contains(t, msg, "opencode")
}

func createFakeBin(t *testing.T, dir, name string) {
    t.Helper()
    path := filepath.Join(dir, name)
    require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\necho ok\n"), 0o755))
}
```

- [ ] **Step 2: Implement init.go**

`pkg/initcmd/init.go`:

```go
// Package initcmd implements the "quorex init" command.
package initcmd

import (
    "os/exec"
)

// priorityHarnesses defines detection order.
var priorityHarnesses = []string{"claude", "codex", "opencode"}

// DetectHarness returns the first available AI harness found in PATH.
// Returns empty string if none found.
func DetectHarness() string {
    for _, h := range priorityHarnesses {
        if _, err := exec.LookPath(h); err == nil {
            return h
        }
    }
    return ""
}

// InstallInstructions returns the install guidance message.
func InstallInstructions() string {
    return `No supported AI harness found. Install one of:
  claude   — https://claude.ai/code
  codex    — https://github.com/openai/codex
  opencode — https://opencode.ai

Then re-run: quorex init`
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./pkg/initcmd/... -v 2>&1 | tail -15
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add pkg/initcmd/
git commit -m "feat(initcmd): harness detection and install instructions for quorex init"
```

---

## Task 11: Wire Everything into cmd/quorex/main.go

**Files:**
- Modify: `cmd/quorex/main.go`

Add new CLI subcommands: `quorex run` (with pool), `quorex plans fix`, `quorex init`.

- [ ] **Step 1: Add `run` subcommand with pool flags**

Add to opts struct in `cmd/quorex/main.go`:

```go
// Quorex-specific pool flags
TaskExecutors   string `long:"task-executors" description:"comma-separated executors or '*' for all"`
TaskParallel    int    `long:"task-parallel" default:"0" description:"parallel executor count (0 = all)"`
ReviewExecutors string `long:"review-executors" description:"executor(s) used as judge"`
QuorexConfig    string `long:"quorex-config" default:"quorex.toml" description:"path to quorex.toml project config"`
```

- [ ] **Step 2: Load quorexcfg before processor.Run**

In the main run path (after existing config load), add:

```go
// load quorex.toml if present
var qcfg *quorexcfg.Config
if _, err := os.Stat(o.QuorexConfig); err == nil {
    qcfg, err = quorexcfg.LoadFile(o.QuorexConfig)
    if err != nil {
        log.Fatalf("[ERROR] load quorex.toml: %v", err)
    }
}
```

- [ ] **Step 3: Add `plans fix` subcommand handler**

```go
case "plans", "fix":
    if len(args) < 2 {
        fmt.Fprintln(os.Stderr, "usage: quorex plans fix <file>")
        os.Exit(1)
    }
    data, err := os.ReadFile(args[1])
    if err != nil {
        log.Fatalf("read plan: %v", err)
    }
    errs := planval.Validate(data)
    if len(errs) == 0 {
        fmt.Println("plan file is valid")
        return
    }
    fmt.Print(planval.FormatErrors(args[1], errs))
    // TODO: invoke agent to fix in Task 12
    os.Exit(1)
```

- [ ] **Step 4: Add `init` subcommand handler**

```go
case "init":
    harness := initcmd.DetectHarness()
    if harness == "" {
        fmt.Println(initcmd.InstallInstructions())
        os.Exit(1)
    }
    fmt.Printf("Using %s to analyze project...\n", harness)
    // TODO: invoke harness to generate quorex.toml in Task 12
```

- [ ] **Step 5: Build**

```bash
go build ./cmd/quorex/
./quorex --version
```

Expected: binary builds and outputs version.

- [ ] **Step 6: Commit**

```bash
git add cmd/quorex/
git commit -m "feat(cmd): wire pool flags, plans fix, init subcommands into main"
```

---

## Task 12: Acceptance Criteria Validation

Run these checks and fix anything that fails:

- [ ] **Build**

```bash
go build ./cmd/quorex/
```

Expected: exit 0.

- [ ] **All unit tests pass**

```bash
go test ./... -race -count=1 2>&1 | tail -20
```

Expected: all PASS (skip tests requiring live AI tools are acceptable).

- [ ] **Lint**

```bash
golangci-lint run ./... 2>&1 | head -30
```

Expected: 0 errors (warnings acceptable).

- [ ] **quorexcfg: round-trip test**

```bash
cat > /tmp/test.toml << 'EOF'
[executor.claude]
args = ["--dangerously-skip-permissions"]
protocol = "claude-json-stream"

[pool]
task_executors = "*"
task_parallel = 2
review_executors = "claude"
timeout = "5m"
provider_retries = 1

[[hook]]
name = "typecheck"
command = "echo"
args = ["ok"]
phase = "pre"
EOF
./quorex --quorex-config /tmp/test.toml --version
```

Expected: binary starts without error.

- [ ] **planval: detect bad plan**

```bash
cat > /tmp/bad.md << 'EOF'
# My Feature

## implement foo

code goes here
EOF
./quorex plans fix /tmp/bad.md
```

Expected: prints error with "## Task: <name>" guidance and "quorex plans fix" suggestion, exit 1.

- [ ] **planval: accept good plan**

```bash
cat > /tmp/good.md << 'EOF'
# My Feature

## Task: implement foo

code goes here
EOF
./quorex plans fix /tmp/good.md
```

Expected: "plan file is valid", exit 0.

- [ ] **init: no harness**

```bash
PATH=/tmp/empty ./quorex init
```

Expected: prints install instructions with claude/codex/opencode URLs, exit 1.

- [ ] **Push to remote**

```bash
git push origin master
```

Expected: all commits pushed.

---

## Acceptance Criteria

| Criterion | How to verify |
|-----------|---------------|
| Module renamed to `github.com/alex-mextner/quorex` | `head -1 go.mod` |
| Binary is `quorex` (not `ralphex`) | `./quorex --version` |
| TOML config loads with all spec fields | `go test ./pkg/quorexcfg/... -v` |
| Worktree created/removed per executor | `go test ./pkg/pool/... -v -run TestWorktree` |
| Pre-hooks abort on failure | `go test ./pkg/hooks/... -v` |
| Post-hooks exclude failing provider | `go test ./pkg/hooks/... -v` |
| Judge eval prompt contains all diffs and categories | `go test ./pkg/judge/... -v` |
| Judge synthesis prompt contains matrix | `go test ./pkg/judge/... -v` |
| Pool runs N executors in parallel goroutines | `go test ./pkg/pool/... -v -run TestPool_RunParallel` |
| Transient failure retried | `go test ./pkg/pool/... -v -run TestPool_RunParallel_OneFailsRetry` |
| Plan format errors have line numbers | `go test ./pkg/planval/... -v` |
| `quorex plans fix` suggests fix command | binary smoke test |
| `quorex init` detects harness priority | `go test ./pkg/initcmd/... -v` |
| 0 race conditions | `go test ./... -race` |
| golangci-lint passes | `golangci-lint run ./...` |
