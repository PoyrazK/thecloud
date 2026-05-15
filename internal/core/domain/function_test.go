package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFunctionUpdateValidate(t *testing.T) {
	t.Run("timeout_too_low", func(t *testing.T) {
		timeout := 0
		err := (&FunctionUpdate{Timeout: &timeout}).Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "timeout")
	})

	t.Run("timeout_too_high", func(t *testing.T) {
		timeout := 901
		err := (&FunctionUpdate{Timeout: &timeout}).Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "timeout")
	})

	t.Run("memory_too_low", func(t *testing.T) {
		mem := 32
		err := (&FunctionUpdate{MemoryMB: &mem}).Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "memory")
	})

	t.Run("memory_too_high", func(t *testing.T) {
		mem := 10241
		err := (&FunctionUpdate{MemoryMB: &mem}).Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "memory")
	})

	t.Run("cpu_too_low", func(t *testing.T) {
		cpu := 0.05
		err := (&FunctionUpdate{CPUs: &cpu}).Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cpu")
	})

	t.Run("cpu_too_high", func(t *testing.T) {
		cpu := 9.0
		err := (&FunctionUpdate{CPUs: &cpu}).Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cpu")
	})

	t.Run("max_concurrent_invocation_too_low", func(t *testing.T) {
		mci := -1
		err := (&FunctionUpdate{MaxConcurrentInvocations: &mci}).Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max_concurrent_invocations")
	})

	t.Run("max_concurrent_invocation_too_high", func(t *testing.T) {
		mci := 1001
		err := (&FunctionUpdate{MaxConcurrentInvocations: &mci}).Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max_concurrent_invocations")
	})

	t.Run("max_queue_depth_too_low", func(t *testing.T) {
		mqd := -1
		err := (&FunctionUpdate{MaxQueueDepth: &mqd}).Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max_queue_depth")
	})

	t.Run("valid_max_concurrent_invocation_within_bounds", func(t *testing.T) {
		mci := 500
		err := (&FunctionUpdate{MaxConcurrentInvocations: &mci}).Validate()
		assert.NoError(t, err)
	})

	t.Run("valid_timeout_memory_cpu", func(t *testing.T) {
		timeout := 300
		mem := 256
		cpu := 2.0
		err := (&FunctionUpdate{Timeout: &timeout, MemoryMB: &mem, CPUs: &cpu}).Validate()
		assert.NoError(t, err)
	})

	t.Run("env_var_both_value_and_secret_ref", func(t *testing.T) {
		u := &FunctionUpdate{
			EnvVars: []*EnvVar{{Key: "API_KEY", Value: "plain", SecretRef: "@secret"}},
		}
		err := u.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot have both")
	})

	t.Run("env_var_neither_value_nor_secret_ref", func(t *testing.T) {
		u := &FunctionUpdate{
			EnvVars: []*EnvVar{{Key: "API_KEY"}},
		}
		err := u.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "either value or secret_ref")
	})

	t.Run("env_var_only_value", func(t *testing.T) {
		u := &FunctionUpdate{
			EnvVars: []*EnvVar{{Key: "FOO", Value: "bar"}},
		}
		err := u.Validate()
		assert.NoError(t, err)
	})

	t.Run("env_var_only_secret_ref", func(t *testing.T) {
		u := &FunctionUpdate{
			EnvVars: []*EnvVar{{Key: "API_KEY", SecretRef: "@my-secret"}},
		}
		err := u.Validate()
		assert.NoError(t, err)
	})

	t.Run("env_var_mixed_one_invalid", func(t *testing.T) {
		u := &FunctionUpdate{
			EnvVars: []*EnvVar{
				{Key: "FOO", Value: "bar"},
				{Key: "BAR"}, // missing both value and secret_ref
			},
		}
		err := u.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "either value or secret_ref")
	})
}
