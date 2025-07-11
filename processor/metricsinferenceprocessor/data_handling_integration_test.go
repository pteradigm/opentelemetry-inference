// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package metricsinferenceprocessor

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// TestDataHandlingIntegrationWithMLServer tests data handling modes with MLServer
func TestDataHandlingIntegrationWithMLServer(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("INTEGRATION_TEST") != "1" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=1 to run.")
	}

	testCases := []struct {
		name             string
		modelName        string
		dataHandlingMode string
		windowSize       int
		inputDataPoints  int
		expectedResults  []float64
		description      string
	}{
		{
			name:             "latest_mode_scaling",
			modelName:        "simple-scaler",
			dataHandlingMode: "latest",
			windowSize:       0,
			inputDataPoints:  5,
			expectedResults:  []float64{100.0}, // Last value (50) * 2
			description:      "Latest mode should only process the most recent data point",
		},
		{
			name:             "window_mode_scaling",
			modelName:        "simple-scaler",
			dataHandlingMode: "window",
			windowSize:       3,
			inputDataPoints:  5,
			expectedResults:  []float64{60.0, 80.0, 100.0}, // Last 3 values (30,40,50) * 2
			description:      "Window mode should process last N data points",
		},
		{
			name:             "all_mode_scaling",
			modelName:        "simple-scaler",
			dataHandlingMode: "all",
			windowSize:       0,
			inputDataPoints:  3,
			expectedResults:  []float64{20.0, 40.0, 60.0}, // All values (10,20,30) * 2
			description:      "All mode should process all data points",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create config with data handling
			cfg := &Config{
				GRPCClientSettings: GRPCClientSettings{
					Endpoint: "localhost:9081", // MLServer gRPC endpoint
					UseSSL:   false,
				},
				Rules: []Rule{
					{
						ModelName: tc.modelName,
						Inputs:    []string{"test.cpu.usage"},
					},
				},
				Timeout: 10,
				DataHandling: DataHandlingConfig{
					Mode:               tc.dataHandlingMode,
					WindowSize:         tc.windowSize,
					AlignTimestamps:    false,
					TimestampTolerance: 1000,
				},
			}

			// Create test metrics with multiple data points
			md := createTimeSeriesMetrics("test.cpu.usage", tc.inputDataPoints)

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

			// Verify results
			allMetrics := sink.AllMetrics()
			require.Len(t, allMetrics, 1, "Expected metrics to be processed")

			// Find the output metric
			var outputMetric pmetric.Metric
			for i := 0; i < allMetrics[0].ResourceMetrics().Len(); i++ {
				rm := allMetrics[0].ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						metric := sm.Metrics().At(k)
						// Try various possible output names
						if metric.Name() == "test_cpu_usage.scaled_output" ||
							metric.Name() == "test_cpu_usage.output" ||
							metric.Name() == "test.cpu.usage.output" ||
							metric.Name() == "cpu_usage.scaled_result" ||
							metric.Name() == "test_cpu_usage.scaled_result" {
							outputMetric = metric
						}
					}
				}
			}

			require.False(t, outputMetric.Type() == pmetric.MetricTypeEmpty, "Output metric not found")

			// Verify data points match expected results
			gauge := outputMetric.Gauge()
			require.Equal(t, len(tc.expectedResults), gauge.DataPoints().Len(), tc.description)

			// Verify values
			for i := 0; i < gauge.DataPoints().Len(); i++ {
				dp := gauge.DataPoints().At(i)
				assert.Equal(t, tc.expectedResults[i], dp.DoubleValue(),
					"Data point %d should match expected value", i)
			}
		})
	}
}

// TestDataHandlingWithMultipleInputs tests data handling with multiple input metrics
func TestDataHandlingWithMultipleInputs(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("INTEGRATION_TEST") != "1" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=1 to run.")
	}

	// Create config with multiple inputs
	cfg := &Config{
		GRPCClientSettings: GRPCClientSettings{
			Endpoint: "localhost:9081", // MLServer gRPC endpoint
			UseSSL:   false,
		},
		Rules: []Rule{
			{
				ModelName: "simple-sum",
				Inputs:    []string{"metric1", "metric2"},
			},
		},
		Timeout: 10,
		DataHandling: DataHandlingConfig{
			Mode:               "latest",
			AlignTimestamps:    true,
			TimestampTolerance: 1000, // 1 second
		},
	}

	// Create test metrics with aligned timestamps
	md := createAlignedMetrics()

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

	// Verify that only the latest aligned pair was processed
	allMetrics := sink.AllMetrics()
	require.Len(t, allMetrics, 1)

	// Find output metric
	var outputMetric pmetric.Metric
	for i := 0; i < allMetrics[0].ResourceMetrics().Len(); i++ {
		rm := allMetrics[0].ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				if metric.Name() == "metric1_metric2.sum_output" ||
					metric.Name() == "metric1_metric2.sum_result" {
					outputMetric = metric
				}
			}
		}
	}

	require.False(t, outputMetric.Type() == pmetric.MetricTypeEmpty, "Output metric not found")

	// Should have only 1 data point (latest mode)
	gauge := outputMetric.Gauge()
	assert.Equal(t, 1, gauge.DataPoints().Len(), "Latest mode should produce single output")

	// The sum of the latest values (30 + 40 = 70)
	assert.Equal(t, 70.0, gauge.DataPoints().At(0).DoubleValue())
}

// Helper functions

func createTimeSeriesMetrics(metricName string, numDataPoints int) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	metric := sm.Metrics().AppendEmpty()
	metric.SetName(metricName)
	gauge := metric.SetEmptyGauge()

	baseTime := time.Now()
	for i := 0; i < numDataPoints; i++ {
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetDoubleValue(float64((i + 1) * 10)) // 10, 20, 30, etc.
		dp.SetTimestamp(pcommon.NewTimestampFromTime(baseTime.Add(time.Duration(i) * time.Second)))
	}

	return md
}

func createAlignedMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	// Metric 1
	metric1 := sm.Metrics().AppendEmpty()
	metric1.SetName("metric1")
	gauge1 := metric1.SetEmptyGauge()

	baseTime := time.Now()
	for i := 0; i < 3; i++ {
		dp := gauge1.DataPoints().AppendEmpty()
		dp.SetDoubleValue(float64((i + 1) * 10)) // 10, 20, 30
		dp.SetTimestamp(pcommon.NewTimestampFromTime(baseTime.Add(time.Duration(i) * time.Second)))
	}

	// Metric 2 with slightly offset timestamps (within tolerance)
	metric2 := sm.Metrics().AppendEmpty()
	metric2.SetName("metric2")
	gauge2 := metric2.SetEmptyGauge()

	for i := 0; i < 3; i++ {
		dp := gauge2.DataPoints().AppendEmpty()
		dp.SetDoubleValue(float64((i + 2) * 10)) // 20, 30, 40
		dp.SetTimestamp(pcommon.NewTimestampFromTime(
			baseTime.Add(time.Duration(i)*time.Second + 100*time.Millisecond))) // 100ms offset
	}

	return md
}
