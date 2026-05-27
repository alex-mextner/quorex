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
	r := hooks.NewRunner(defs, t.TempDir())
	err := r.RunPre(context.Background())
	require.NoError(t, err)
}

func TestRunner_PreHooks_Failure(t *testing.T) {
	defs := []quorexcfg.HookDef{
		{Name: "fail", Command: "false", Phase: quorexcfg.PhasePre},
	}
	r := hooks.NewRunner(defs, t.TempDir())
	err := r.RunPre(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fail")
}

func TestRunner_PreHooks_StopsOnFirstFailure(t *testing.T) {
	defs := []quorexcfg.HookDef{
		{Name: "fail", Command: "false", Phase: quorexcfg.PhasePre},
		{Name: "should-not-run", Command: "true", Phase: quorexcfg.PhasePre},
	}
	r := hooks.NewRunner(defs, t.TempDir())
	err := r.RunPre(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"fail"`)
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

func TestRunner_PhaseFiltering(t *testing.T) {
	// pre-only runner should not run post hooks and vice versa
	defs := []quorexcfg.HookDef{
		{Name: "pre-fail", Command: "false", Phase: quorexcfg.PhasePre},
		{Name: "post-ok", Command: "true", Phase: quorexcfg.PhasePost},
	}
	r := hooks.NewRunner(defs, t.TempDir())

	// RunPost should not run the pre-fail hook → no error
	err := r.RunPost(context.Background(), t.TempDir())
	require.NoError(t, err)

	// RunPre should run pre-fail → error
	err = r.RunPre(context.Background())
	require.Error(t, err)
}

func TestRunner_Empty(t *testing.T) {
	r := hooks.NewRunner(nil, t.TempDir())
	assert.NoError(t, r.RunPre(context.Background()))
	assert.NoError(t, r.RunPost(context.Background(), t.TempDir()))
}
