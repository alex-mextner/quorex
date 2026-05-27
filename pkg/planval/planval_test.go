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

const multipleErrors = "# My Feature\n\n## task foo\n\n```go\nfunc x() {}\n"

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
	assert.Equal(t, 5, errs[0].Line)
}

func TestValidate_MultipleErrors(t *testing.T) {
	errs := planval.Validate([]byte(multipleErrors))
	assert.Len(t, errs, 2) // bad header + unclosed fence
}

func TestValidate_FenceInsideSection_NotFlagged(t *testing.T) {
	// headings inside fences should not be checked
	plan := "# Feature\n\n## Task: foo\n\n```md\n## task bar\n```\n"
	errs := planval.Validate([]byte(plan))
	assert.Empty(t, errs)
}

func TestFormatErrors_Empty(t *testing.T) {
	msg := planval.FormatErrors("plans/test.md", nil)
	assert.Empty(t, msg)
}

func TestFormatErrors_WithErrors(t *testing.T) {
	errs := planval.Validate([]byte(missingTaskHeader))
	msg := planval.FormatErrors("plans/test.md", errs)
	assert.Contains(t, msg, `Error: invalid plan file "plans/test.md"`)
	assert.Contains(t, msg, "Line 3")
	assert.Contains(t, msg, "quorex plans fix plans/test.md")
}
