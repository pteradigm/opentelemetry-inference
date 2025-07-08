#!/bin/bash

# Test script for the simple KServe test environment
# This script validates that the MLServer setup is working correctly

set -e

echo "🔧 Testing Simple KServe Test Environment..."

# Function to wait for service to be ready
wait_for_service() {
    local url=$1
    local max_attempts=30
    local attempt=1
    
    echo "⏳ Waiting for service at $url to be ready..."
    
    while [ $attempt -le $max_attempts ]; do
        if curl -s -f "$url" > /dev/null 2>&1; then
            echo "✅ Service is ready!"
            return 0
        fi
        
        echo "   Attempt $attempt/$max_attempts - service not ready yet..."
        sleep 2
        attempt=$((attempt + 1))
    done
    
    echo "❌ Service failed to become ready after $max_attempts attempts"
    return 1
}

# Check if container is already running
if ! podman-compose ps | grep -q "testenv_mlserver_1.*Up"; then
  echo "🚀 Starting MLServer test environment..."
  podman-compose up --build -d
else
  echo "✅ MLServer test environment is already running"
fi

# Wait for MLServer to be ready
wait_for_service "http://localhost:9080/v2/health/live"
wait_for_service "http://localhost:9080/v2/health/ready"

echo "🔍 Testing MLServer endpoints..."

# Test server health
echo "Testing server liveness..."
curl -s -f http://localhost:9080/v2/health/live
echo "✅ Server is alive"

echo "Testing server readiness..."
curl -s -f http://localhost:9080/v2/health/ready
echo "✅ Server is ready"

# Test model metadata
echo "Testing model metadata..."
response=$(curl -s http://localhost:9080/v2/models/simple-scaler)
echo "Model metadata response: $response"

if echo "$response" | grep -q "simple-scaler"; then
    echo "✅ Model metadata retrieved successfully"
else
    echo "❌ Model metadata test failed"
    exit 1
fi

# Test inference
echo "🧪 Testing model inference..."
inference_response=$(curl -s -X POST http://localhost:9080/v2/models/simple-scaler/infer \
  -H "Content-Type: application/json" \
  -d '{
    "inputs": [
      {
        "name": "test_input",
        "shape": [1],
        "datatype": "FP64",
        "data": [5.0]
      }
    ]
  }')

echo "Inference response: $inference_response"

# Check if the response contains expected scaled value (5.0 * 2.0 = 10.0)
if echo "$inference_response" | grep -q "10"; then
    echo "✅ Inference test passed - input 5.0 was scaled to 10.0"
else
    echo "❌ Inference test failed - unexpected response"
    echo "Expected scaled value (10.0) not found in response"
    exit 1
fi

# Test gRPC endpoint (basic connectivity)
echo "🔌 Testing gRPC endpoint connectivity..."
if nc -z localhost 9081; then
    echo "✅ gRPC port (9081) is accessible"
else
    echo "❌ gRPC port (9081) is not accessible"
    exit 1
fi

echo ""
echo "🎉 All tests passed! The KServe test environment is working correctly."
echo ""
echo "📋 Summary:"
echo "   - MLServer is running on http://localhost:9080 (HTTP)"
echo "   - gRPC endpoint available on localhost:9081"
echo "   - Model 'simple-scaler' is ready and functional"
echo "   - Scaling factor: 2.0 (input 5.0 → output 10.0)"
echo ""
echo "To run integration tests:"
echo "   cd ../processor/metricsinferenceprocessor"
echo "   INTEGRATION_TEST=1 go test -tags=integration -v -run TestMLServerIntegration"
echo ""
echo "To stop the environment:"
echo "   podman-compose down"