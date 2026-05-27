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
