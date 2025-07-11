// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsinferenceprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"

	"github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/internal/testutil"
	pb "github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/proto/v2"
)

func TestMetadataDiscovery(t *testing.T) {
	tests := []struct {
		name            string
		config          *Config
		modelMetadata   map[string]*pb.ModelMetadataResponse
		expectedOutputs map[string][]string // model -> output names
	}{
		{
			name: "discover_all_outputs_no_config",
			config: &Config{
				GRPCClientSettings: GRPCClientSettings{
					Endpoint: "localhost:8081",
				},
				Rules: []Rule{
					{
						ModelName: "simple-scaler",
						Inputs:    []string{"cpu.utilization"},
						// No outputs configured - should discover from metadata
					},
				},
			},
			modelMetadata: map[string]*pb.ModelMetadataResponse{
				"simple-scaler": {
					Name: "simple-scaler",
					Outputs: []*pb.ModelMetadataResponse_TensorMetadata{
						{
							Name:     "scaled_value",
							Datatype: "FP64",
							Shape:    []int64{1},
						},
						{
							Name:     "confidence",
							Datatype: "FP32",
							Shape:    []int64{1},
						},
					},
				},
			},
			expectedOutputs: map[string][]string{
				"simple-scaler": {"cpu_utilization.scaled_value", "cpu_utilization.confidence"},
			},
		},
		{
			name: "multiple_models_with_discovery",
			config: &Config{
				GRPCClientSettings: GRPCClientSettings{
					Endpoint: "localhost:8081",
				},
				Rules: []Rule{
					{
						ModelName: "model-a",
						Inputs:    []string{"input1"},
						// No outputs configured
					},
					{
						ModelName: "model-b",
						Inputs:    []string{"input2"},
						// No outputs configured
					},
				},
			},
			modelMetadata: map[string]*pb.ModelMetadataResponse{
				"model-a": {
					Name: "model-a",
					Outputs: []*pb.ModelMetadataResponse_TensorMetadata{
						{
							Name:     "prediction_a",
							Datatype: "FP64",
							Shape:    []int64{1},
						},
					},
				},
				"model-b": {
					Name: "model-b",
					Outputs: []*pb.ModelMetadataResponse_TensorMetadata{
						{
							Name:     "prediction_b",
							Datatype: "INT64",
							Shape:    []int64{1},
						},
					},
				},
			},
			expectedOutputs: map[string][]string{
				"model-a": {"input1.prediction_a"},
				"model-b": {"input2.prediction_b"},
			},
		},
		{
			name: "no_metadata_available",
			config: &Config{
				GRPCClientSettings: GRPCClientSettings{
					Endpoint: "localhost:8081",
				},
				Rules: []Rule{
					{
						ModelName: "unknown-model",
						Inputs:    []string{"input1"},
						// No outputs configured and no metadata available
					},
				},
			},
			modelMetadata: map[string]*pb.ModelMetadataResponse{},
			expectedOutputs: map[string][]string{
				"unknown-model": {}, // No outputs discovered
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			mockServer := testutil.NewMockInferenceServer()
			mockServer.Start(t)
			defer mockServer.Stop()

			// Configure mock metadata responses
			for modelName, metadata := range tt.modelMetadata {
				mockServer.SetModelMetadata(modelName, metadata)
			}

			// Update config to use mock server endpoint
			tt.config.GRPCClientSettings.Endpoint = mockServer.Endpoint()

			// Create processor
			processor, err := newMetricsProcessor(
				tt.config,
				consumertest.NewNop(),
				zap.NewNop(),
			)
			require.NoError(t, err)

			// Start processor (triggers metadata discovery)
			err = processor.Start(context.Background(), nil)
			require.NoError(t, err)
			defer processor.Shutdown(context.Background())

			// Verify discovered outputs
			for modelName, expectedOutputNames := range tt.expectedOutputs {
				// Find the rule for this model
				var rule *internalRule
				for i := range processor.rules {
					if processor.rules[i].modelName == modelName {
						rule = &processor.rules[i]
						break
					}
				}
				require.NotNil(t, rule, "Rule not found for model: %s", modelName)

				// Check number of outputs
				assert.Equal(t, len(expectedOutputNames), len(rule.outputs),
					"Model %s: expected %d outputs, got %d",
					modelName, len(expectedOutputNames), len(rule.outputs))

				// Check output names
				for i, expectedName := range expectedOutputNames {
					if i < len(rule.outputs) {
						assert.Equal(t, expectedName, rule.outputs[i].name,
							"Model %s output %d: expected name %s, got %s",
							modelName, i, expectedName, rule.outputs[i].name)
						assert.True(t, rule.outputs[i].discovered,
							"Model %s output %d should be marked as discovered",
							modelName, i)
					}
				}
			}
		})
	}
}

func TestMetadataDataTypeConversion(t *testing.T) {
	tests := []struct {
		kserveType   string
		expectedType string
	}{
		{"FP32", "float"},
		{"FP64", "float"},
		{"INT8", "int"},
		{"INT16", "int"},
		{"INT32", "int"},
		{"INT64", "int"},
		{"BOOL", "bool"},
		{"BYTES", "string"},
		{"UNKNOWN", "float"}, // default
	}

	for _, tt := range tests {
		t.Run(tt.kserveType, func(t *testing.T) {
			result := convertKServeDataType(tt.kserveType)
			assert.Equal(t, tt.expectedType, result)
		})
	}
}

func TestMetadataDiscoveryWithParameters(t *testing.T) {
	// Create mock server
	mockServer := testutil.NewMockInferenceServer()
	mockServer.Start(t)
	defer mockServer.Stop()

	// Configure metadata response
	mockServer.SetModelMetadata("parameterized-model", &pb.ModelMetadataResponse{
		Name: "parameterized-model",
		Outputs: []*pb.ModelMetadataResponse_TensorMetadata{
			{
				Name:     "output_tensor",
				Datatype: "FP64",
				Shape:    []int64{1},
			},
		},
	})

	config := &Config{
		GRPCClientSettings: GRPCClientSettings{
			Endpoint: mockServer.Endpoint(),
		},
		Rules: []Rule{
			{
				ModelName: "parameterized-model",
				Inputs:    []string{"input1"},
				// No outputs configured - rely on discovery
				Parameters: map[string]interface{}{
					"threshold": 0.5,
					"mode":      "fast",
				},
			},
		},
	}

	processor, err := newMetricsProcessor(config, consumertest.NewNop(), zap.NewNop())
	require.NoError(t, err)

	err = processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())

	// Verify the rule has discovered outputs
	assert.Equal(t, 1, len(processor.rules))
	rule := processor.rules[0]
	assert.Equal(t, 1, len(rule.outputs))
	assert.Equal(t, "input1.output_tensor", rule.outputs[0].name)
	assert.Equal(t, "float", rule.outputs[0].dataType)
	assert.True(t, rule.outputs[0].discovered)

	// Verify parameters are preserved
	assert.Equal(t, 2, len(rule.parameters))
	assert.Equal(t, 0.5, rule.parameters["threshold"])
	assert.Equal(t, "fast", rule.parameters["mode"])
}

func TestOutputNameDecoration(t *testing.T) {
	// Create a processor instance to test the decoration method
	cfg := &Config{
		GRPCClientSettings: GRPCClientSettings{
			Endpoint: "localhost:8081",
		},
	}
	processor, err := newMetricsProcessor(cfg, consumertest.NewNop(), zap.NewNop())
	require.NoError(t, err)

	tests := []struct {
		name           string
		rule           internalRule
		outputName     string
		outputIndex    int
		expectedResult string
	}{
		{
			name: "single_input_decoration",
			rule: internalRule{
				modelName: "test-model",
				inputs:    []string{"system.cpu.utilization"},
			},
			outputName:     "scaled_result",
			outputIndex:    0,
			expectedResult: "cpu_utilization.scaled_result",
		},
		{
			name: "multiple_inputs_decoration",
			rule: internalRule{
				modelName: "test-model",
				inputs:    []string{"cpu.usage", "memory.usage"},
			},
			outputName:     "anomaly_score",
			outputIndex:    0,
			expectedResult: "cpu_usage_memory_usage.anomaly_score",
		},
		{
			name: "no_inputs_fallback_to_model",
			rule: internalRule{
				modelName: "test-model",
				inputs:    []string{},
			},
			outputName:     "prediction",
			outputIndex:    1,
			expectedResult: "test-model.prediction",
		},
		{
			name: "no_inputs_no_model_fallback",
			rule: internalRule{
				modelName: "",
				inputs:    []string{},
			},
			outputName:     "output",
			outputIndex:    2,
			expectedResult: "output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.decorateOutputName(&tt.rule, tt.outputName, tt.outputIndex)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
