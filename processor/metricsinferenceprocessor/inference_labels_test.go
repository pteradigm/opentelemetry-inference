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

func TestAttributesPreservedWithModelLabels(t *testing.T) {
	// Start mock inference server
	mockServer := testutil.NewMockInferenceServer()
	mockServer.Start(t)
	defer mockServer.Stop()

	// Configure a mock response for the scale model
	mockResponse := &pb.ModelInferResponse{
		ModelName:    "simple-scaler",
		ModelVersion: "v1.0",
		Id:           "test-request",
		Outputs: []*pb.ModelInferResponse_InferOutputTensor{
			{
				Name:     "scaled_output",
				Datatype: "FP64",
				Shape:    []int64{1},
				Contents: &pb.InferTensorContents{
					Fp64Contents: []float64{50.0}, // 25.0 * 2.0
				},
			},
		},
	}
	mockServer.SetModelResponse("simple-scaler", mockResponse)

	// Create processor config
	cfg := &Config{
		GRPCClientSettings: GRPCClientSettings{
			Endpoint: mockServer.Endpoint(),
		},
		Rules: []Rule{
			{
				ModelName:    "simple-scaler",
				ModelVersion: "v1.0",
				Inputs:       []string{"test.metric"},
				OutputPattern: "{output}",
				Outputs: []OutputSpec{
					{
						Name: "test.metric.scaled",
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

	// Create test metrics
	metrics := createTestMetricsWithAttributes()

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
		if metric.Name() == "test.metric.scaled" {
			inferenceMetric = metric
			break
		}
	}
	require.NotNil(t, inferenceMetric, "inference output metric not found")

	// Verify the metric has data points
	require.Equal(t, pmetric.MetricTypeGauge, inferenceMetric.Type())
	gauge := inferenceMetric.Gauge()
	require.Equal(t, 1, gauge.DataPoints().Len())

	// Verify inference labels are present
	dp := gauge.DataPoints().At(0)
	attrs := dp.Attributes()

	// Debug: print all attributes
	t.Logf("Output metric attributes count: %d", attrs.Len())
	attrs.Range(func(k string, v pcommon.Value) bool {
		t.Logf("  %s: %s", k, v.AsString())
		return true
	})
	
	// Verify original attributes are preserved with namespacing
	originalAttr, ok := attrs.Get("test.metric.test.label")
	require.True(t, ok, "namespaced test.label attribute missing")
	assert.Equal(t, "test.value", originalAttr.Str())
	
	hostAttr, ok := attrs.Get("test.metric.host")
	require.True(t, ok, "namespaced host attribute missing")
	assert.Equal(t, "test-host", hostAttr.Str())
	
	// Verify model name and version labels are present (but not status)
	modelName, hasModelName := attrs.Get("otel.inference.model.name")
	assert.True(t, hasModelName, "model name label should be present")
	assert.Equal(t, "simple-scaler", modelName.Str())
	
	modelVersion, hasModelVersion := attrs.Get("otel.inference.model.version")
	assert.True(t, hasModelVersion, "model version label should be present")
	assert.Equal(t, "v1.0", modelVersion.Str())
	
	// Verify NO status label is added (we removed it)
	_, hasStatus := attrs.Get("otel.inference.status")
	assert.False(t, hasStatus, "status label should not be present")
}


func createTestMetricsWithAttributes() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	// Create a gauge metric with attributes
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test.metric")
	gauge := metric.SetEmptyGauge()

	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(25.0)

	// Add test attributes to verify they're preserved
	attrs := dp.Attributes()
	attrs.PutStr("test.label", "test.value")
	attrs.PutStr("host", "test-host")

	return md
}

