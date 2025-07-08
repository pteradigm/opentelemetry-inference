// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsinferenceprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/internal/metadata"
	"github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/internal/testutil"
	pb "github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/proto/v2"
)

func TestBroadcastSemanticsForMixedInputs(t *testing.T) {
	// Start mock inference server
	mockServer := testutil.NewMockInferenceServer()
	mockServer.Start(t)
	defer mockServer.Stop()

	// Configure a mock response for product operation
	// Input 1: memory utilization with 3 states (used, free, cached) = [50.0, 30.0, 20.0]
	// Input 2: memory limit (no attributes) = [8000000000.0] (8GB)
	// Expected output: [4000000000.0, 2400000000.0, 1600000000.0] (utilization * limit)
	mockResponse := &pb.ModelInferResponse{
		ModelName:    "simple-product",
		ModelVersion: "1",
		Id:           "test-request",
		Outputs: []*pb.ModelInferResponse_InferOutputTensor{
			{
				Name:     "product_result",
				Datatype: "FP64",
				Shape:    []int64{3}, // Three output values for three memory states
				Contents: &pb.InferTensorContents{
					// Values arranged by sorted attribute order: cached, free, used
					Fp64Contents: []float64{1600000000.0, 2400000000.0, 4000000000.0},
				},
			},
		},
	}
	mockServer.SetModelResponse("simple-product", mockResponse)

	// Create processor config
	cfg := &Config{
		GRPCClientSettings: GRPCClientSettings{
			Endpoint: mockServer.Endpoint(),
		},
		Rules: []Rule{
			{
				ModelName: "simple-product",
				Inputs:    []string{"system.memory.utilization", "system.memory.limit"},
				OutputPattern: "{output}",
				Outputs: []OutputSpec{
					{
						Name: "memory_utilization_memory_limit.product_result",
					},
				},
			},
		},
	}

	// Create test consumer
	sink := &consumertest.MetricsSink{}

	// Create processor
	factory := NewFactory()
	processor, err := factory.CreateMetrics(
		context.Background(),
		processortest.NewNopSettings(metadata.Type),
		cfg,
		sink,
	)
	require.NoError(t, err)
	require.NoError(t, processor.Start(context.Background(), nil))
	defer func() {
		require.NoError(t, processor.Shutdown(context.Background()))
	}()

	// Create test metrics with mixed attribute schemas
	metrics := createMetricsWithMixedAttributeSchemas()

	// Process metrics
	err = processor.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)

	// Verify results
	allMetrics := sink.AllMetrics()
	require.Len(t, allMetrics, 1)

	outputMetrics := allMetrics[0]
	require.Equal(t, 1, outputMetrics.ResourceMetrics().Len())

	rm := outputMetrics.ResourceMetrics().At(0)
	require.Equal(t, 1, rm.ScopeMetrics().Len())

	sm := rm.ScopeMetrics().At(0)
	require.Equal(t, 3, sm.Metrics().Len()) // Original 2 + inference output

	// Find the inference output metric
	var inferenceMetric pmetric.Metric
	for i := 0; i < sm.Metrics().Len(); i++ {
		metric := sm.Metrics().At(i)
		if metric.Name() == "memory_utilization_memory_limit.product_result" {
			inferenceMetric = metric
			break
		}
	}
	require.NotNil(t, inferenceMetric, "inference output metric not found")

	// Verify the metric has correct number of data points (should broadcast to all memory states)
	require.Equal(t, pmetric.MetricTypeGauge, inferenceMetric.Type())
	gauge := inferenceMetric.Gauge()
	require.Equal(t, 3, gauge.DataPoints().Len(), "should have 3 output data points for broadcast")

	// Verify each data point has the correct attributes and values
	// Order-independent verification since attribute sorting affects order
	expectedStateValues := map[string]float64{
		"used":   4000000000.0, // 50% * 8GB = 4GB
		"free":   2400000000.0, // 30% * 8GB = 2.4GB
		"cached": 1600000000.0, // 20% * 8GB = 1.6GB
	}

	actualStateValues := make(map[string]float64)
	
	for i := 0; i < gauge.DataPoints().Len(); i++ {
		dp := gauge.DataPoints().At(i)
		attrs := dp.Attributes()
		
		// Check that state attribute is preserved from the attributed input
		// Now using namespaced attributes
		state, ok := attrs.Get("system.memory.utilization.state")
		require.True(t, ok, "system.memory.utilization.state attribute should be present on data point %d", i)
		
		actualStateValues[state.Str()] = dp.DoubleValue()
		
		// Verify state attribute is preserved (no inference labels should be added)
	}
	
	// Verify all state values match expected
	assert.Equal(t, expectedStateValues, actualStateValues, "all state values should match expected")
}

func createMetricsWithMixedAttributeSchemas() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	// Metric 1: system.memory.utilization - HAS attributes (state)
	metric1 := sm.Metrics().AppendEmpty()
	metric1.SetName("system.memory.utilization")
	gauge1 := metric1.SetEmptyGauge()

	// Data point 1: used memory (50%)
	dp1 := gauge1.DataPoints().AppendEmpty()
	dp1.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp1.SetDoubleValue(50.0)
	dp1.Attributes().PutStr("state", "used")
	dp1.Attributes().PutStr("host", "server-1")

	// Data point 2: free memory (30%)
	dp2 := gauge1.DataPoints().AppendEmpty()
	dp2.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp2.SetDoubleValue(30.0)
	dp2.Attributes().PutStr("state", "free")
	dp2.Attributes().PutStr("host", "server-1")

	// Data point 3: cached memory (20%)
	dp3 := gauge1.DataPoints().AppendEmpty()
	dp3.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp3.SetDoubleValue(20.0)
	dp3.Attributes().PutStr("state", "cached")
	dp3.Attributes().PutStr("host", "server-1")

	// Metric 2: system.memory.limit - NO attributes (single value)
	metric2 := sm.Metrics().AppendEmpty()
	metric2.SetName("system.memory.limit")
	gauge2 := metric2.SetEmptyGauge()

	// Single data point: total memory limit (8GB)
	dp4 := gauge2.DataPoints().AppendEmpty()
	dp4.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp4.SetDoubleValue(8000000000.0) // 8GB in bytes
	dp4.Attributes().PutStr("host", "server-1")

	return md
}
