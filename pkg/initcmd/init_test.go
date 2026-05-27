package initcmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alex-mextner/quorex/pkg/initcmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectHarness_Claude(t *testing.T) {
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

func TestInstallInstructions(t *testing.T) {
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
