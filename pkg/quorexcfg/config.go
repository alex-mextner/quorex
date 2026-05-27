// Package quorexcfg provides TOML project configuration for quorex (quorex.toml).
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

// builtinCommands maps built-in executor names to their default commands.
// Built-ins exist implicitly; users overlay them via [executor.<name>].
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
	TaskExecutorsRaw    string        `toml:"task_executors"`
	TaskParallel        int           `toml:"task_parallel"`
	ReviewExecutors     []string      `toml:"-"`
	ReviewExecutorsRaw  string        `toml:"review_executors"`
	Timeout             time.Duration `toml:"-"`
	TimeoutRaw          string        `toml:"timeout"`
	ProviderRetries     int           `toml:"provider_retries"`
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

// rawConfig matches the TOML file structure exactly.
type rawConfig struct {
	Executor map[string]rawExecutorDef `toml:"executor"`
	Pool     PoolConfig                `toml:"pool"`
	Judge    JudgeConfig               `toml:"judge"`
	Hook     []HookDef                 `toml:"hook"`
}

// rawExecutorDef includes an explicit enabled field to distinguish false from unset.
type rawExecutorDef struct {
	Command        string   `toml:"command"`
	Args           []string `toml:"args"`
	Role           Role     `toml:"role"`
	Mode           Mode     `toml:"mode"`
	Protocol       string   `toml:"protocol"`
	Enabled        *bool    `toml:"enabled"` // pointer to detect explicit false
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
	data, err := os.ReadFile(path) //nolint:gosec // user-provided config path
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

	// seed built-ins with defaults
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

	// user-defined executors not in built-ins
	for name, def := range raw.Executor {
		if _, isBuiltin := builtinCommands[name]; isBuiltin {
			continue
		}
		enabled := true
		if def.Enabled != nil {
			enabled = *def.Enabled
		}
		role := def.Role
		if role == "" {
			role = RoleTask
		}
		mode := def.Mode
		if mode == "" {
			mode = ModeEdit
		}
		cfg.Executors[name] = &ExecutorDef{
			Name:     name,
			Command:  def.Command,
			Args:     def.Args,
			Role:     role,
			Mode:     mode,
			Protocol: def.Protocol,
			Enabled:  enabled,
		}
	}

	// parse pool timeout
	if raw.Pool.TimeoutRaw != "" {
		dur, err := time.ParseDuration(raw.Pool.TimeoutRaw)
		if err != nil {
			return nil, fmt.Errorf("pool.timeout %q: %w", raw.Pool.TimeoutRaw, err)
		}
		cfg.Pool.Timeout = dur
	}
	if cfg.Pool.Timeout == 0 {
		cfg.Pool.Timeout = 10 * time.Minute
	}

	// parse review executors
	cfg.Pool.ReviewExecutors = splitComma(raw.Pool.ReviewExecutorsRaw)

	// default hook phase to pre
	for i := range cfg.Hooks {
		if cfg.Hooks[i].Phase == "" {
			cfg.Hooks[i].Phase = PhasePre
		}
	}

	return cfg, nil
}

// ResolveTaskExecutors returns enabled task-role executors.
// If task_executors = "*" or empty, returns all enabled task-role executors.
// Otherwise filters the named list.
func (c *Config) ResolveTaskExecutors() []*ExecutorDef {
	raw := c.Pool.TaskExecutorsRaw

	if raw == "" || raw == "*" {
		var result []*ExecutorDef
		for _, e := range c.Executors {
			if e.Enabled && e.Role == RoleTask {
				result = append(result, e)
			}
		}
		return result
	}

	var result []*ExecutorDef
	for _, name := range splitComma(raw) {
		if e, ok := c.Executors[name]; ok && e.Enabled {
			result = append(result, e)
		}
	}
	return result
}

// ResolveReviewExecutor returns the first available review executor, falling back to the claude built-in.
func (c *Config) ResolveReviewExecutor() *ExecutorDef {
	for _, name := range c.Pool.ReviewExecutors {
		if e, ok := c.Executors[name]; ok && e.Enabled {
			return e
		}
	}
	if e, ok := c.Executors["claude"]; ok {
		return e
	}
	return &ExecutorDef{Name: "claude", Command: "claude"}
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

func applyOverlay(base *ExecutorDef, overlay rawExecutorDef) {
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
	if overlay.Enabled != nil {
		base.Enabled = *overlay.Enabled
	}
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
