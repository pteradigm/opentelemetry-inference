// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsinferenceprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatternEvaluator_Evaluate(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		rule        *internalRule
		outputName  string
		expected    string
		expectError bool
	}{
		{
			name:    "simple output replacement",
			pattern: "ml.{output}",
			rule: &internalRule{
				modelName: "test-model",
			},
			outputName: "prediction",
			expected:   "ml.prediction",
		},
		{
			name:    "model and output replacement",
			pattern: "ml.{model}.{output}",
			rule: &internalRule{
				modelName: "cpu-predictor",
			},
			outputName: "prediction",
			expected:   "ml.cpu-predictor.prediction",
		},
		{
			name:    "version replacement",
			pattern: "{model}.{version}.{output}",
			rule: &internalRule{
				modelName:    "predictor",
				modelVersion: "v1",
			},
			outputName: "score",
			expected:   "predictor.v1.score",
		},
		{
			name:    "empty version",
			pattern: "{model}.{version}.{output}",
			rule: &internalRule{
				modelName:    "predictor",
				modelVersion: "", // empty version
			},
			outputName: "score",
			expected:   "predictor..score", // double dot when version is empty
		},
		{
			name:    "input replacement",
			pattern: "{input}.{output}",
			rule: &internalRule{
				inputs: []string{"system.cpu.utilization"},
			},
			outputName: "prediction",
			expected:   "system.cpu.utilization.prediction",
		},
		{
			name:    "input array replacement",
			pattern: "{input[0]}.enhanced.{output}",
			rule: &internalRule{
				inputs: []string{"cpu.usage", "memory.usage"},
			},
			outputName: "value",
			expected:   "cpu.usage.enhanced.value",
		},
		{
			name:    "multiple input replacement",
			pattern: "{input[0]}_{input[1]}.{output}",
			rule: &internalRule{
				inputs: []string{"cpu", "memory"},
			},
			outputName: "correlation",
			expected:   "cpu_memory.correlation",
		},
		{
			name:    "out of bounds input index",
			pattern: "{input[5]}.{output}",
			rule: &internalRule{
				inputs: []string{"cpu.usage"},
			},
			outputName: "prediction",
			expected:   "cpu.usage.prediction", // Falls back to first input
		},
		{
			name:    "complex pattern",
			pattern: "metrics.{model}.{input[0]}.{output}",
			rule: &internalRule{
				modelName: "analyzer",
				inputs:    []string{"app.requests"},
			},
			outputName: "rate",
			expected:   "metrics.analyzer.app.requests.rate",
		},
		{
			name:    "undefined variable",
			pattern: "{undefined}.{output}",
			rule: &internalRule{
				modelName: "test",
			},
			outputName:  "result",
			expectError: true,
		},
		{
			name:       "partial replacement",
			pattern:    "prefix_{output}_suffix",
			rule:       &internalRule{},
			outputName: "middle",
			expected:   "prefix_middle_suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := NewPatternEvaluator(tt.pattern, tt.rule)
			result, err := evaluator.Evaluate(tt.outputName)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestValidateOutputPattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		expectError bool
		errorMsg    string
	}{
		{
			name:    "empty pattern is valid",
			pattern: "",
		},
		{
			name:    "simple valid pattern",
			pattern: "{model}.{output}",
		},
		{
			name:    "complex valid pattern",
			pattern: "ml.{model}.{input[0]}.{version}.{output}",
		},
		{
			name:        "unbalanced braces - missing close",
			pattern:     "{model.{output}",
			expectError: true,
			errorMsg:    "unbalanced braces",
		},
		{
			name:        "unbalanced braces - missing open",
			pattern:     "model}.{output}",
			expectError: true,
			errorMsg:    "unbalanced braces",
		},
		{
			name:        "invalid variable",
			pattern:     "{invalid_var}.{output}",
			expectError: true,
			errorMsg:    "invalid variable: invalid_var",
		},
		{
			name:    "valid input array",
			pattern: "{input[0]}_{input[1]}_{input[2]}.{output}",
		},
		{
			name:        "invalid input array syntax",
			pattern:     "{input[abc]}.{output}",
			expectError: true,
			errorMsg:    "invalid variable: input[abc]",
		},
		{
			name:        "nested braces",
			pattern:     "{model.{version}}.{output}",
			expectError: true,
			errorMsg:    "invalid variable: model.{version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOutputPattern(tt.pattern)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultDecorateOutputName(t *testing.T) {
	mp := &metricsinferenceprocessor{
		config: &Config{
			Naming: NamingConfig{}, // Empty config to trigger default behavior
		},
	}

	tests := []struct {
		name       string
		rule       *internalRule
		outputName string
		expected   string
	}{
		{
			name: "single input with 2 parts",
			rule: &internalRule{
				inputs: []string{"cpu.usage"},
			},
			outputName: "prediction",
			expected:   "cpu_usage.prediction",
		},
		{
			name: "single input with 3 parts",
			rule: &internalRule{
				inputs: []string{"system.cpu.utilization"},
			},
			outputName: "prediction",
			expected:   "cpu_utilization.prediction",
		},
		{
			name: "single input with 4 parts",
			rule: &internalRule{
				inputs: []string{"app.service.api.latency"},
			},
			outputName: "p95",
			expected:   "api_latency.p95",
		},
		{
			name: "multiple inputs - 2 parts each",
			rule: &internalRule{
				inputs: []string{"cpu.usage", "memory.usage"},
			},
			outputName: "correlation",
			expected:   "cpu_usage_memory_usage.correlation",
		},
		{
			name: "multiple inputs - mixed parts",
			rule: &internalRule{
				inputs: []string{"system.cpu.utilization", "memory.usage", "app.api.requests"},
			},
			outputName: "anomaly",
			expected:   "cpu_utilization_memory_usage_api_requests.anomaly",
		},
		{
			name: "no inputs - fallback to model name",
			rule: &internalRule{
				modelName: "predictor",
				inputs:    []string{},
			},
			outputName: "result",
			expected:   "predictor.result",
		},
		{
			name: "no inputs or model - final fallback",
			rule: &internalRule{
				inputs: []string{},
			},
			outputName: "result",
			expected:   "result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mp.defaultDecorateOutputName(tt.rule, tt.outputName, 0)
			assert.Equal(t, tt.expected, result)
		})
	}
}
