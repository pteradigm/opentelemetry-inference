// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsinferenceprocessor

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processortest"
	"google.golang.org/grpc/codes"

	"github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/internal/metadata"
	"github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/internal/testutil"
	pb "github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/proto/v2"
)

// Define a feature flag for matching attributes
var matchAttributes = featuregate.GlobalRegistry().MustRegister(
	"metricsinferenceprocessor.matchAttributes",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, the metrics inference processor will match attributes when processing metrics"),
)

type TestMetric struct {
	MetricNames  []string
	MetricValues [][]float64
}

type TestMetricIntGauge struct {
	MetricNames  []string
	MetricValues [][]int64
}

type MetricsGenerationTest struct {
	Name       string
	Rules      []Rule
	InMetrics  pmetric.Metrics
	OutMetrics pmetric.Metrics
}

var testCases = []MetricsGenerationTest{
	{
		Name:  "metrics_generation_expect_all",
		Rules: nil,
		InMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2"},
			MetricValues: [][]float64{{100}, {4}},
		}),
		OutMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2"},
			MetricValues: [][]float64{{100}, {4}},
		}),
	},
	{
		Name: "metrics_generation_rule_scale",
		Rules: []Rule{
			{
				ModelName: "scale_5",
				Inputs:    []string{"metric_1"},
				Outputs: []OutputSpec{
					{Name: "metric_1_scaled"},
				},
			},
		},
		InMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2"},
			MetricValues: [][]float64{{100}, {4}},
		}),
		OutMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2", "metric_1_scaled"},
			MetricValues: [][]float64{{100}, {4}, {500}},
		}),
	},
	{
		Name: "metrics_generation_missing_first_metric",
		Rules: []Rule{
			{
				ModelName: "scale_5",
				Inputs:    []string{},
				Outputs: []OutputSpec{
					{Name: "metric_1_scaled"},
				},
			},
		},
		InMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2"},
			MetricValues: [][]float64{{100}, {4}},
		}),
		OutMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2"},
			MetricValues: [][]float64{{100}, {4}},
		}),
	},
	{
		Name: "metrics_generation_rule_calculate_divide",
		Rules: []Rule{
			{
				ModelName: "calculate_divide",
				Inputs:    []string{"metric_1", "metric_2"},
				Outputs: []OutputSpec{
					{Name: "metric_1_calculated_divide"},
				},
			},
		},
		InMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2"},
			MetricValues: [][]float64{{100}, {4}},
		}),
		OutMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2", "metric_1_calculated_divide"},
			MetricValues: [][]float64{{100}, {4}, {25}},
		}),
	},
	{
		Name: "metrics_generation_rule_calculate_multiply",
		Rules: []Rule{
			{
				ModelName: "calculate_multiply",
				Inputs:    []string{"metric_1", "metric_2"},
				Outputs: []OutputSpec{
					{Name: "metric_1_calculated_multiply"},
				},
			},
		},
		InMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2"},
			MetricValues: [][]float64{{100}, {4}},
		}),
		OutMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2", "metric_1_calculated_multiply"},
			MetricValues: [][]float64{{100}, {4}, {400}},
		}),
	},
	{
		Name: "metrics_generation_rule_calculate_add",
		Rules: []Rule{
			{
				ModelName: "calculate_add",
				Inputs:    []string{"metric_1", "metric_2"},
				Outputs: []OutputSpec{
					{Name: "metric_1_calculated_add"},
				},
			},
		},
		InMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2"},
			MetricValues: [][]float64{{100}, {4}},
		}),
		OutMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2", "metric_1_calculated_add"},
			MetricValues: [][]float64{{100}, {4}, {104}},
		}),
	},
	{
		Name: "metrics_generation_rule_calculate_subtract",
		Rules: []Rule{
			{
				ModelName: "calculate_subtract",
				Inputs:    []string{"metric_1", "metric_2"},
				Outputs: []OutputSpec{
					{Name: "metric_1_calculated_subtract"},
				},
			},
		},
		InMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2"},
			MetricValues: [][]float64{{100}, {4}},
		}),
		OutMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2", "metric_1_calculated_subtract"},
			MetricValues: [][]float64{{100}, {4}, {96}},
		}),
	},
	{
		Name: "metrics_generation_rule_calculate_percent",
		Rules: []Rule{
			{
				ModelName: "calculate_percent",
				Inputs:    []string{"metric_1", "metric_2"},
				Outputs: []OutputSpec{
					{Name: "metric_1_calculated_percent"},
				},
			},
		},
		InMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2"},
			MetricValues: [][]float64{{20}, {200}},
		}),
		OutMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2", "metric_1_calculated_percent"},
			MetricValues: [][]float64{{20}, {200}, {10}},
		}),
	},
	{
		Name: "metrics_generation_rule_calculate_missing_2nd_metric",
		Rules: []Rule{
			{
				ModelName: "calculate_multiply",
				Inputs:    []string{"metric_1"},
				Outputs: []OutputSpec{
					{Name: "metric_1_calculated_multiply"},
				},
			},
		},
		InMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2"},
			MetricValues: [][]float64{{100}, {4}},
		}),
		OutMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2"},
			MetricValues: [][]float64{{100}, {4}},
		}),
	},
	{
		Name: "metrics_generation_rule_calculate_divide_op2_zero",
		Rules: []Rule{
			{
				ModelName: "calculate_divide",
				Inputs:    []string{"metric_1", "metric_3"},
				Outputs: []OutputSpec{
					{Name: "metric_1_calculated_divide"},
				},
			},
		},
		InMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_3"},
			MetricValues: [][]float64{{100}, {0}},
		}),
		OutMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_3"},
			MetricValues: [][]float64{{100}, {0}},
		}),
	},
	{
		Name: "metrics_generation_rule_calculate_invalid_operation",
		Rules: []Rule{
			{
				ModelName: "calculate_invalid",
				Inputs:    []string{"metric_1", "metric_2"},
				Outputs: []OutputSpec{
					{Name: "metric_1_calculated_invalid"},
				},
			},
		},
		InMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2"},
			MetricValues: [][]float64{{100}, {0}},
		}),
		OutMetrics: generateTestMetrics(TestMetric{
			MetricNames:  []string{"metric_1", "metric_2"},
			MetricValues: [][]float64{{100}, {0}},
		}),
	},
	{
		Name: "metrics_generation_test_int_gauge_add",
		Rules: []Rule{
			{
				ModelName: "calculate_add",
				Inputs:    []string{"metric_1", "metric_2"},
				Outputs: []OutputSpec{
					{Name: "metric_calculated"},
				},
			},
		},
		InMetrics: generateTestMetricsWithIntDatapoint(TestMetricIntGauge{
			MetricNames:  []string{"metric_1", "metric_2"},
			MetricValues: [][]int64{{100}, {5}},
		}),
		OutMetrics: getOutputForIntGaugeTest(),
	},
}

func TestMetricsInferenceProcessor(t *testing.T) {
	// Test cases that simulate the behavior previously tested with test mode
	tests := []struct {
		name          string
		rules         []Rule
		inputMetrics  pmetric.Metrics
		mockResponses map[string]*pb.ModelInferResponse
		expectedNames []string
	}{
		{
			name:  "metrics_generation_expect_all",
			rules: nil,
			inputMetrics: generateTestMetrics(TestMetric{
				MetricNames:  []string{"metric_1", "metric_2"},
				MetricValues: [][]float64{{100}, {4}},
			}),
			expectedNames: []string{"metric_1", "metric_2"},
		},
		{
			name: "metrics_generation_rule_scale",
			rules: []Rule{
				{
					ModelName: "scale_5",
					Inputs:    []string{"metric_1"},
					OutputPattern: "{output}",
					Outputs: []OutputSpec{
						{Name: "metric_1_scaled"},
					},
				},
			},
			inputMetrics: generateTestMetrics(TestMetric{
				MetricNames:  []string{"metric_1", "metric_2"},
				MetricValues: [][]float64{{100}, {4}},
			}),
			mockResponses: map[string]*pb.ModelInferResponse{
				"scale_5": testutil.CreateMockResponseForScaling("scale_5", 5.0, 100.0),
			},
			expectedNames: []string{"metric_1", "metric_2", "metric_1_scaled"},
		},
		{
			name: "metrics_generation_rule_calculate_divide",
			rules: []Rule{
				{
					ModelName: "calculate_divide",
					Inputs:    []string{"metric_1", "metric_2"},
					OutputPattern: "{output}", // Use explicit output pattern
					Outputs: []OutputSpec{
						{Name: "metric_1_calculated_divide"},
					},
				},
			},
			inputMetrics: generateTestMetrics(TestMetric{
				MetricNames:  []string{"metric_1", "metric_2"},
				MetricValues: [][]float64{{100}, {4}},
			}),
			mockResponses: map[string]*pb.ModelInferResponse{
				"calculate_divide": testutil.CreateMockResponseForCalculation("calculate_divide", 25.0),
			},
			expectedNames: []string{"metric_1", "metric_2", "metric_1_calculated_divide"},
		},
		{
			name: "metrics_generation_rule_calculate_add",
			rules: []Rule{
				{
					ModelName: "calculate_add",
					Inputs:    []string{"metric_1", "metric_2"},
					OutputPattern: "{output}", // Use explicit output pattern
					Outputs: []OutputSpec{
						{Name: "metric_1_calculated_add"},
					},
				},
			},
			inputMetrics: generateTestMetrics(TestMetric{
				MetricNames:  []string{"metric_1", "metric_2"},
				MetricValues: [][]float64{{100}, {4}},
			}),
			mockResponses: map[string]*pb.ModelInferResponse{
				"calculate_add": testutil.CreateMockResponseForCalculation("calculate_add", 104.0),
			},
			expectedNames: []string{"metric_1", "metric_2", "metric_1_calculated_add"},
		},
		{
			name: "metrics_generation_int_gauge_test",
			rules: []Rule{
				{
					ModelName: "calculate_add",
					Inputs:    []string{"metric_1", "metric_2"},
					OutputPattern: "{output}", // Use explicit output pattern
					Outputs: []OutputSpec{
						{Name: "metric_calculated"},
					},
				},
			},
			inputMetrics: generateTestMetricsWithIntDatapoint(TestMetricIntGauge{
				MetricNames:  []string{"metric_1", "metric_2"},
				MetricValues: [][]int64{{100}, {5}},
			}),
			mockResponses: map[string]*pb.ModelInferResponse{
				"calculate_add": testutil.CreateMockResponseForCalculation("calculate_add", 105.0),
			},
			expectedNames: []string{"metric_1", "metric_2", "metric_calculated"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			mockServer := testutil.NewMockInferenceServer()
			mockServer.Start(t)
			defer mockServer.Stop()

			// Configure mock responses
			for modelName, response := range tt.mockResponses {
				mockServer.SetModelResponse(modelName, response)
			}

			// Create processor configuration
			cfg := &Config{
				Rules: tt.rules,
				GRPCClientSettings: GRPCClientSettings{
					Endpoint: mockServer.GetAddress(),
				},
				Timeout: 10,
			}

			// Create consumer and processor
			next := new(consumertest.MetricsSink)
			mp, err := newMetricsProcessor(cfg, next, processortest.NewNopSettings(metadata.Type).Logger)
			assert.NotNil(t, mp)
			assert.NoError(t, err)

			// Test capabilities
			caps := mp.Capabilities()
			assert.True(t, caps.MutatesData)

			// Start processor
			err = mp.Start(context.Background(), nil)
			assert.NoError(t, err)
			defer func() {
				err := mp.Shutdown(context.Background())
				assert.NoError(t, err)
			}()

			// Process metrics
			cErr := mp.ConsumeMetrics(context.Background(), tt.inputMetrics)
			assert.NoError(t, cErr)
			got := next.AllMetrics()

			require.Len(t, got, 1)

			// Verify expected metric names are present
			actualNames := make(map[string]bool)
			for i := 0; i < got[0].ResourceMetrics().Len(); i++ {
				rm := got[0].ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						metric := sm.Metrics().At(k)
						actualNames[metric.Name()] = true
					}
				}
			}

			for _, expectedName := range tt.expectedNames {
				assert.True(t, actualNames[expectedName], "Expected metric '%s' not found", expectedName)
			}
		})
	}
}

func generateTestMetrics(tm TestMetric) pmetric.Metrics {
	md := pmetric.NewMetrics()
	now := time.Now()

	rm := md.ResourceMetrics().AppendEmpty()
	ms := rm.ScopeMetrics().AppendEmpty().Metrics()
	for i, name := range tm.MetricNames {
		m := ms.AppendEmpty()
		m.SetName(name)
		dps := m.SetEmptyGauge().DataPoints()
		dps.EnsureCapacity(len(tm.MetricValues[i]))
		for _, value := range tm.MetricValues[i] {
			dp := dps.AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(10 * time.Second)))
			dp.SetDoubleValue(value)
		}
	}

	return md
}

func generateTestMetricsWithIntDatapoint(tm TestMetricIntGauge) pmetric.Metrics {
	md := pmetric.NewMetrics()
	now := time.Now()

	rm := md.ResourceMetrics().AppendEmpty()
	ms := rm.ScopeMetrics().AppendEmpty().Metrics()
	for i, name := range tm.MetricNames {
		m := ms.AppendEmpty()
		m.SetName(name)
		dps := m.SetEmptyGauge().DataPoints()
		dps.EnsureCapacity(len(tm.MetricValues[i]))
		for _, value := range tm.MetricValues[i] {
			dp := dps.AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(10 * time.Second)))
			dp.SetIntValue(value)
		}
	}

	return md
}

func getOutputForIntGaugeTest() pmetric.Metrics {
	intGaugeOutputMetrics := generateTestMetricsWithIntDatapoint(TestMetricIntGauge{
		MetricNames:  []string{"metric_1", "metric_2"},
		MetricValues: [][]int64{{100}, {5}},
	})
	ilm := intGaugeOutputMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	doubleMetric := ilm.AppendEmpty()
	doubleMetric.SetName("metric_calculated")
	neweDoubleDataPoint := doubleMetric.SetEmptyGauge().DataPoints().AppendEmpty()
	neweDoubleDataPoint.SetDoubleValue(105)

	return intGaugeOutputMetrics
}

type GoldenTestCases struct {
	Name                       string
	TestDir                    string
	MatchAttributesFlagEnabled bool
}

func TestGoldenFileMetrics(t *testing.T) {
	testCases := []GoldenTestCases{
		// Basic inference tests
		{
			Name:    "basic_cpu_prediction",
			TestDir: "basic_inference",
		},
		{
			Name:    "multiple_outputs",
			TestDir: "basic_inference",
		},
		{
			Name:    "no_rules",
			TestDir: "basic_inference",
		},
		// Input metric types tests
		{
			Name:    "sum_gauge_inference",
			TestDir: "input_metric_types",
		},
		{
			Name:    "gauge_only_inference",
			TestDir: "input_metric_types",
		},
		{
			Name:    "sum_only_inference",
			TestDir: "input_metric_types",
		},
		{
			Name:    "multi_attribute_inference",
			TestDir: "input_metric_types",
		},
		// Multi-model tests
		{
			Name:    "multiple_models_same_input",
			TestDir: "multi_model",
		},
		{
			Name:    "multiple_models_different_inputs",
			TestDir: "multi_model",
		},
		{
			Name:    "sequential_processing",
			TestDir: "multi_model",
		},
		{
			Name:    "model_versioning",
			TestDir: "multi_model",
		},
		// Data types tests
		{
			Name:    "float_output",
			TestDir: "data_types",
		},
		{
			Name:    "int_output",
			TestDir: "data_types",
		},
		{
			Name:    "double_output",
			TestDir: "data_types",
		},
		{
			Name:    "mixed_types",
			TestDir: "data_types",
		},
		{
			Name:    "int_gauge_input",
			TestDir: "data_types",
		},
		// Error handling tests
		{
			Name:    "server_error",
			TestDir: "error_handling",
		},
		{
			Name:    "missing_input_metric",
			TestDir: "error_handling",
		},
		{
			Name:    "model_not_ready",
			TestDir: "error_handling",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			// Create mock server for inference
			mockServer := testutil.NewMockInferenceServer()
			mockServer.Start(t)
			defer mockServer.Stop()

			// Set up mock responses based on test case
			switch testCase.Name {
			// Basic inference tests
			case "basic_cpu_prediction":
				mockServer.SetModelResponse("cpu_prediction", testutil.CreateMockResponseForScaling("cpu_prediction", 1.13, 0.75))
			case "multiple_outputs":
				mockServer.SetModelResponse("health_prediction", testutil.CreateMockResponseForMultipleOutputs("health_prediction", []float64{0.92, 1.0}))
			case "no_rules":
				// No mock responses needed for passthrough test
			case "metadata_discovery":
				// Set up metadata for discovery model
				mockServer.SetModelMetadata("discovery_model", &pb.ModelMetadataResponse{
					Name: "discovery_model",
					Outputs: []*pb.ModelMetadataResponse_TensorMetadata{
						{
							Name:     "discovered_prediction",
							Datatype: "FP64",
							Shape:    []int64{1},
						},
						{
							Name:     "discovered_confidence",
							Datatype: "FP64",
							Shape:    []int64{1},
						},
					},
				})
				// Set up inference response
				mockServer.SetModelResponse("discovery_model", &pb.ModelInferResponse{
					ModelName: "discovery_model",
					Outputs: []*pb.ModelInferResponse_InferOutputTensor{
						{
							Name:     "discovered_prediction",
							Datatype: "FP64",
							Shape:    []int64{1},
							Contents: &pb.InferTensorContents{
								Fp64Contents: []float64{0.8475},
							},
						},
						{
							Name:     "discovered_confidence",
							Datatype: "FP64",
							Shape:    []int64{1},
							Contents: &pb.InferTensorContents{
								Fp64Contents: []float64{0.95},
							},
						},
					},
				})
			
			// Input metric types tests
			case "sum_gauge_inference":
				mockServer.SetModelResponse("filesystem_prediction", testutil.CreateMockResponseForFilesystem("filesystem_prediction", 52428800000.0))
			case "gauge_only_inference":
				mockServer.SetModelResponse("utilization_prediction", testutil.CreateMockResponseForScaling("utilization_prediction", 1.2, 0.8))
			case "sum_only_inference":
				mockServer.SetModelResponse("usage_prediction", testutil.CreateMockResponseForDataType("usage_prediction", "INT64", int64(45036953600)))
			case "multi_attribute_inference":
				mockServer.SetModelResponse("capacity_anomaly_detection", testutil.CreateMockResponseForMultipleOutputs("capacity_anomaly_detection", []float64{0.15, 0.0}))
			
			// Multi-model tests
			case "multiple_models_same_input":
				mockServer.SetModelResponse("cpu_anomaly_detector", testutil.CreateMockResponseForScaling("cpu_anomaly_detector", 1.1, 0.75))
				mockServer.SetModelResponse("cpu_predictor", testutil.CreateMockResponseForScaling("cpu_predictor", 1.15, 0.75))
			case "multiple_models_different_inputs":
				mockServer.SetModelResponse("cpu_model", testutil.CreateMockResponseForScaling("cpu_model", 1.1, 0.75))
				mockServer.SetModelResponse("memory_model", testutil.CreateMockResponseForScaling("memory_model", 1.2, 0.45))
				mockServer.SetModelResponse("combined_model", testutil.CreateMockResponseForCalculation("combined_model", 0.89))
			case "sequential_processing":
				mockServer.SetModelResponse("stage1_model", testutil.CreateMockResponseForScaling("stage1_model", 1.0, 0.75))
				mockServer.SetModelResponse("stage2_model", testutil.CreateMockResponseForScaling("stage2_model", 1.0, 0.45))
			case "model_versioning":
				// Set up responses for both model versions
				mockServer.SetModelResponse("cpu_model", testutil.CreateMockResponseForScaling("cpu_model", 1.1, 0.75))
			
			// Data types tests
			case "float_output":
				mockServer.SetModelResponse("float_prediction_model", testutil.CreateMockResponseForDataType("float_prediction_model", "FP32", float32(0.85)))
			case "int_output":
				mockServer.SetModelResponse("int_prediction_model", testutil.CreateMockResponseForDataType("int_prediction_model", "INT32", int32(1)))
			case "double_output":
				mockServer.SetModelResponse("double_prediction_model", testutil.CreateMockResponseForDataType("double_prediction_model", "FP64", float64(0.85)))
			case "mixed_types":
				values := map[string]interface{}{
					"anomaly_score": float32(0.15),
					"alert_level":   int32(1),
					"confidence":    float64(0.95),
				}
				mockServer.SetModelResponse("mixed_types_model", testutil.CreateMockResponseForMixedTypes("mixed_types_model", values))
			case "int_gauge_input":
				mockServer.SetModelResponse("int_input_model", testutil.CreateMockResponseForDataType("int_input_model", "INT64", int64(1100)))
			
			// Error handling tests
			case "server_error":
				mockServer.SetModelError("failing_model", testutil.CreateMockErrorResponse(codes.Internal, "model inference failed"))
			case "missing_input_metric":
				mockServer.SetModelResponse("cpu_prediction", testutil.CreateMockResponseForScaling("cpu_prediction", 1.13, 0.75))
			case "model_not_ready":
				mockServer.SetModelError("not_ready_model", testutil.CreateMockErrorResponse(codes.Unavailable, "model not ready"))
			}

			// Load configuration
			var configPath string
			if testCase.Name == "metadata_discovery" {
				configPath = filepath.Join("testdata", testCase.TestDir, "metadata_discovery_config.yaml")
			} else {
				configPath = filepath.Join("testdata", testCase.TestDir, "config.yaml")
			}
			cm, err := confmaptest.LoadConf(configPath)
			assert.NoError(t, err)

			next := new(consumertest.MetricsSink)
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(fmt.Sprintf("%s/%s", "metricsinference", testCase.Name))
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			// Update the endpoint to use the mock server (or use test endpoint for no_rules test)
			if cfgTyped, ok := cfg.(*Config); ok {
				if testCase.Name != "no_rules" {
					cfgTyped.GRPCClientSettings.Endpoint = mockServer.GetAddress()
				} else {
					// For no_rules test, use the special test endpoint and clear rules
					cfgTyped.GRPCClientSettings.Endpoint = "localhost:12345"
					cfgTyped.Rules = []Rule{}
				}
			}

			mip, err := factory.CreateMetrics(
				context.Background(),
				processortest.NewNopSettings(metadata.Type),
				cfg,
				next,
			)
			assert.NotNil(t, mip)
			assert.NoError(t, err)

			assert.True(t, mip.Capabilities().MutatesData)
			require.NoError(t, mip.Start(context.Background(), nil))

			// Read input metrics
			inputMetrics, err := golden.ReadMetrics(filepath.Join("testdata", testCase.TestDir, "metrics_input.yaml"))
			assert.NoError(t, err)

			// Process metrics
			err = mip.ConsumeMetrics(context.Background(), inputMetrics)
			assert.NoError(t, err)

			got := next.AllMetrics()
			expectedFilePath := filepath.Join("testdata", testCase.TestDir, fmt.Sprintf("%s_%s", testCase.Name, "expected.yaml"))
			
			// Uncomment the following line to generate expected files during development
			// golden.WriteMetrics(t, expectedFilePath, got[0])
			
			expected, err := golden.ReadMetrics(expectedFilePath)
			assert.NoError(t, err)
			assert.Len(t, got, 1)
			
			err = pmetrictest.CompareMetrics(expected, got[0],
				pmetrictest.IgnoreMetricDataPointsOrder(),
				pmetrictest.IgnoreMetricsOrder(),
				pmetrictest.IgnoreStartTimestamp(),
				pmetrictest.IgnoreTimestamp())
			assert.NoError(t, err)
		})
	}
}
