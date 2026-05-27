package judge_test

import (
	"testing"

	"github.com/alex-mextner/quorex/pkg/judge"
	"github.com/stretchr/testify/assert"
)

func TestMatrix_Render(t *testing.T) {
	m := judge.Matrix{
		Providers:  []string{"claude", "codex"},
		Categories: []string{"Architecture", "Correctness"},
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
		Providers:  []string{"claude", "codex"},
		Categories: []string{"Correctness"},
		Scores:     map[string]map[string]bool{"Correctness": {"codex": true}},
	}
	diffs := map[string]string{
		"claude": "diff claude",
		"codex":  "diff codex",
	}
	prompt := judge.BuildSynthesisPrompt("implement foo function", matrix, diffs)
	assert.Contains(t, prompt, "matrix")
	assert.Contains(t, prompt, "implement foo")
	assert.Contains(t, prompt, "codex")
}

func TestParseMatrix(t *testing.T) {
	output := `Architecture: claude
Correctness: codex
Code style: none
`
	m := judge.ParseMatrix(output, []string{"claude", "codex"})
	assert.Len(t, m.Categories, 3)
	assert.True(t, m.Scores["Architecture"]["claude"])
	assert.True(t, m.Scores["Correctness"]["codex"])
	assert.Empty(t, m.Scores["Code style"])
}
