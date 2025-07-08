// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsinferenceprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/internal/testutil"
	pb "github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/proto/v2"
)

func TestProcessorNamingStrategies(t *testing.T) {
	tests := []struct {
		name           string
		rules          []Rule
		mockResponses  map[string]*pb.ModelInferResponse
		expectedNames  map[string]bool // Map of expected metric names
		description    string
	}{
		// Tests using intelligent naming (no output pattern)
		{
			name: "intelligent_naming_single_input",
			rules: []Rule{
				{
					ModelName: "predictor",
					Inputs:    []string{"cpu.usage"},
					Outputs: []OutputSpec{
						{Name: "prediction"},
					},
				},
			},
			mockResponses: map[string]*pb.ModelInferResponse{
				"predictor": testutil.CreateMockResponseForScaling("predictor", 1.0, 75.0),
			},
			expectedNames: map[string]bool{
				"cpu.usage":            true,
				"cpu_usage.prediction": true, // Intelligent naming
			},
			description: "Single input should produce intelligently named output",
		},
		{
			name: "intelligent_naming_multiple_inputs",
			rules: []Rule{
				{
					ModelName: "anomaly_detector",
					Inputs:    []string{"cpu.usage", "memory.usage"},
					Outputs: []OutputSpec{
						{Name: "anomaly_score"},
					},
				},
			},
			mockResponses: map[string]*pb.ModelInferResponse{
				"anomaly_detector": testutil.CreateMockResponseForScaling("anomaly_detector", 1.0, 0.15),
			},
			expectedNames: map[string]bool{
				"cpu.usage":                            true,
				"memory.usage":                         true,
				"cpu_usage_memory_usage.anomaly_score": true, // Intelligent naming
			},
			description: "Multiple inputs should produce combined intelligent naming",
		},
		{
			name: "intelligent_naming_with_dots",
			rules: []Rule{
				{
					ModelName: "scaler",
					Inputs:    []string{"system.cpu.utilization"},
					Outputs: []OutputSpec{
						{Name: "scaled"},
					},
				},
			},
			mockResponses: map[string]*pb.ModelInferResponse{
				"scaler": testutil.CreateMockResponseForScaling("scaler", 2.0, 50.0),
			},
			expectedNames: map[string]bool{
				"system.cpu.utilization":    true,
				"cpu_utilization.scaled": true, // Intelligent naming shortens the input
			},
			description: "Dots in input names should be converted to underscores",
		},
		// Tests using explicit output patterns
		{
			name: "output_pattern_exact_name",
			rules: []Rule{
				{
					ModelName:     "predictor",
					Inputs:        []string{"cpu.usage"},
					OutputPattern: "{output}",
					Outputs: []OutputSpec{
						{Name: "cpu.predicted"},
					},
				},
			},
			mockResponses: map[string]*pb.ModelInferResponse{
				"predictor": testutil.CreateMockResponseForScaling("predictor", 1.0, 75.0),
			},
			expectedNames: map[string]bool{
				"cpu.usage":     true,
				"cpu.predicted": true, // Exact name from output pattern
			},
			description: "Output pattern {output} should use exact configured name",
		},
		{
			name: "output_pattern_with_model",
			rules: []Rule{
				{
					ModelName:     "anomaly_v2",
					Inputs:        []string{"cpu.usage", "memory.usage"},
					OutputPattern: "{model}.{output}",
					Outputs: []OutputSpec{
						{Name: "score"},
					},
				},
			},
			mockResponses: map[string]*pb.ModelInferResponse{
				"anomaly_v2": testutil.CreateMockResponseForScaling("anomaly_v2", 1.0, 0.25),
			},
			expectedNames: map[string]bool{
				"cpu.usage":       true,
				"memory.usage":    true,
				"anomaly_v2.score": true, // Pattern with model name
			},
			description: "Output pattern with {model} placeholder",
		},
		{
			name: "output_pattern_with_input",
			rules: []Rule{
				{
					ModelName:     "scaler",
					Inputs:        []string{"network.throughput"},
					OutputPattern: "{input}.scaled_by_model",
					Outputs: []OutputSpec{
						{Name: "result"},
					},
				},
			},
			mockResponses: map[string]*pb.ModelInferResponse{
				"scaler": testutil.CreateMockResponseForScaling("scaler", 10.0, 1000.0),
			},
			expectedNames: map[string]bool{
				"network.throughput":                    true,
				"network.throughput.scaled_by_model": true, // Pattern with input name
			},
			description: "Output pattern with {input} placeholder",
		},
		// Mixed scenario - multiple rules with different strategies
		{
			name: "mixed_naming_strategies",
			rules: []Rule{
				{
					ModelName: "predictor",
					Inputs:    []string{"cpu.usage"},
					// No output pattern - use intelligent naming
					Outputs: []OutputSpec{
						{Name: "prediction"},
					},
				},
				{
					ModelName:     "scaler",
					Inputs:        []string{"memory.usage"},
					OutputPattern: "scaled.{input}",
					Outputs: []OutputSpec{
						{Name: "value"},
					},
				},
			},
			mockResponses: map[string]*pb.ModelInferResponse{
				"predictor": testutil.CreateMockResponseForScaling("predictor", 1.0, 75.0),
				"scaler":    testutil.CreateMockResponseForScaling("scaler", 2.0, 50.0),
			},
			expectedNames: map[string]bool{
				"cpu.usage":            true,
				"memory.usage":         true,
				"cpu_usage.prediction": true,        // Intelligent naming
				"scaled.memory.usage":  true,        // Pattern-based naming
			},
			description: "Mix of intelligent and pattern-based naming",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			mockServer := testutil.NewMockInferenceServer()
			mockServer.Start(t)
			defer mockServer.Stop()

			// Set up mock responses
			for model, response := range tt.mockResponses {
				mockServer.SetModelResponse(model, response)
			}

			// Create processor config
			cfg := &Config{
				GRPCClientSettings: GRPCClientSettings{
					Endpoint: mockServer.Endpoint(),
				},
				Rules: tt.rules,
			}

			// Create processor
			sink := new(consumertest.MetricsSink)
			processor, err := newMetricsProcessor(cfg, sink, zap.NewNop())
			require.NoError(t, err)

			err = processor.Start(context.Background(), nil)
			require.NoError(t, err)
			defer processor.Shutdown(context.Background())

			// Create test metrics
			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()
			sm := rm.ScopeMetrics().AppendEmpty()
			
			// Add all input metrics
			inputNames := make(map[string]bool)
			for _, rule := range tt.rules {
				for _, input := range rule.Inputs {
					if !inputNames[input] {
						metric := sm.Metrics().AppendEmpty()
						metric.SetName(input)
						metric.SetEmptyGauge()
						dp := metric.Gauge().DataPoints().AppendEmpty()
						dp.SetDoubleValue(100.0)
						inputNames[input] = true
					}
				}
			}

			// Process metrics
			err = processor.ConsumeMetrics(context.Background(), md)
			require.NoError(t, err)

			// Verify output
			allMetrics := sink.AllMetrics()
			require.Len(t, allMetrics, 1)

			// Collect all metric names
			actualNames := make(map[string]bool)
			result := allMetrics[0]
			for i := 0; i < result.ResourceMetrics().Len(); i++ {
				rm := result.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						metric := sm.Metrics().At(k)
						actualNames[metric.Name()] = true
						t.Logf("Found metric: %s", metric.Name())
					}
				}
			}

			// Verify expected metrics exist
			for expectedName := range tt.expectedNames {
				assert.True(t, actualNames[expectedName], 
					"%s: Expected metric '%s' not found. Description: %s", 
					tt.name, expectedName, tt.description)
			}

			// Verify no unexpected metrics
			for actualName := range actualNames {
				assert.True(t, tt.expectedNames[actualName], 
					"%s: Unexpected metric '%s' found", 
					tt.name, actualName)
			}
		})
	}
}

// TestExplicitVsIntelligentNaming tests that both naming strategies work correctly
func TestExplicitVsIntelligentNaming(t *testing.T) {
	// Start mock server
	mockServer := testutil.NewMockInferenceServer()
	mockServer.Start(t)
	defer mockServer.Stop()

	mockServer.SetModelResponse("model1", testutil.CreateMockResponseForScaling("model1", 1.0, 95.0))

	testCases := []struct {
		name         string
		rule         Rule
		expectedName string
	}{
		{
			name: "explicit_pattern",
			rule: Rule{
				ModelName:     "model1",
				Inputs:        []string{"input.metric"},
				OutputPattern: "custom.{model}.output",
				Outputs: []OutputSpec{
					{Name: "result"},
				},
			},
			expectedName: "custom.model1.output",
		},
		{
			name: "intelligent_naming",
			rule: Rule{
				ModelName: "model1",
				Inputs:    []string{"input.metric"},
				// No OutputPattern
				Outputs: []OutputSpec{
					{Name: "result"},
				},
			},
			expectedName: "input_metric.result",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				GRPCClientSettings: GRPCClientSettings{
					Endpoint: mockServer.Endpoint(),
				},
				Rules: []Rule{tc.rule},
			}

			sink := new(consumertest.MetricsSink)
			processor, err := newMetricsProcessor(cfg, sink, zap.NewNop())
			require.NoError(t, err)

			err = processor.Start(context.Background(), nil)
			require.NoError(t, err)
			defer processor.Shutdown(context.Background())

			// Create input metric
			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()
			sm := rm.ScopeMetrics().AppendEmpty()
			metric := sm.Metrics().AppendEmpty()
			metric.SetName("input.metric")
			metric.SetEmptyGauge()
			dp := metric.Gauge().DataPoints().AppendEmpty()
			dp.SetDoubleValue(50.0)

			// Process
			err = processor.ConsumeMetrics(context.Background(), md)
			require.NoError(t, err)

			// Check output
			allMetrics := sink.AllMetrics()
			require.Len(t, allMetrics, 1)

			// Find the output metric
			found := false
			result := allMetrics[0]
			for i := 0; i < result.ResourceMetrics().Len(); i++ {
				rm := result.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						metric := sm.Metrics().At(k)
						if metric.Name() == tc.expectedName {
							found = true
							break
						}
					}
				}
			}

			assert.True(t, found, "Expected metric '%s' not found", tc.expectedName)
		})
	}
}