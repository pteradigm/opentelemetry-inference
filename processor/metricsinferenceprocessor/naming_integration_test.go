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
	"go.uber.org/zap"

	pb "github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/proto/v2"
	"github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/internal/testutil"
)

func TestIntelligentNamingIntegration(t *testing.T) {
	// Start mock inference server
	mockServer := testutil.NewMockInferenceServer()
	mockServer.Start(t)
	defer mockServer.Stop()

	// Configure mock responses
	mockServer.SetModelResponse("anomaly-detector", &pb.ModelInferResponse{
		ModelName:    "anomaly-detector",
		ModelVersion: "1",
		Id:           "test-request",
		Outputs: []*pb.ModelInferResponse_InferOutputTensor{
			{
				Name:     "anomaly_score",
				Datatype: "FP64",
				Shape:    []int64{1},
				Contents: &pb.InferTensorContents{
					Fp64Contents: []float64{0.85},
				},
			},
		},
	})

	// Set up metadata for automatic discovery
	mockServer.SetModelMetadata("anomaly-detector", &pb.ModelMetadataResponse{
		Name: "anomaly-detector",
		Outputs: []*pb.ModelMetadataResponse_TensorMetadata{
			{
				Name:     "anomaly_score",
				Datatype: "FP64",
				Shape:    []int64{1},
			},
		},
	})

	tests := []struct {
		name           string
		config         *Config
		inputMetrics   []string
		expectedOutput string
	}{
		{
			name: "intelligent_naming_with_common_prefix",
			config: &Config{
				GRPCClientSettings: GRPCClientSettings{
					Endpoint: mockServer.Endpoint(),
				},
				Rules: []Rule{
					{
						ModelName: "anomaly-detector",
						Inputs:    []string{"system.cpu.utilization", "system.memory.usage"},
						// No outputs specified - will use discovery + intelligent naming
					},
				},
				Naming: DefaultNamingConfig(),
			},
			inputMetrics:   []string{"system.cpu.utilization", "system.memory.usage"},
			expectedOutput: "cpu_utilization_memory_usage.anomaly_score",
		},
		{
			name: "custom_stem_parts",
			config: &Config{
				GRPCClientSettings: GRPCClientSettings{
					Endpoint: mockServer.Endpoint(),
				},
				Rules: []Rule{
					{
						ModelName: "anomaly-detector",
						Inputs:    []string{"app.service.api.latency"},
					},
				},
				Naming: NamingConfig{
					MaxStemParts:           3,
					SkipCommonDomains:      false,
					EnableCategoryGrouping: true,
					AbbreviationThreshold:  4,
				},
			},
			inputMetrics:   []string{"app.service.api.latency"},
			expectedOutput: "service_api_latency.anomaly_score",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test consumer
			sink := &consumertest.MetricsSink{}

			// Create processor
			processor, err := newMetricsProcessor(tt.config, sink, zap.NewNop())
			require.NoError(t, err)

			// Start processor
			err = processor.Start(context.Background(), nil)
			require.NoError(t, err)
			defer processor.Shutdown(context.Background())

			// Create test metrics
			inputMetrics := createTestMetrics(tt.inputMetrics)

			// Process metrics
			err = processor.ConsumeMetrics(context.Background(), inputMetrics)
			require.NoError(t, err)

			// Verify output
			allMetrics := sink.AllMetrics()
			require.Len(t, allMetrics, 1)

			// Find the inference output metric
			outputMetrics := allMetrics[0]
			found := false
			outputMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().RemoveIf(
				func(metric pmetric.Metric) bool {
					if metric.Name() == tt.expectedOutput {
						found = true
						// Verify it's a gauge with expected value
						assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())
						assert.Equal(t, 1, metric.Gauge().DataPoints().Len())
						assert.InDelta(t, 0.85, metric.Gauge().DataPoints().At(0).DoubleValue(), 0.001)
					}
					return false
				})

			assert.True(t, found, "Expected output metric %s not found", tt.expectedOutput)
		})
	}
}

func TestNamingWithPatternOverride(t *testing.T) {
	// Start mock inference server
	mockServer := testutil.NewMockInferenceServer()
	mockServer.Start(t)
	defer mockServer.Stop()

	// Configure mock response
	mockServer.SetModelResponse("predictor", &pb.ModelInferResponse{
		ModelName:    "predictor",
		ModelVersion: "1",
		Id:           "test-request",
		Outputs: []*pb.ModelInferResponse_InferOutputTensor{
			{
				Name:     "result",
				Datatype: "FP64",
				Shape:    []int64{1},
				Contents: &pb.InferTensorContents{
					Fp64Contents: []float64{42.0},
				},
			},
		},
	})

	// Set up metadata for automatic discovery
	mockServer.SetModelMetadata("predictor", &pb.ModelMetadataResponse{
		Name: "predictor",
		Outputs: []*pb.ModelMetadataResponse_TensorMetadata{
			{
				Name:     "result",
				Datatype: "FP64",
				Shape:    []int64{1},
			},
		},
	})

	// Create processor with pattern override
	config := &Config{
		GRPCClientSettings: GRPCClientSettings{
			Endpoint: mockServer.Endpoint(),
		},
		Rules: []Rule{
			{
				ModelName:     "predictor",
				Inputs:        []string{"system.cpu.utilization"},
				OutputPattern: "custom.{model}.{output}",
				// No outputs specified - will use discovery + pattern naming
			},
		},
		Naming: DefaultNamingConfig(),
	}

	sink := &consumertest.MetricsSink{}
	processor, err := newMetricsProcessor(config, sink, zap.NewNop())
	require.NoError(t, err)

	err = processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())

	// Process metrics
	inputMetrics := createTestMetrics([]string{"system.cpu.utilization"})
	err = processor.ConsumeMetrics(context.Background(), inputMetrics)
	require.NoError(t, err)

	// Verify custom pattern was used
	allMetrics := sink.AllMetrics()
	require.Len(t, allMetrics, 1)

	found := false
	expectedName := "custom.predictor.result"
	allMetrics[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().RemoveIf(
		func(metric pmetric.Metric) bool {
			if metric.Name() == expectedName {
				found = true
			}
			return false
		})

	assert.True(t, found, "Expected output metric %s not found", expectedName)
}

func createTestMetrics(metricNames []string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	for _, name := range metricNames {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(name)
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetDoubleValue(75.0)
	}

	return md
}