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

func TestMultipleDataPointsPreserveAttributes(t *testing.T) {
	// Start mock inference server
	mockServer := testutil.NewMockInferenceServer()
	mockServer.Start(t)
	defer mockServer.Stop()

	// Configure a mock response that doubles input values
	// We'll create a custom response since the standard function expects single values
	mockResponse := &pb.ModelInferResponse{
		ModelName:    "test-scaler",
		ModelVersion: "1",
		Id:           "test-request",
		Outputs: []*pb.ModelInferResponse_InferOutputTensor{
			{
				Name:     "scaled_output",
				Datatype: "FP64",
				Shape:    []int64{3}, // Three output values
				Contents: &pb.InferTensorContents{
					// Values arranged to match sorted attribute order: cached, free, used
					Fp64Contents: []float64{60.0, 40.0, 20.0}, // cached(30→60), free(20→40), used(10→20)
				},
			},
		},
	}
	mockServer.SetModelResponse("test-scaler", mockResponse)

	// Create processor config
	cfg := &Config{
		GRPCClientSettings: GRPCClientSettings{
			Endpoint: mockServer.Endpoint(),
		},
		Rules: []Rule{
			{
				ModelName:     "test-scaler",
				Inputs:        []string{"memory.usage"},
				OutputPattern: "{output}",
				Outputs: []OutputSpec{
					{
						Name: "memory.usage.scaled",
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

	// Create test metrics with multiple data points (different states)
	metrics := createMetricsWithMultipleDataPoints()

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
	require.Equal(t, 2, sm.Metrics().Len()) // Original + inference output

	// Find the inference output metric
	var inferenceMetric pmetric.Metric
	for i := 0; i < sm.Metrics().Len(); i++ {
		metric := sm.Metrics().At(i)
		if metric.Name() == "memory.usage.scaled" {
			inferenceMetric = metric
			break
		}
	}
	require.NotNil(t, inferenceMetric, "inference output metric not found")

	// Verify the metric has correct number of data points
	require.Equal(t, pmetric.MetricTypeGauge, inferenceMetric.Type())
	gauge := inferenceMetric.Gauge()
	require.Equal(t, 3, gauge.DataPoints().Len(), "should have 3 output data points")

	// Verify each data point has the correct attributes and values
	// Now that ordering is fixed, values should correctly match their inputs
	expectedStateValues := map[string]float64{
		"used":   20.0, // doubled from input 10.0
		"free":   40.0, // doubled from input 20.0
		"cached": 60.0, // doubled from input 30.0
	}

	// Collect actual data points by state for order-independent verification
	actualStateValues := make(map[string]float64)

	for i := 0; i < gauge.DataPoints().Len(); i++ {
		dp := gauge.DataPoints().At(i)
		attrs := dp.Attributes()

		// Check that state attribute is preserved with namespacing
		state, ok := attrs.Get("memory.usage.state")
		require.True(t, ok, "memory.usage.state attribute should be present on data point %d", i)

		actualStateValues[state.Str()] = dp.DoubleValue()

		// Verify state attribute is preserved (no inference labels should be added)
	}

	// Verify we have all expected states with correct values
	assert.Equal(t, expectedStateValues, actualStateValues, "all state values should match expected")
}

func createMetricsWithMultipleDataPoints() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	// Create a gauge metric with multiple data points (different states)
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("memory.usage")
	gauge := metric.SetEmptyGauge()

	// Data point 1: used memory
	dp1 := gauge.DataPoints().AppendEmpty()
	dp1.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp1.SetDoubleValue(10.0)
	dp1.Attributes().PutStr("state", "used")
	dp1.Attributes().PutStr("host", "server-1")

	// Data point 2: free memory
	dp2 := gauge.DataPoints().AppendEmpty()
	dp2.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp2.SetDoubleValue(20.0)
	dp2.Attributes().PutStr("state", "free")
	dp2.Attributes().PutStr("host", "server-1")

	// Data point 3: cached memory
	dp3 := gauge.DataPoints().AppendEmpty()
	dp3.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp3.SetDoubleValue(30.0)
	dp3.Attributes().PutStr("state", "cached")
	dp3.Attributes().PutStr("host", "server-1")

	return md
}
