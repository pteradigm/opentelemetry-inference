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
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/codes"

	"github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/internal/testutil"
	pb "github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/proto/v2"
)

func TestMetricsInferenceProcessorWithMockServer(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		inputMetrics  pmetric.Metrics
		mockResponses map[string]interface{} // model name -> response or error
		expectedCount int                    // expected number of metrics in output
		expectedNames []string               // expected metric names in output
	}{
		{
			name: "no_rules_passthrough",
			config: &Config{
				Rules: []Rule{},
			},
			inputMetrics: testutil.GenerateTestMetrics(testutil.TestMetric{
				MetricNames:  []string{"metric_1", "metric_2"},
				MetricValues: [][]float64{{100}, {4}},
			}),
			expectedCount: 2,
			expectedNames: []string{"metric_1", "metric_2"},
		},
		{
			name: "scale_rule_with_mock_response",
			config: &Config{
				Rules: []Rule{
					{
						ModelName: "scale_5",
						Inputs:    []string{"metric_1"},
						OutputPattern: "{output}",
						Outputs: []OutputSpec{
							{Name: "metric_1_scaled"},
						},
					},
				},
			},
			inputMetrics: testutil.GenerateTestMetrics(testutil.TestMetric{
				MetricNames:  []string{"metric_1", "metric_2"},
				MetricValues: [][]float64{{100}, {4}},
			}),
			mockResponses: map[string]interface{}{
				"scale_5": testutil.CreateMockResponseForScaling("scale_5", 5.0, 100.0),
			},
			expectedCount: 3,
			expectedNames: []string{"metric_1", "metric_2", "metric_1_scaled"},
		},
		{
			name: "calculate_rule_with_mock_response",
			config: &Config{
				Rules: []Rule{
					{
						ModelName: "calculate_add",
						Inputs:    []string{"metric_1", "metric_2"},
						OutputPattern: "{output}",
						Outputs: []OutputSpec{
							{Name: "metric_calculated"},
						},
					},
				},
			},
			inputMetrics: testutil.GenerateTestMetrics(testutil.TestMetric{
				MetricNames:  []string{"metric_1", "metric_2"},
				MetricValues: [][]float64{{100}, {4}},
			}),
			mockResponses: map[string]interface{}{
				"calculate_add": testutil.CreateMockResponseForCalculation("calculate_add", 104.0),
			},
			expectedCount: 3,
			expectedNames: []string{"metric_1", "metric_2", "metric_calculated"},
		},
		{
			name: "inference_server_error",
			config: &Config{
				Rules: []Rule{
					{
						ModelName: "failing_model",
						Inputs:    []string{"metric_1"},
						Outputs: []OutputSpec{
							{Name: "metric_failed"},
						},
					},
				},
			},
			inputMetrics: testutil.GenerateTestMetrics(testutil.TestMetric{
				MetricNames:  []string{"metric_1"},
				MetricValues: [][]float64{{100}},
			}),
			mockResponses: map[string]interface{}{
				"failing_model": testutil.CreateMockErrorResponse(codes.Internal, "model inference failed"),
			},
			expectedCount: 1, // Only original metric, no new metric due to error
			expectedNames: []string{"metric_1"},
		},
		{
			name: "multiple_models",
			config: &Config{
				Rules: []Rule{
					{
						ModelName: "scale_2",
						Inputs:    []string{"metric_1"},
						OutputPattern: "{output}",
						Outputs: []OutputSpec{
							{Name: "metric_1_scaled"},
						},
					},
					{
						ModelName: "calculate_add",
						Inputs:    []string{"metric_1", "metric_2"},
						OutputPattern: "{output}",
						Outputs: []OutputSpec{
							{Name: "metric_calculated"},
						},
					},
				},
			},
			inputMetrics: testutil.GenerateTestMetrics(testutil.TestMetric{
				MetricNames:  []string{"metric_1", "metric_2"},
				MetricValues: [][]float64{{50}, {25}},
			}),
			mockResponses: map[string]interface{}{
				"scale_2":       testutil.CreateMockResponseForScaling("scale_2", 2.0, 50.0),
				"calculate_add": testutil.CreateMockResponseForCalculation("calculate_add", 75.0),
			},
			expectedCount: 4,
			expectedNames: []string{"metric_1", "metric_2", "metric_1_scaled", "metric_calculated"},
		},
		{
			name: "sum_two_metrics",
			config: &Config{
				Rules: []Rule{
					{
						ModelName: "sum_model",
						Inputs:    []string{"metric_a", "metric_b"},
						OutputPattern: "{output}",
						Outputs: []OutputSpec{
							{Name: "metric_sum"},
						},
					},
				},
			},
			inputMetrics: testutil.GenerateTestMetrics(testutil.TestMetric{
				MetricNames:  []string{"metric_a", "metric_b"},
				MetricValues: [][]float64{{10.5}, {7.3}},
			}),
			mockResponses: map[string]interface{}{
				"sum_model": testutil.CreateMockResponseForCalculation("sum_model", 17.8), // 10.5 + 7.3
			},
			expectedCount: 3,
			expectedNames: []string{"metric_a", "metric_b", "metric_sum"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			mockServer := testutil.NewMockInferenceServer()
			mockServer.Start(t)
			defer mockServer.Stop()

			// Configure mock responses/errors
			for modelName, response := range tt.mockResponses {
				switch resp := response.(type) {
				case error:
					mockServer.SetModelError(modelName, resp)
				default:
					mockServer.SetModelResponse(modelName, response.(*pb.ModelInferResponse))
				}
			}

			// Configure processor with mock server endpoint
			tt.config.GRPCClientSettings = GRPCClientSettings{
				Endpoint: mockServer.GetAddress(),
			}
			tt.config.Timeout = 5 // 5 second timeout for tests

			// Create consumer to capture output
			sink := &consumertest.MetricsSink{}
			logger := zaptest.NewLogger(t)

			// Create processor using standard constructor (not test mode)
			processor, err := newMetricsProcessor(tt.config, sink, logger)
			require.NoError(t, err)

			// Start processor (establishes gRPC connection)
			err = processor.Start(context.Background(), nil)
			require.NoError(t, err)
			defer func() {
				err := processor.Shutdown(context.Background())
				assert.NoError(t, err)
			}()

			// Process metrics
			err = processor.ConsumeMetrics(context.Background(), tt.inputMetrics)
			require.NoError(t, err)

			// Verify results
			require.Len(t, sink.AllMetrics(), 1, "Expected exactly one metrics batch")
			outputMetrics := sink.AllMetrics()[0]

			// Count total metrics across all resource/scope metrics
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
					}
				}
			}

			assert.Equal(t, tt.expectedCount, totalMetrics, "Unexpected number of metrics")

			// Verify expected metric names are present
			for _, expectedName := range tt.expectedNames {
				assert.True(t, actualNames[expectedName], "Expected metric '%s' not found", expectedName)
			}

			// Verify mock server was called appropriately
			requests := mockServer.GetRequests()
			expectedRequestCount := 0
			for range tt.mockResponses {
				expectedRequestCount++
			}
			assert.Len(t, requests, expectedRequestCount, "Unexpected number of inference requests")

			// Verify server health check was called
			assert.Greater(t, mockServer.GetServerLiveCalls(), 0, "ServerLive should have been called during Start()")
		})
	}
}

func TestMetricsInferenceProcessorStartupFailure(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectedError string
	}{
		{
			name: "invalid_endpoint",
			config: &Config{
				GRPCClientSettings: GRPCClientSettings{
					Endpoint: "invalid:endpoint:format",
				},
				Rules: []Rule{
					{
						ModelName: "test_model",
						Inputs:    []string{"metric_1"},
						Outputs: []OutputSpec{
							{Name: "output_metric"},
						},
					},
				},
			},
			expectedError: "inference server health check failed",
		},
		{
			name: "unreachable_endpoint",
			config: &Config{
				GRPCClientSettings: GRPCClientSettings{
					Endpoint: "localhost:99999", // Assuming this port is not in use
				},
				Rules: []Rule{
					{
						ModelName: "test_model",
						Inputs:    []string{"metric_1"},
						Outputs: []OutputSpec{
							{Name: "output_metric"},
						},
					},
				},
				Timeout: 1, // Short timeout for faster test
			},
			expectedError: "inference server health check failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := &consumertest.MetricsSink{}
			logger := zaptest.NewLogger(t)

			processor, err := newMetricsProcessor(tt.config, sink, logger)
			require.NoError(t, err)

			// Start should fail
			err = processor.Start(context.Background(), nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestMetricsInferenceProcessorConfiguration(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		inputMetrics pmetric.Metrics
		setupMock    func(*testutil.MockInferenceServer)
		verifyMock   func(*testing.T, *testutil.MockInferenceServer)
	}{
		{
			name: "with_parameters",
			config: &Config{
				Rules: []Rule{
					{
						ModelName: "parameterized_model",
						Inputs:    []string{"metric_1"},
						Outputs: []OutputSpec{
							{Name: "output_metric"},
						},
						Parameters: map[string]interface{}{
							"threshold": 0.5,
							"enabled":   true,
							"mode":      "prediction",
						},
					},
				},
			},
			inputMetrics: testutil.GenerateTestMetrics(testutil.TestMetric{
				MetricNames:  []string{"metric_1"},
				MetricValues: [][]float64{{42.0}},
			}),
			setupMock: func(mock *testutil.MockInferenceServer) {
				mock.SetModelResponse("parameterized_model", testutil.CreateMockResponseForCalculation("parameterized_model", 1.0))
			},
			verifyMock: func(t *testing.T, mock *testutil.MockInferenceServer) {
				requests := mock.GetRequests()
				require.Len(t, requests, 1)

				req := requests[0]
				assert.Equal(t, "parameterized_model", req.ModelName)

				// Verify parameters were sent
				require.NotNil(t, req.Parameters)
				assert.Contains(t, req.Parameters, "threshold")
				assert.Contains(t, req.Parameters, "enabled")
				assert.Contains(t, req.Parameters, "mode")
			},
		},
		{
			name: "with_model_version",
			config: &Config{
				Rules: []Rule{
					{
						ModelName:    "versioned_model",
						ModelVersion: "v2.1",
						Inputs:       []string{"metric_1"},
						Outputs: []OutputSpec{
							{Name: "output_metric"},
						},
					},
				},
			},
			inputMetrics: testutil.GenerateTestMetrics(testutil.TestMetric{
				MetricNames:  []string{"metric_1"},
				MetricValues: [][]float64{{100.0}},
			}),
			setupMock: func(mock *testutil.MockInferenceServer) {
				mock.SetModelResponse("versioned_model", testutil.CreateMockResponseForCalculation("versioned_model", 2.0))
			},
			verifyMock: func(t *testing.T, mock *testutil.MockInferenceServer) {
				requests := mock.GetRequests()
				require.Len(t, requests, 1)

				req := requests[0]
				assert.Equal(t, "versioned_model", req.ModelName)
				assert.Equal(t, "v2.1", req.ModelVersion)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			mockServer := testutil.NewMockInferenceServer()
			mockServer.Start(t)
			defer mockServer.Stop()

			// Setup mock responses
			tt.setupMock(mockServer)

			// Configure processor
			tt.config.GRPCClientSettings = GRPCClientSettings{
				Endpoint: mockServer.GetAddress(),
			}

			sink := &consumertest.MetricsSink{}
			logger := zaptest.NewLogger(t)

			processor, err := newMetricsProcessor(tt.config, sink, logger)
			require.NoError(t, err)

			err = processor.Start(context.Background(), nil)
			require.NoError(t, err)
			defer func() {
				err := processor.Shutdown(context.Background())
				assert.NoError(t, err)
			}()

			// Process metrics
			err = processor.ConsumeMetrics(context.Background(), tt.inputMetrics)
			require.NoError(t, err)

			// Verify mock interactions
			tt.verifyMock(t, mockServer)
		})
	}
}
