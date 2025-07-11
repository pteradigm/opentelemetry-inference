// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package metricsinferenceprocessor

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processortest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/internal/metadata"
	"github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/internal/testutil"
	pb "github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/proto/v2"
)

// TestMLServerIntegration tests the processor against a real MLServer instance
// This test requires MLServer to be running on localhost:9081 (see testenv/docker-compose.yml)
//
// To run this test:
// 1. cd testenv && docker-compose up -d
// 2. go test -tags=integration -v -run TestMLServerIntegration
// 3. cd testenv && docker-compose down
func TestMLServerIntegration(t *testing.T) {
	// Check if integration testing is enabled
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=1 to run.")
	}

	// MLServer endpoint (adjust if needed)
	endpoint := "localhost:9081"

	// First, verify MLServer is accessible
	t.Run("verify_mlserver_connection", func(t *testing.T) {
		conn, err := grpc.Dial(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		defer conn.Close()

		client := pb.NewGRPCInferenceServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Test server health
		_, err = client.ServerLive(ctx, &pb.ServerLiveRequest{})
		require.NoError(t, err, "MLServer should be running on %s. Start it with: cd testenv && docker-compose up -d", endpoint)

		// Test server readiness
		_, err = client.ServerReady(ctx, &pb.ServerReadyRequest{})
		require.NoError(t, err)

		// Test model readiness
		_, err = client.ModelReady(ctx, &pb.ModelReadyRequest{Name: "simple-scaler"})
		require.NoError(t, err, "Model 'simple-scaler' should be ready")

		// Test sum model readiness
		_, err = client.ModelReady(ctx, &pb.ModelReadyRequest{Name: "simple-sum"})
		require.NoError(t, err, "Model 'simple-sum' should be ready")
	})

	t.Run("processor_with_real_mlserver", func(t *testing.T) {
		// Configure processor to use real MLServer
		cfg := &Config{
			GRPCClientSettings: GRPCClientSettings{
				Endpoint: endpoint,
			},
			Rules: []Rule{
				{
					ModelName: "simple-scaler",
					Inputs:    []string{"test.metric"},
					// No outputs configured - will discover from model metadata
				},
			},
			Timeout: 30,
		}

		// Create consumer and processor
		sink := &consumertest.MetricsSink{}
		processor, err := newMetricsProcessor(cfg, sink, processortest.NewNopSettings(metadata.Type).Logger)
		require.NoError(t, err)

		// Start processor (connects to MLServer)
		err = processor.Start(context.Background(), nil)
		require.NoError(t, err)
		defer func() {
			err := processor.Shutdown(context.Background())
			assert.NoError(t, err)
		}()

		// Create test metrics
		inputMetrics := testutil.GenerateTestMetrics(testutil.TestMetric{
			MetricNames:  []string{"test.metric"},
			MetricValues: [][]float64{{5.0}}, // Should be scaled to 10.0
		})

		// Process metrics through the processor
		err = processor.ConsumeMetrics(context.Background(), inputMetrics)
		require.NoError(t, err)

		// Verify results
		require.Len(t, sink.AllMetrics(), 1, "Expected exactly one metrics batch")
		outputMetrics := sink.AllMetrics()[0]

		// Count metrics and verify names
		totalMetrics := 0
		actualNames := make(map[string]bool)

		for i := 0; i < outputMetrics.ResourceMetrics().Len(); i++ {
			rm := outputMetrics.ResourceMetrics().At(i)
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				sm := rm.ScopeMetrics().At(j)
				totalMetrics += sm.Metrics().Len()

				for k := 0; k < sm.Metrics().Len(); k++ {
					metric := sm.Metrics().At(k)
					actualNames[metric.Name()] = true

					// If this is the scaled metric, verify the value
					if metric.Name() == "test.metric.scaled_result" {
						require.Equal(t, pmetric.MetricTypeGauge, metric.Type())
						dataPoints := metric.Gauge().DataPoints()
						require.Greater(t, dataPoints.Len(), 0)

						// The MLServer should scale 5.0 to 10.0 (scale factor = 2.0)
						actualValue := dataPoints.At(0).DoubleValue()
						assert.Equal(t, 10.0, actualValue, "Expected scaled value to be 10.0 (5.0 * 2.0)")
					}
				}
			}
		}

		// Verify we have both original and scaled metrics
		assert.Equal(t, 2, totalMetrics, "Expected 2 metrics (original + scaled)")
		assert.True(t, actualNames["test.metric"], "Original metric should be present")
		assert.True(t, actualNames["test_metric.scaled_result"], "Scaled metric should be present")
	})

	t.Run("multiple_metrics_with_mlserver", func(t *testing.T) {
		// Test with multiple metrics - this should fail validation since simple-scaler expects only 1 input
		// but the rule tries to send 2 inputs (cpu.usage, memory.usage)
		cfg := &Config{
			GRPCClientSettings: GRPCClientSettings{
				Endpoint: endpoint,
			},
			Rules: []Rule{
				{
					ModelName: "simple-scaler",
					Inputs:    []string{"cpu.usage", "memory.usage"},
					// No outputs configured - will discover from model metadata
				},
			},
			Timeout: 30,
		}

		sink := &consumertest.MetricsSink{}
		processor, err := newMetricsProcessor(cfg, sink, processortest.NewNopSettings(metadata.Type).Logger)
		require.NoError(t, err)

		err = processor.Start(context.Background(), nil)
		require.NoError(t, err)
		defer func() {
			err := processor.Shutdown(context.Background())
			assert.NoError(t, err)
		}()

		// Create test metrics with multiple values
		inputMetrics := testutil.GenerateTestMetrics(testutil.TestMetric{
			MetricNames:  []string{"cpu.usage", "memory.usage", "disk.usage"},
			MetricValues: [][]float64{{25.0}, {50.0}, {75.0}},
		})

		err = processor.ConsumeMetrics(context.Background(), inputMetrics)
		require.NoError(t, err)

		// Verify results
		require.Len(t, sink.AllMetrics(), 1)
		outputMetrics := sink.AllMetrics()[0]

		actualNames := make(map[string]bool)
		totalMetrics := 0
		for i := 0; i < outputMetrics.ResourceMetrics().Len(); i++ {
			rm := outputMetrics.ResourceMetrics().At(i)
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				sm := rm.ScopeMetrics().At(j)
				totalMetrics += sm.Metrics().Len()
				for k := 0; k < sm.Metrics().Len(); k++ {
					metric := sm.Metrics().At(k)
					actualNames[metric.Name()] = true
				}
			}
		}

		// Should have only original metrics (no scaled output since validation failed)
		assert.Equal(t, 3, totalMetrics, "Should only have original metrics when validation fails")
		assert.True(t, actualNames["cpu.usage"], "Original CPU metric should be present")
		assert.True(t, actualNames["memory.usage"], "Original memory metric should be present")
		assert.True(t, actualNames["disk.usage"], "Original disk metric should be present")
		// No scaled metric should be present due to input validation failure
		assert.False(t, actualNames["cpu_usage_memory_usage.scaled_result"], "Scaled metric should not be present due to validation failure")
	})

	t.Run("error_handling_with_mlserver", func(t *testing.T) {
		// Test with non-existent model
		cfg := &Config{
			GRPCClientSettings: GRPCClientSettings{
				Endpoint: endpoint,
			},
			Rules: []Rule{
				{
					ModelName: "non-existent-model", // This should cause an error
					Inputs:    []string{"test.metric"},
					// No outputs configured - metadata discovery will also fail
				},
			},
			Timeout: 30,
		}

		sink := &consumertest.MetricsSink{}
		processor, err := newMetricsProcessor(cfg, sink, processortest.NewNopSettings(metadata.Type).Logger)
		require.NoError(t, err)

		err = processor.Start(context.Background(), nil)
		require.NoError(t, err)
		defer func() {
			err := processor.Shutdown(context.Background())
			assert.NoError(t, err)
		}()

		inputMetrics := testutil.GenerateTestMetrics(testutil.TestMetric{
			MetricNames:  []string{"test.metric"},
			MetricValues: [][]float64{{1.0}},
		})

		// This should not fail, but the inference will fail and be logged
		err = processor.ConsumeMetrics(context.Background(), inputMetrics)
		require.NoError(t, err)

		// Original metrics should still be passed through
		require.Len(t, sink.AllMetrics(), 1)
		outputMetrics := sink.AllMetrics()[0]

		// Should only have the original metric (no inference result due to error)
		totalMetrics := 0
		for i := 0; i < outputMetrics.ResourceMetrics().Len(); i++ {
			rm := outputMetrics.ResourceMetrics().At(i)
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				sm := rm.ScopeMetrics().At(j)
				totalMetrics += sm.Metrics().Len()
			}
		}
		assert.Equal(t, 1, totalMetrics, "Should only have original metric when inference fails")
	})

	t.Run("sum_model_with_mlserver", func(t *testing.T) {
		// Test sum model that adds two metrics
		cfg := &Config{
			GRPCClientSettings: GRPCClientSettings{
				Endpoint: endpoint,
			},
			Rules: []Rule{
				{
					ModelName: "simple-sum",
					Inputs:    []string{"metric.a", "metric.b"},
					// No outputs configured - will discover from model metadata
				},
			},
			Timeout: 30,
		}

		sink := &consumertest.MetricsSink{}
		processor, err := newMetricsProcessor(cfg, sink, processortest.NewNopSettings(metadata.Type).Logger)
		require.NoError(t, err)

		err = processor.Start(context.Background(), nil)
		require.NoError(t, err)
		defer func() {
			err := processor.Shutdown(context.Background())
			assert.NoError(t, err)
		}()

		// Create test metrics with two values to sum
		inputMetrics := testutil.GenerateTestMetrics(testutil.TestMetric{
			MetricNames:  []string{"metric.a", "metric.b", "metric.c"},
			MetricValues: [][]float64{{10.5}, {7.3}, {100.0}}, // Sum of first two should be 17.8
		})

		err = processor.ConsumeMetrics(context.Background(), inputMetrics)
		require.NoError(t, err)

		// Verify results
		require.Len(t, sink.AllMetrics(), 1)
		outputMetrics := sink.AllMetrics()[0]

		actualNames := make(map[string]bool)
		var sumValue float64
		totalMetrics := 0

		for i := 0; i < outputMetrics.ResourceMetrics().Len(); i++ {
			rm := outputMetrics.ResourceMetrics().At(i)
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				sm := rm.ScopeMetrics().At(j)
				totalMetrics += sm.Metrics().Len()

				for k := 0; k < sm.Metrics().Len(); k++ {
					metric := sm.Metrics().At(k)
					actualNames[metric.Name()] = true

					// Check if this is the sum metric and get its value
					if metric.Name() == "a_b.sum_result" {
						require.Equal(t, pmetric.MetricTypeGauge, metric.Type())
						dataPoints := metric.Gauge().DataPoints()
						require.Greater(t, dataPoints.Len(), 0)
						sumValue = dataPoints.At(0).DoubleValue()
					}
				}
			}
		}

		// Should have original metrics plus the sum
		assert.Equal(t, 4, totalMetrics, "Expected 4 metrics (3 original + 1 sum)")
		assert.True(t, actualNames["metric.a"], "Original metric.a should be present")
		assert.True(t, actualNames["metric.b"], "Original metric.b should be present")
		assert.True(t, actualNames["metric.c"], "Original metric.c should be present")
		assert.True(t, actualNames["a_b.sum_result"], "Sum metric should be present")

		// Verify the sum value (10.5 + 7.3 = 17.8)
		assert.InDelta(t, 17.8, sumValue, 0.001, "Sum should be 17.8 (10.5 + 7.3)")
	})
}
