// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/proto/v2"
)

// MockInferenceServer implements the GRPCInferenceService for testing
type MockInferenceServer struct {
	pb.UnimplementedGRPCInferenceServiceServer

	// Configuration
	responses     map[string]*pb.ModelInferResponse
	metadata      map[string]*pb.ModelMetadataResponse
	errors        map[string]error

	// Request tracking
	requests        []*pb.ModelInferRequest
	serverLiveCalls int

	// Server management
	server   *grpc.Server
	listener net.Listener
	address  string
}

// NewMockInferenceServer creates a new mock inference server
func NewMockInferenceServer() *MockInferenceServer {
	return &MockInferenceServer{
		responses: make(map[string]*pb.ModelInferResponse),
		metadata:  make(map[string]*pb.ModelMetadataResponse),
		errors:    make(map[string]error),
		requests:  make([]*pb.ModelInferRequest, 0),
	}
}

// SetModelResponse configures the response for a specific model
func (m *MockInferenceServer) SetModelResponse(modelName string, response *pb.ModelInferResponse) {
	m.responses[modelName] = response
}

// SetModelError configures an error response for a specific model
func (m *MockInferenceServer) SetModelError(modelName string, err error) {
	m.errors[modelName] = err
}

// SetModelMetadata configures the metadata response for a specific model
func (m *MockInferenceServer) SetModelMetadata(modelName string, metadata *pb.ModelMetadataResponse) {
	m.metadata[modelName] = metadata
}

// Endpoint returns the server endpoint address
func (m *MockInferenceServer) Endpoint() string {
	return m.address
}

// GetRequests returns all received inference requests
func (m *MockInferenceServer) GetRequests() []*pb.ModelInferRequest {
	return m.requests
}

// GetServerLiveCalls returns the number of ServerLive calls received
func (m *MockInferenceServer) GetServerLiveCalls() int {
	return m.serverLiveCalls
}

// GetAddress returns the server address
func (m *MockInferenceServer) GetAddress() string {
	return m.address
}

// Start starts the mock server on a random available port
func (m *MockInferenceServer) Start(t *testing.T) {
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	m.listener = lis
	m.address = lis.Addr().String()

	m.server = grpc.NewServer()
	pb.RegisterGRPCInferenceServiceServer(m.server, m)

	go func() {
		if err := m.server.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			t.Errorf("Failed to serve: %v", err)
		}
	}()

	// Wait for server to be ready
	time.Sleep(10 * time.Millisecond)
}

// Stop stops the mock server
func (m *MockInferenceServer) Stop() {
	if m.server != nil {
		// GracefulStop ensures all pending RPCs are completed
		m.server.GracefulStop()
		// Give the server time to fully clean up
		time.Sleep(50 * time.Millisecond)
	}
	if m.listener != nil {
		m.listener.Close()
	}
}

// Reset clears all requests and responses
func (m *MockInferenceServer) Reset() {
	m.requests = make([]*pb.ModelInferRequest, 0)
	m.responses = make(map[string]*pb.ModelInferResponse)
	m.metadata = make(map[string]*pb.ModelMetadataResponse)
	m.errors = make(map[string]error)
	m.serverLiveCalls = 0
}

// ServerLive implements the health check
func (m *MockInferenceServer) ServerLive(ctx context.Context, req *pb.ServerLiveRequest) (*pb.ServerLiveResponse, error) {
	m.serverLiveCalls++
	return &pb.ServerLiveResponse{Live: true}, nil
}

// ServerReady implements the readiness check
func (m *MockInferenceServer) ServerReady(ctx context.Context, req *pb.ServerReadyRequest) (*pb.ServerReadyResponse, error) {
	return &pb.ServerReadyResponse{Ready: true}, nil
}

// ModelReady implements the model readiness check
func (m *MockInferenceServer) ModelReady(ctx context.Context, req *pb.ModelReadyRequest) (*pb.ModelReadyResponse, error) {
	// Check if we have a response configured for this model
	if _, exists := m.responses[req.Name]; exists {
		return &pb.ModelReadyResponse{Ready: true}, nil
	}

	// Check if we have an error configured for this model
	if _, exists := m.errors[req.Name]; exists {
		return &pb.ModelReadyResponse{Ready: false}, nil
	}

	// Default to ready for unknown models
	return &pb.ModelReadyResponse{Ready: true}, nil
}

// ServerMetadata implements the server metadata retrieval
func (m *MockInferenceServer) ServerMetadata(ctx context.Context, req *pb.ServerMetadataRequest) (*pb.ServerMetadataResponse, error) {
	return &pb.ServerMetadataResponse{
		Name:       "mock-inference-server",
		Version:    "1.0.0",
		Extensions: []string{"health_check"},
	}, nil
}

// ModelMetadata implements the model metadata retrieval
func (m *MockInferenceServer) ModelMetadata(ctx context.Context, req *pb.ModelMetadataRequest) (*pb.ModelMetadataResponse, error) {
	// Check if we have custom metadata for this model
	if metadata, exists := m.metadata[req.Name]; exists {
		return metadata, nil
	}

	// Check if we have an error configured for this model
	if err, exists := m.errors[req.Name]; exists {
		return nil, err
	}

	// For testing purposes, return an error if no metadata is configured
	// This simulates a model that doesn't support metadata queries
	return nil, status.Error(codes.NotFound, fmt.Sprintf("model metadata not found for model: %s", req.Name))
}

// ModelInfer implements the main inference endpoint
func (m *MockInferenceServer) ModelInfer(ctx context.Context, req *pb.ModelInferRequest) (*pb.ModelInferResponse, error) {
	// Store the request for verification
	m.requests = append(m.requests, req)

	// Check if we have an error configured for this model
	if err, exists := m.errors[req.ModelName]; exists {
		return nil, err
	}

	// Check if we have a response configured for this model
	if response, exists := m.responses[req.ModelName]; exists {
		return response, nil
	}

	// Generate a default response based on the model name
	return m.generateDefaultResponse(req), nil
}

// generateDefaultResponse creates a default response based on the request
func (m *MockInferenceServer) generateDefaultResponse(req *pb.ModelInferRequest) *pb.ModelInferResponse {
	response := &pb.ModelInferResponse{
		ModelName:    req.ModelName,
		ModelVersion: req.ModelVersion,
		Id:           req.Id,
		Outputs:      make([]*pb.ModelInferResponse_InferOutputTensor, 0),
	}

	// Generate outputs based on model name patterns
	switch {
	case containsString(req.ModelName, "scale"):
		// For scaling models, return scaled values
		scaleFactor := extractScaleFactor(req.ModelName)
		response.Outputs = append(response.Outputs, &pb.ModelInferResponse_InferOutputTensor{
			Name:     "scaled_output",
			Datatype: "FP64",
			Shape:    []int64{1},
			Contents: &pb.InferTensorContents{
				Fp64Contents: []float64{100.0 * scaleFactor}, // Default scaled value
			},
		})

	case containsString(req.ModelName, "calculate"):
		// For calculation models, return calculated values
		response.Outputs = append(response.Outputs, &pb.ModelInferResponse_InferOutputTensor{
			Name:     "calculated_output",
			Datatype: "FP64",
			Shape:    []int64{1},
			Contents: &pb.InferTensorContents{
				Fp64Contents: []float64{25.0}, // Default calculated value
			},
		})

	default:
		// Default response
		response.Outputs = append(response.Outputs, &pb.ModelInferResponse_InferOutputTensor{
			Name:     "output",
			Datatype: "FP64",
			Shape:    []int64{1},
			Contents: &pb.InferTensorContents{
				Fp64Contents: []float64{1.0},
			},
		})
	}

	return response
}

// Helper functions
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}

func extractScaleFactor(modelName string) float64 {
	// Extract scale factor from model names like "scale_5"
	if len(modelName) > 6 && modelName[:6] == "scale_" {
		switch modelName[6:] {
		case "2":
			return 2.0
		case "5":
			return 5.0
		case "10":
			return 10.0
		default:
			return 1.0
		}
	}
	return 1.0
}

// CreateMockResponseForScaling creates a mock response for scaling operations
func CreateMockResponseForScaling(modelName string, scaleFactor float64, inputValue float64) *pb.ModelInferResponse {
	return &pb.ModelInferResponse{
		ModelName:    modelName,
		ModelVersion: "1",
		Id:           "test-request",
		Outputs: []*pb.ModelInferResponse_InferOutputTensor{
			{
				Name:     "scaled_output",
				Datatype: "FP64",
				Shape:    []int64{1},
				Contents: &pb.InferTensorContents{
					Fp64Contents: []float64{inputValue * scaleFactor},
				},
			},
		},
	}
}

// CreateMockResponseForCalculation creates a mock response for calculation operations
func CreateMockResponseForCalculation(modelName string, result float64) *pb.ModelInferResponse {
	return &pb.ModelInferResponse{
		ModelName:    modelName,
		ModelVersion: "1",
		Id:           "test-request",
		Outputs: []*pb.ModelInferResponse_InferOutputTensor{
			{
				Name:     "calculated_output",
				Datatype: "FP64",
				Shape:    []int64{1},
				Contents: &pb.InferTensorContents{
					Fp64Contents: []float64{result},
				},
			},
		},
	}
}

// CreateMockResponseForMultipleOutputs creates a mock response for models with multiple outputs
func CreateMockResponseForMultipleOutputs(modelName string, outputValues []float64) *pb.ModelInferResponse {
	outputs := make([]*pb.ModelInferResponse_InferOutputTensor, len(outputValues))
	
	for i, value := range outputValues {
		var datatype string
		var contents *pb.InferTensorContents
		
		// Determine data type based on value (simple heuristic)
		if value == float64(int64(value)) {
			datatype = "INT64"
			contents = &pb.InferTensorContents{
				Int64Contents: []int64{int64(value)},
			}
		} else {
			datatype = "FP64"
			contents = &pb.InferTensorContents{
				Fp64Contents: []float64{value},
			}
		}
		
		outputs[i] = &pb.ModelInferResponse_InferOutputTensor{
			Name:     fmt.Sprintf("output_%d", i),
			Datatype: datatype,
			Shape:    []int64{1},
			Contents: contents,
		}
	}
	
	return &pb.ModelInferResponse{
		ModelName:    modelName,
		ModelVersion: "1",
		Id:           "test-request",
		Outputs:      outputs,
	}
}

// CreateMockResponseForDataType creates a mock response with specific data type
func CreateMockResponseForDataType(modelName string, dataType string, value interface{}) *pb.ModelInferResponse {
	output := &pb.ModelInferResponse_InferOutputTensor{
		Name:     "output",
		Datatype: dataType,
		Shape:    []int64{1},
	}
	
	switch dataType {
	case "FP32":
		if v, ok := value.(float32); ok {
			output.Contents = &pb.InferTensorContents{
				Fp32Contents: []float32{v},
			}
		}
	case "FP64":
		if v, ok := value.(float64); ok {
			output.Contents = &pb.InferTensorContents{
				Fp64Contents: []float64{v},
			}
		}
	case "INT32":
		if v, ok := value.(int32); ok {
			output.Contents = &pb.InferTensorContents{
				IntContents: []int32{v},
			}
		}
	case "INT64":
		if v, ok := value.(int64); ok {
			output.Contents = &pb.InferTensorContents{
				Int64Contents: []int64{v},
			}
		}
	default:
		// Default to FP64
		if v, ok := value.(float64); ok {
			output.Datatype = "FP64"
			output.Contents = &pb.InferTensorContents{
				Fp64Contents: []float64{v},
			}
		}
	}
	
	return &pb.ModelInferResponse{
		ModelName:    modelName,
		ModelVersion: "1",
		Id:           "test-request",
		Outputs:      []*pb.ModelInferResponse_InferOutputTensor{output},
	}
}

// CreateMockResponseForMixedTypes creates a mock response with multiple outputs of different types
func CreateMockResponseForMixedTypes(modelName string, values map[string]interface{}) *pb.ModelInferResponse {
	outputs := make([]*pb.ModelInferResponse_InferOutputTensor, 0, len(values))
	
	i := 0
	for _, value := range values {
		output := &pb.ModelInferResponse_InferOutputTensor{
			Name:  fmt.Sprintf("output_%d", i),
			Shape: []int64{1},
		}
		
		switch v := value.(type) {
		case float32:
			output.Datatype = "FP32"
			output.Contents = &pb.InferTensorContents{
				Fp32Contents: []float32{v},
			}
		case float64:
			output.Datatype = "FP64"
			output.Contents = &pb.InferTensorContents{
				Fp64Contents: []float64{v},
			}
		case int32:
			output.Datatype = "INT32"
			output.Contents = &pb.InferTensorContents{
				IntContents: []int32{v},
			}
		case int64:
			output.Datatype = "INT64"
			output.Contents = &pb.InferTensorContents{
				Int64Contents: []int64{v},
			}
		default:
			// Default to FP64 with zero value
			output.Datatype = "FP64"
			output.Contents = &pb.InferTensorContents{
				Fp64Contents: []float64{0.0},
			}
		}
		
		outputs = append(outputs, output)
		i++
	}
	
	return &pb.ModelInferResponse{
		ModelName:    modelName,
		ModelVersion: "1",
		Id:           "test-request",
		Outputs:      outputs,
	}
}

// CreateMockResponseForFilesystem creates a mock response for filesystem prediction scenarios
func CreateMockResponseForFilesystem(modelName string, capacityPrediction float64) *pb.ModelInferResponse {
	return &pb.ModelInferResponse{
		ModelName:    modelName,
		ModelVersion: "1",
		Id:           "test-request",
		Outputs: []*pb.ModelInferResponse_InferOutputTensor{
			{
				Name:     "filesystem_output",
				Datatype: "FP64",
				Shape:    []int64{1},
				Contents: &pb.InferTensorContents{
					Fp64Contents: []float64{capacityPrediction},
				},
			},
		},
	}
}

// CreateMockErrorResponse creates an error for testing error scenarios
func CreateMockErrorResponse(code codes.Code, message string) error {
	return status.Error(code, message)
}
