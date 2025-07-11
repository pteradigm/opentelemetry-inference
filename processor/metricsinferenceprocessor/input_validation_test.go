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

func TestInputValidation_CorrectInputs(t *testing.T) {
	// Start mock inference server
	mockServer := testutil.NewMockInferenceServer()
	mockServer.Start(t)
	defer mockServer.Stop()

	// Configure model metadata with expected inputs
	modelMetadata := &pb.ModelMetadataResponse{
		Name:     "test-validation-model",
		Platform: "test",
		Inputs: []*pb.ModelMetadataResponse_TensorMetadata{
			{
				Name:     "input_tensor",
				Datatype: "FP64",
				Shape:    []int64{-1}, // Variable size 1D tensor
			},
		},
		Outputs: []*pb.ModelMetadataResponse_TensorMetadata{
			{
				Name:     "output_tensor",
				Datatype: "FP64",
				Shape:    []int64{-1},
			},
		},
	}
	mockServer.SetModelMetadata("test-validation-model", modelMetadata)

	// Configure mock response
	mockResponse := &pb.ModelInferResponse{
		ModelName:    "test-validation-model",
		ModelVersion: "1",
		Id:           "test-request",
		Outputs: []*pb.ModelInferResponse_InferOutputTensor{
			{
				Name:     "output_tensor",
				Datatype: "FP64",
				Shape:    []int64{1},
				Contents: &pb.InferTensorContents{
					Fp64Contents: []float64{42.0},
				},
			},
		},
	}
	mockServer.SetModelResponse("test-validation-model", mockResponse)

	// Create processor config with correct input
	cfg := &Config{
		GRPCClientSettings: GRPCClientSettings{
			Endpoint: mockServer.Endpoint(),
		},
		Rules: []Rule{
			{
				ModelName: "test-validation-model",
				Inputs:    []string{"test.metric"}, // Single input as expected
				Outputs: []OutputSpec{
					{Name: "test.output"},
				},
			},
		},
	}

	// Create processor
	sink := &consumertest.MetricsSink{}
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

	// Create test metrics with correct type (FP64 compatible)
	metrics := createValidationTestMetrics("test.metric", pmetric.NumberDataPointValueTypeDouble, 1)

	// Process metrics - should succeed
	err = processor.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)

	// Verify output was created
	allMetrics := sink.AllMetrics()
	require.Len(t, allMetrics, 1)
}

func TestInputValidation_WrongInputCount(t *testing.T) {
	// Start mock inference server
	mockServer := testutil.NewMockInferenceServer()
	mockServer.Start(t)
	defer mockServer.Stop()

	// Configure model metadata expecting 2 inputs
	modelMetadata := &pb.ModelMetadataResponse{
		Name:     "two-input-model",
		Platform: "test",
		Inputs: []*pb.ModelMetadataResponse_TensorMetadata{
			{
				Name:     "input1",
				Datatype: "FP64",
				Shape:    []int64{1},
			},
			{
				Name:     "input2",
				Datatype: "FP64",
				Shape:    []int64{1},
			},
		},
		Outputs: []*pb.ModelMetadataResponse_TensorMetadata{
			{
				Name:     "output",
				Datatype: "FP64",
				Shape:    []int64{1},
			},
		},
	}
	mockServer.SetModelMetadata("two-input-model", modelMetadata)

	// Create processor config with only 1 input (should fail)
	cfg := &Config{
		GRPCClientSettings: GRPCClientSettings{
			Endpoint: mockServer.Endpoint(),
		},
		Rules: []Rule{
			{
				ModelName: "two-input-model",
				Inputs:    []string{"test.metric"}, // Only 1 input, but model expects 2
				Outputs: []OutputSpec{
					{Name: "test.output"},
				},
			},
		},
	}

	// Create processor
	sink := &consumertest.MetricsSink{}
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
	metrics := createValidationTestMetrics("test.metric", pmetric.NumberDataPointValueTypeDouble, 1)

	// Process metrics - should continue without inference due to validation failure
	err = processor.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err) // Processing continues, but no inference performed

	// Verify no inference output was created (only original metric passed through)
	allMetrics := sink.AllMetrics()
	require.Len(t, allMetrics, 1)

	// Should only have the original metric, no inference output
	outputMetrics := allMetrics[0]
	rm := outputMetrics.ResourceMetrics().At(0)
	sm := rm.ScopeMetrics().At(0)
	require.Equal(t, 1, sm.Metrics().Len()) // Only original metric
	assert.Equal(t, "test.metric", sm.Metrics().At(0).Name())
}

func TestInputValidation_IncompatibleDataType(t *testing.T) {
	// Start mock inference server
	mockServer := testutil.NewMockInferenceServer()
	mockServer.Start(t)
	defer mockServer.Stop()

	// Configure model metadata expecting specific data type
	modelMetadata := &pb.ModelMetadataResponse{
		Name:     "type-strict-model",
		Platform: "test",
		Inputs: []*pb.ModelMetadataResponse_TensorMetadata{
			{
				Name:     "bool_input",
				Datatype: "BOOL", // Expects boolean
				Shape:    []int64{1},
			},
		},
		Outputs: []*pb.ModelMetadataResponse_TensorMetadata{
			{
				Name:     "output",
				Datatype: "BOOL",
				Shape:    []int64{1},
			},
		},
	}
	mockServer.SetModelMetadata("type-strict-model", modelMetadata)

	// Create processor config
	cfg := &Config{
		GRPCClientSettings: GRPCClientSettings{
			Endpoint: mockServer.Endpoint(),
		},
		Rules: []Rule{
			{
				ModelName: "type-strict-model",
				Inputs:    []string{"test.metric"},
				Outputs: []OutputSpec{
					{Name: "test.output"},
				},
			},
		},
	}

	// Create processor
	sink := &consumertest.MetricsSink{}
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

	// Create test metrics with double type (should be compatible with BOOL via INT64)
	metrics := createValidationTestMetrics("test.metric", pmetric.NumberDataPointValueTypeInt, 1)

	// Process metrics - should succeed (INT64 is compatible with BOOL)
	err = processor.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)
}

func TestInputValidation_WrongShape(t *testing.T) {
	// Start mock inference server
	mockServer := testutil.NewMockInferenceServer()
	mockServer.Start(t)
	defer mockServer.Stop()

	// Configure model metadata expecting scalar input
	modelMetadata := &pb.ModelMetadataResponse{
		Name:     "scalar-model",
		Platform: "test",
		Inputs: []*pb.ModelMetadataResponse_TensorMetadata{
			{
				Name:     "scalar_input",
				Datatype: "FP64",
				Shape:    []int64{}, // Scalar (no dimensions)
			},
		},
		Outputs: []*pb.ModelMetadataResponse_TensorMetadata{
			{
				Name:     "output",
				Datatype: "FP64",
				Shape:    []int64{},
			},
		},
	}
	mockServer.SetModelMetadata("scalar-model", modelMetadata)

	// Create processor config
	cfg := &Config{
		GRPCClientSettings: GRPCClientSettings{
			Endpoint: mockServer.Endpoint(),
		},
		Rules: []Rule{
			{
				ModelName: "scalar-model",
				Inputs:    []string{"test.metric"},
				Outputs: []OutputSpec{
					{Name: "test.output"},
				},
			},
		},
	}

	// Create processor
	sink := &consumertest.MetricsSink{}
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

	// Create test metrics with multiple data points (should fail scalar validation)
	metrics := createValidationTestMetrics("test.metric", pmetric.NumberDataPointValueTypeDouble, 3)

	// Process metrics - should continue without inference due to validation failure
	err = processor.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)

	// Verify no inference output created
	allMetrics := sink.AllMetrics()
	require.Len(t, allMetrics, 1)
	outputMetrics := allMetrics[0]
	rm := outputMetrics.ResourceMetrics().At(0)
	sm := rm.ScopeMetrics().At(0)
	require.Equal(t, 1, sm.Metrics().Len()) // Only original metric
}

func TestInputValidation_NoMetadata(t *testing.T) {
	// Start mock inference server without metadata
	mockServer := testutil.NewMockInferenceServer()
	mockServer.Start(t)
	defer mockServer.Stop()

	// Don't set any metadata - should skip validation

	// Configure mock response for successful inference
	mockResponse := &pb.ModelInferResponse{
		ModelName:    "no-metadata-model",
		ModelVersion: "1",
		Id:           "test-request",
		Outputs: []*pb.ModelInferResponse_InferOutputTensor{
			{
				Name:     "output",
				Datatype: "FP64",
				Shape:    []int64{1},
				Contents: &pb.InferTensorContents{
					Fp64Contents: []float64{42.0},
				},
			},
		},
	}
	mockServer.SetModelResponse("no-metadata-model", mockResponse)

	// Create processor config
	cfg := &Config{
		GRPCClientSettings: GRPCClientSettings{
			Endpoint: mockServer.Endpoint(),
		},
		Rules: []Rule{
			{
				ModelName: "no-metadata-model",
				Inputs:    []string{"test.metric"},
				Outputs: []OutputSpec{
					{Name: "test.output"},
				},
			},
		},
	}

	// Create processor
	sink := &consumertest.MetricsSink{}
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
	metrics := createValidationTestMetrics("test.metric", pmetric.NumberDataPointValueTypeDouble, 1)

	// Process metrics - should succeed (no validation when no metadata)
	err = processor.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)

	// Verify inference output was created
	allMetrics := sink.AllMetrics()
	require.Len(t, allMetrics, 1)
	outputMetrics := allMetrics[0]
	rm := outputMetrics.ResourceMetrics().At(0)
	sm := rm.ScopeMetrics().At(0)
	require.Equal(t, 2, sm.Metrics().Len()) // Original + inference output
}

// Helper function to create test metrics with specified type and data point count
func createValidationTestMetrics(metricName string, valueType pmetric.NumberDataPointValueType, dataPointCount int) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	metric := sm.Metrics().AppendEmpty()
	metric.SetName(metricName)
	gauge := metric.SetEmptyGauge()

	for i := 0; i < dataPointCount; i++ {
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

		if valueType == pmetric.NumberDataPointValueTypeInt {
			dp.SetIntValue(int64(i + 1))
		} else {
			dp.SetDoubleValue(float64(i + 1))
		}

		// Add some attributes for multi-datapoint tests
		if dataPointCount > 1 {
			dp.Attributes().PutStr("index", string(rune('a'+i)))
		}
	}

	return md
}
