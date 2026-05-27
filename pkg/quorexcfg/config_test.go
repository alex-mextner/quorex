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

func TestParse_MinimalPoolConfig(t *testing.T) {
	data := []byte(`
[pool]
task_executors   = "claude,codex"
task_parallel    = 2
review_executors = "claude"
timeout          = "10m"
provider_retries = 1
`)
	cfg, err := quorexcfg.Parse(data)
	require.NoError(t, err)
	assert.Equal(t, 2, cfg.Pool.TaskParallel)
	assert.Equal(t, []string{"claude"}, cfg.Pool.ReviewExecutors)
	assert.Equal(t, 10*time.Minute, cfg.Pool.Timeout)
	assert.Equal(t, 1, cfg.Pool.ProviderRetries)
}

func TestParse_BuiltinExecutorDefaults(t *testing.T) {
	data := []byte(``) // empty config — built-ins always present
	cfg, err := quorexcfg.Parse(data)
	require.NoError(t, err)

	claude := cfg.Executors["claude"]
	require.NotNil(t, claude)
	assert.Equal(t, "claude", claude.Command)
	assert.Equal(t, quorexcfg.RoleTask, claude.Role)
	assert.Equal(t, quorexcfg.ModeEdit, claude.Mode)
	assert.True(t, claude.Enabled)

	codex := cfg.Executors["codex"]
	require.NotNil(t, codex)
	assert.Equal(t, "codex", codex.Command)
	assert.True(t, codex.Enabled)
}

func TestParse_BuiltinOverlay(t *testing.T) {
	data := []byte(`
[executor.claude]
args     = ["--output-format", "stream-json", "--dangerously-skip-permissions"]
protocol = "claude-json-stream"
`)
	cfg, err := quorexcfg.Parse(data)
	require.NoError(t, err)

	claude := cfg.Executors["claude"]
	require.NotNil(t, claude)
	assert.Equal(t, "claude", claude.Command) // inferred, not overridden
	assert.Equal(t, []string{"--output-format", "stream-json", "--dangerously-skip-permissions"}, claude.Args)
	assert.Equal(t, "claude-json-stream", claude.Protocol)
}

func TestParse_UserDefinedExecutor(t *testing.T) {
	data := []byte(`
[executor.gemini]
command  = "gemini"
args     = ["--yolo"]
protocol = "plain"
`)
	cfg, err := quorexcfg.Parse(data)
	require.NoError(t, err)

	gemini := cfg.Executors["gemini"]
	require.NotNil(t, gemini)
	assert.Equal(t, "gemini", gemini.Command)
	assert.Equal(t, []string{"--yolo"}, gemini.Args)
	assert.Equal(t, "plain", gemini.Protocol)
	assert.Equal(t, quorexcfg.RoleTask, gemini.Role)
	assert.Equal(t, quorexcfg.ModeEdit, gemini.Mode)
	assert.True(t, gemini.Enabled)
}

func TestParse_DisabledExecutor(t *testing.T) {
	data := []byte(`
[executor.disabled-tool]
command = "sometool"
enabled = false
`)
	cfg, err := quorexcfg.Parse(data)
	require.NoError(t, err)
	assert.False(t, cfg.Executors["disabled-tool"].Enabled)
}

func TestParse_Hooks(t *testing.T) {
	data := []byte(`
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
`)
	cfg, err := quorexcfg.Parse(data)
	require.NoError(t, err)
	require.Len(t, cfg.Hooks, 2)
	assert.Equal(t, "typecheck", cfg.Hooks[0].Name)
	assert.Equal(t, quorexcfg.PhasePre, cfg.Hooks[0].Phase)
	assert.Equal(t, "tests", cfg.Hooks[1].Name)
	assert.Equal(t, quorexcfg.PhasePost, cfg.Hooks[1].Phase)
}

func TestParse_HookDefaultPhasePre(t *testing.T) {
	data := []byte(`
[[hook]]
name    = "lint"
command = "bun"
args    = ["run", "lint"]
`)
	cfg, err := quorexcfg.Parse(data)
	require.NoError(t, err)
	assert.Equal(t, quorexcfg.PhasePre, cfg.Hooks[0].Phase)
}

func TestResolveTaskExecutors_Wildcard(t *testing.T) {
	data := []byte(`
[executor.claude]
args = ["--dangerously-skip-permissions"]

[executor.codex]
args = ["--full-auto"]

[executor.disabled-tool]
command = "sometool"
enabled = false

[pool]
task_executors = "*"
`)
	cfg, err := quorexcfg.Parse(data)
	require.NoError(t, err)

	resolved := cfg.ResolveTaskExecutors()
	assert.Len(t, resolved, 2) // disabled excluded
	names := make([]string, len(resolved))
	for i, e := range resolved {
		names[i] = e.Name
	}
	assert.ElementsMatch(t, []string{"claude", "codex"}, names)
}

func TestResolveTaskExecutors_NamedList(t *testing.T) {
	data := []byte(`
[pool]
task_executors = "claude,codex"
`)
	cfg, err := quorexcfg.Parse(data)
	require.NoError(t, err)

	resolved := cfg.ResolveTaskExecutors()
	require.Len(t, resolved, 2)
	assert.Equal(t, "claude", resolved[0].Name)
	assert.Equal(t, "codex", resolved[1].Name)
}

func TestHooksForPhase(t *testing.T) {
	data := []byte(`
[[hook]]
name    = "pre-hook"
command = "echo"
args    = ["pre"]
phase   = "pre"

[[hook]]
name    = "post-hook"
command = "echo"
args    = ["post"]
phase   = "post"
`)
	cfg, err := quorexcfg.Parse(data)
	require.NoError(t, err)

	pre := cfg.HooksForPhase(quorexcfg.PhasePre)
	require.Len(t, pre, 1)
	assert.Equal(t, "pre-hook", pre[0].Name)

	post := cfg.HooksForPhase(quorexcfg.PhasePost)
	require.Len(t, post, 1)
	assert.Equal(t, "post-hook", post[0].Name)
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "quorex.toml")
	err := os.WriteFile(f, []byte(`
[pool]
task_executors   = "*"
task_parallel    = 3
review_executors = "claude"
timeout          = "5m"
`), 0o600)
	require.NoError(t, err)

	cfg, err := quorexcfg.LoadFile(f)
	require.NoError(t, err)
	assert.Equal(t, 3, cfg.Pool.TaskParallel)
	assert.Equal(t, 5*time.Minute, cfg.Pool.Timeout)
}

func TestParse_InvalidTimeout(t *testing.T) {
	data := []byte(`
[pool]
timeout = "notaduration"
`)
	_, err := quorexcfg.Parse(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pool.timeout")
}
