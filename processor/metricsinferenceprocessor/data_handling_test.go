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
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"

	"github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/internal/testutil"
	pb "github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/proto/v2"
)

func TestDataHandlingModes(t *testing.T) {
	// Set up a mock inference server
	mockServer := testutil.NewMockInferenceServer()
	mockServer.Start(t)
	defer mockServer.Stop()

	tests := []struct {
		name           string
		dataHandling   DataHandlingConfig
		inputDataCount int
		expectedCount  int
		description    string
	}{
		{
			name: "latest_mode_single_point",
			dataHandling: DataHandlingConfig{
				Mode:               "latest",
				AlignTimestamps:    false,
				TimestampTolerance: 1000,
			},
			inputDataCount: 5,
			expectedCount:  1,
			description:    "Latest mode should only send the most recent data point",
		},
		{
			name: "window_mode_size_3",
			dataHandling: DataHandlingConfig{
				Mode:               "window",
				WindowSize:         3,
				AlignTimestamps:    false,
				TimestampTolerance: 1000,
			},
			inputDataCount: 5,
			expectedCount:  3,
			description:    "Window mode should send last 3 data points",
		},
		{
			name: "window_mode_size_exceeds_data",
			dataHandling: DataHandlingConfig{
				Mode:               "window",
				WindowSize:         10,
				AlignTimestamps:    false,
				TimestampTolerance: 1000,
			},
			inputDataCount: 5,
			expectedCount:  5,
			description:    "Window mode should send all points when window exceeds data size",
		},
		{
			name: "all_mode",
			dataHandling: DataHandlingConfig{
				Mode:               "all",
				AlignTimestamps:    false,
				TimestampTolerance: 1000,
			},
			inputDataCount: 5,
			expectedCount:  5,
			description:    "All mode should send all accumulated data points",
		},
		{
			name: "default_empty_mode",
			dataHandling: DataHandlingConfig{
				Mode:               "",
				AlignTimestamps:    false,
				TimestampTolerance: 1000,
			},
			inputDataCount: 5,
			expectedCount:  1,
			description:    "Empty mode should default to latest behavior",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock server requests between tests
			mockServer.Reset()
			
			// Configure mock response - inference servers typically return a single result
			// even when processing multiple data points (e.g., batch prediction)
			mockServer.SetModelResponse("test-scaler", &pb.ModelInferResponse{
				ModelName:    "test-scaler",
				ModelVersion: "v1",
				Outputs: []*pb.ModelInferResponse_InferOutputTensor{
					{
						Name:     "output",
						Datatype: "FP64",
						Shape:    []int64{1},
						Contents: &pb.InferTensorContents{
							Fp64Contents: []float64{200.0}, // Single inference result
						},
					},
				},
			})
			
			// Set up model metadata
			mockServer.SetModelMetadata("test-scaler", &pb.ModelMetadataResponse{
				Name:     "test-scaler",
				Versions: []string{"v1"},
				Platform: "mock",
				Inputs: []*pb.ModelMetadataResponse_TensorMetadata{
					{Name: "input", Datatype: "FP64", Shape: []int64{-1}},
				},
				Outputs: []*pb.ModelMetadataResponse_TensorMetadata{
					{Name: "output", Datatype: "FP64", Shape: []int64{-1}},
				},
			})
			// Create processor config
			cfg := &Config{
				GRPCClientSettings: GRPCClientSettings{
					Endpoint: mockServer.GetAddress(),
				},
				Rules: []Rule{
					{
						ModelName: "test-scaler",
						Inputs:    []string{"test.metric"},
					},
				},
				Timeout:      10,
				DataHandling: tt.dataHandling,
			}

			// Create metrics with multiple data points
			md := createMetricsWithMultipleDataPointsForTest("test.metric", tt.inputDataCount)

			// Create processor
			sink := &consumertest.MetricsSink{}
			mp, err := newMetricsProcessor(cfg, sink, zap.NewNop())
			require.NoError(t, err)
			
			// Start processor
			err = mp.Start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)
			defer mp.Shutdown(context.Background())

			// Process metrics
			err = mp.ConsumeMetrics(context.Background(), md)
			require.NoError(t, err)

			// Verify the output
			allMetrics := sink.AllMetrics()
			require.Len(t, allMetrics, 1)

			// Find the output metric (processor converts dots to underscores in the input name)
			outputMetric := findMetricByName(allMetrics[0], "test_metric.output")
			require.NotNil(t, outputMetric, "Output metric not found")

			// For real-time inference, the processor creates one output metric
			// regardless of how many input data points were sent
			var dpCount int
			switch outputMetric.Type() {
			case pmetric.MetricTypeGauge:
				dpCount = outputMetric.Gauge().DataPoints().Len()
			case pmetric.MetricTypeSum:
				dpCount = outputMetric.Sum().DataPoints().Len()
			}

			// The output should always be 1 data point for real-time inference
			assert.Equal(t, 1, dpCount, "Real-time inference should produce single output")

			// Verify the input was sent correctly by checking the request
			requests := mockServer.GetRequests()
			require.Len(t, requests, 1, "Expected one inference request")
			
			// Check that the input tensor has the expected number of values
			require.Len(t, requests[0].Inputs, 1, "Expected one input tensor")
			inputTensor := requests[0].Inputs[0]
			actualInputCount := len(inputTensor.Contents.Fp64Contents)
			
			assert.Equal(t, tt.expectedCount, actualInputCount, tt.description)
		})
	}
}

func TestDataHandlingWithTemporalAlignment(t *testing.T) {
	// Set up a mock inference server
	mockServer := testutil.NewMockInferenceServer()
	mockServer.Start(t)
	defer mockServer.Stop()

	// Configure mock for multi-input model
	sumResponse := &pb.ModelInferResponse{
		ModelName:    "multi-input",
		ModelVersion: "v1",
		Outputs: []*pb.ModelInferResponse_InferOutputTensor{
			{
				Name:     "sum",
				Datatype: "FP64",
				Shape:    []int64{1},
				Contents: &pb.InferTensorContents{
					Fp64Contents: []float64{100.0},
				},
			},
		},
	}
	mockServer.SetModelResponse("multi-input", sumResponse)
	mockServer.SetModelMetadata("multi-input", &pb.ModelMetadataResponse{
		Name:     "multi-input",
		Versions: []string{"v1"},
		Platform: "mock",
		Inputs: []*pb.ModelMetadataResponse_TensorMetadata{
			{Name: "input1", Datatype: "FP64", Shape: []int64{-1}},
			{Name: "input2", Datatype: "FP64", Shape: []int64{-1}},
		},
		Outputs: []*pb.ModelMetadataResponse_TensorMetadata{
			{Name: "sum", Datatype: "FP64", Shape: []int64{-1}},
		},
	})

	cfg := &Config{
		GRPCClientSettings: GRPCClientSettings{
			Endpoint: mockServer.GetAddress(),
		},
		Rules: []Rule{
			{
				ModelName: "multi-input",
				Inputs:    []string{"metric1", "metric2"},
			},
		},
		Timeout: 10,
		DataHandling: DataHandlingConfig{
			Mode:               "latest",
			AlignTimestamps:    true,
			TimestampTolerance: 1000, // 1 second tolerance
		},
	}

	// Create metrics with misaligned timestamps
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	// Metric 1 with timestamps at 0s, 10s, 20s
	metric1 := sm.Metrics().AppendEmpty()
	metric1.SetName("metric1")
	gauge1 := metric1.SetEmptyGauge()
	for i := 0; i < 3; i++ {
		dp := gauge1.DataPoints().AppendEmpty()
		dp.SetDoubleValue(float64(i + 1))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(int64(i*10), 0)))
	}

	// Metric 2 with timestamps at 0.5s, 10.5s, 20.5s (within tolerance)
	metric2 := sm.Metrics().AppendEmpty()
	metric2.SetName("metric2")
	gauge2 := metric2.SetEmptyGauge()
	for i := 0; i < 3; i++ {
		dp := gauge2.DataPoints().AppendEmpty()
		dp.SetDoubleValue(float64(i + 10))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(int64(i*10), 500000000))) // 500ms offset
	}

	// Create processor
	sink := &consumertest.MetricsSink{}
	mp, err := newMetricsProcessor(cfg, sink, zap.NewNop())
	require.NoError(t, err)

	// Start processor
	err = mp.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer mp.Shutdown(context.Background())

	// Process metrics
	err = mp.ConsumeMetrics(context.Background(), md)
	require.NoError(t, err)

	// Verify output
	allMetrics := sink.AllMetrics()
	require.Len(t, allMetrics, 1)

	// Find output metric
	outputMetric := findMetricByName(allMetrics[0], "metric1_metric2.sum")
	require.NotNil(t, outputMetric, "Output metric not found")

	// Should only have 1 data point (latest aligned pair)
	assert.Equal(t, 1, outputMetric.Gauge().DataPoints().Len(), 
		"Temporal alignment with latest mode should produce 1 data point")
}

// Helper functions

func createMetricsWithMultipleDataPointsForTest(metricName string, count int) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	
	metric := sm.Metrics().AppendEmpty()
	metric.SetName(metricName)
	gauge := metric.SetEmptyGauge()
	
	baseTime := time.Now()
	for i := 0; i < count; i++ {
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetDoubleValue(float64(i + 1) * 10.0)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(baseTime.Add(time.Duration(i) * time.Second)))
	}
	
	return md
}

func findMetricByName(md pmetric.Metrics, name string) pmetric.Metric {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				if metric.Name() == name {
					return metric
				}
			}
		}
	}
	return pmetric.NewMetric() // Return a properly initialized empty metric
}