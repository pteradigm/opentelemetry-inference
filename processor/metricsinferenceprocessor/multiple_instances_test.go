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

func TestMultipleInstancesWithUniqueOutputs(t *testing.T) {
	// Test that multiple instances of the same model produce uniquely named outputs

	mockServer := testutil.NewMockInferenceServer()
	mockServer.Start(t)
	defer mockServer.Stop()

	// Configure metadata for the scaling model
	mockServer.SetModelMetadata("scaler", &pb.ModelMetadataResponse{
		Name: "scaler",
		Outputs: []*pb.ModelMetadataResponse_TensorMetadata{
			{
				Name:     "scaled_value",
				Datatype: "FP64",
				Shape:    []int64{1},
			},
		},
	})

	// Configure inference responses
	mockServer.SetModelResponse("scaler", &pb.ModelInferResponse{
		ModelName: "scaler",
		Outputs: []*pb.ModelInferResponse_InferOutputTensor{
			{
				Name:     "scaled_value",
				Datatype: "FP64",
				Shape:    []int64{1},
				Contents: &pb.InferTensorContents{
					Fp64Contents: []float64{10.0}, // Scaled result
				},
			},
		},
	})

	config := &Config{
		GRPCClientSettings: GRPCClientSettings{
			Endpoint: mockServer.Endpoint(),
		},
		Rules: []Rule{
			{
				// Instance 1: Scale CPU metrics
				ModelName: "scaler",
				Inputs:    []string{"system.cpu.utilization"},
				// No outputs configured - will discover and decorate with input name
			},
			{
				// Instance 2: Scale memory metrics (same model, different input)
				ModelName: "scaler",
				Inputs:    []string{"system.memory.utilization"},
				// No outputs configured - will discover and decorate with input name
			},
			{
				// Instance 3: Scale multiple metrics (same model, multiple inputs)
				ModelName: "scaler",
				Inputs:    []string{"app.requests", "app.latency"},
				// No outputs configured - will discover and decorate with "_multi" suffix
			},
		},
		Timeout: 30,
	}

	sink := &consumertest.MetricsSink{}
	processor, err := newMetricsProcessor(config, sink, zap.NewNop())
	require.NoError(t, err)

	err = processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())

	// Verify that each rule has uniquely decorated output names
	assert.Equal(t, 3, len(processor.rules))

	// Rule 1: Single input (CPU) should be decorated as "cpu_utilization.scaled_value"
	assert.Equal(t, 1, len(processor.rules[0].outputs))
	assert.Equal(t, "cpu_utilization.scaled_value", processor.rules[0].outputs[0].name)
	assert.True(t, processor.rules[0].outputs[0].discovered)

	// Rule 2: Single input (memory) should be decorated as "memory_utilization.scaled_value"
	assert.Equal(t, 1, len(processor.rules[1].outputs))
	assert.Equal(t, "memory_utilization.scaled_value", processor.rules[1].outputs[0].name)
	assert.True(t, processor.rules[1].outputs[0].discovered)

	// Rule 3: Multiple inputs should be decorated as "requests_latency.scaled_value"
	assert.Equal(t, 1, len(processor.rules[2].outputs))
	assert.Equal(t, "requests_latency.scaled_value", processor.rules[2].outputs[0].name)
	assert.True(t, processor.rules[2].outputs[0].discovered)

	// Create test metrics that match the inputs
	inputMetrics := testutil.GenerateTestMetrics(testutil.TestMetric{
		MetricNames:  []string{"system.cpu.utilization", "system.memory.utilization", "app.requests", "app.latency", "unrelated.metric"},
		MetricValues: [][]float64{{5.0}, {3.0}, {100.0}, {20.0}, {42.0}},
	})

	// Process metrics through the processor
	err = processor.ConsumeMetrics(context.Background(), inputMetrics)
	require.NoError(t, err)

	// Verify results
	require.Len(t, sink.AllMetrics(), 1, "Expected exactly one metrics batch")
	outputMetrics := sink.AllMetrics()[0]

	// Collect all metric names
	actualNames := make(map[string]bool)
	for i := 0; i < outputMetrics.ResourceMetrics().Len(); i++ {
		rm := outputMetrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				actualNames[metric.Name()] = true
			}
		}
	}

	// Verify all original metrics are present
	assert.True(t, actualNames["system.cpu.utilization"], "Original CPU metric should be present")
	assert.True(t, actualNames["system.memory.utilization"], "Original memory metric should be present")
	assert.True(t, actualNames["app.requests"], "Original requests metric should be present")
	assert.True(t, actualNames["app.latency"], "Original latency metric should be present")
	assert.True(t, actualNames["unrelated.metric"], "Unrelated metric should be present")

	// Verify uniquely decorated output metrics are present
	assert.True(t, actualNames["cpu_utilization.scaled_value"], "CPU scaled metric should be present")
	assert.True(t, actualNames["memory_utilization.scaled_value"], "Memory scaled metric should be present")
	assert.True(t, actualNames["requests_latency.scaled_value"], "Multi-input scaled metric should be present")

	// Verify we don't have conflicting names (what would happen without decoration)
	conflictingNames := []string{"scaled_value"} // This would be the undecoratedname
	for _, name := range conflictingNames {
		assert.False(t, actualNames[name], "Should not have undecorated name: %s", name)
	}

	t.Logf("Generated metric names: %v", func() []string {
		var names []string
		for name := range actualNames {
			names = append(names, name)
		}
		return names
	}())
}
