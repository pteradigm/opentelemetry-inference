# Testing Strategy for Metrics Inference Processor

This document outlines the three-tier testing strategy for the metrics inference processor, covering unit tests, integration tests, and end-to-end (e2e) tests.

## Test Categories

### 1. Unit Tests (with Mock Server)

**Purpose**: Test processor logic, naming strategies, and error handling in isolation

**Test Files**:
- `processor_test.go` - Core processor functionality
- `processor_naming_test.go` - Intelligent naming vs output patterns
- `processor_metadata_test.go` - Metadata discovery and handling
- `broadcast_test.go` - Broadcasting semantics
- `multi_datapoint_test.go` - Multi-datapoint handling
- `inference_labels_test.go` - Label/attribute preservation with namespacing

**Naming Strategy Coverage**:
- **Intelligent Naming Tests**:
  - Single input metric → `{input}.{output}`
  - Multiple input metrics → `{input1}_{input2}.{output}`
  - Complex input names with dots/underscores
  - Category-based abbreviations
  
- **Output Pattern Tests**:
  - `{output}` - Exact output name
  - `{model}.{output}` - Model-prefixed output
  - `{input}.{output}` - Input-prefixed output
  - Custom patterns with literals

**Key Test Scenarios**:
```go
// Example: Test with intelligent naming (no output pattern)
{
    ModelName: "anomaly_detector",
    Inputs:    []string{"cpu.usage", "memory.usage"},
    Outputs: []OutputSpec{
        {Name: "anomaly_score"},
    },
}
// Expected output: "cpu_usage_memory_usage.anomaly_score"

// Example: Test with output pattern
{
    ModelName:     "anomaly_detector",
    Inputs:        []string{"cpu.usage", "memory.usage"},
    OutputPattern: "{model}.{output}",
    Outputs: []OutputSpec{
        {Name: "score"},
    },
}
// Expected output: "anomaly_detector.score"
```

**Attribute Namespacing Tests**:
- Input attributes are namespaced to prevent conflicts
- Example: `state` from `system.cpu.utilization` → `system.cpu.utilization.state`
- Example: `state` from `system.memory.utilization` → `system.memory.utilization.state`
- Ensures attributes from different inputs remain distinct
- Preserves semantic meaning of attributes in multi-input scenarios

### 2. Integration Tests (with KServe/MLServer)

**Purpose**: Test real gRPC communication with actual inference servers

**Test File**: `integration_test.go`

**Requirements**:
- MLServer running with test models
- Tagged with `//go:build integration`
- Run with: `INTEGRATION_TEST=1 go test -tags=integration`

**Test Coverage**:
- Model metadata discovery
- Actual inference calls
- Error handling with real server
- Multiple model scenarios
- Both naming strategies with real models

### 3. End-to-End Tests (testenv)

**Purpose**: Validate complete pipeline with OpenTelemetry Collector

**Location**: `testenv/`

**Components**:
- MLServer with custom Python models
- Docker Compose setup
- Test validation scripts

**Models Available**:
- `simple-scaler` - Scales input by 2.0
- `simple-sum` - Sums multiple inputs
- `simple-product` - Multiplies inputs

**Test Scenarios**:
1. Full collector pipeline with inference
2. Multiple rules with different naming strategies
3. Performance and latency validation
4. Resource usage monitoring

## Test Execution Guide

### Running Unit Tests
```bash
# Run all unit tests
go test -v ./...

# Run specific naming tests
go test -v -run TestProcessorNamingStrategies .

# Run golden file tests
go test -v -run TestGoldenFileMetrics .
```

### Running Integration Tests
```bash
# Start MLServer first (if not using testenv)
cd testenv && podman-compose up -d

# Run integration tests
INTEGRATION_TEST=1 go test -tags=integration -v -run TestMLServerIntegration .
```

### Running E2E Tests
```bash
# Start test environment
cd testenv
./test-setup.sh

# Run e2e validation
# (Additional e2e test scripts to be added)
```

## Test Data Organization

### Golden Files (`testdata/`)
- Organized by test category
- Each test has:
  - `config.yaml` - Processor configuration
  - `*_input.yaml` - Input metrics
  - `*_expected.yaml` - Expected output

### Naming Strategy Examples in Tests

#### Intelligent Naming (Default)
```yaml
rules:
  - model_name: "cpu_predictor"
    inputs: ["system.cpu.utilization"]
    outputs:
      - name: "prediction"
# Result: "system_cpu_utilization.prediction"
```

#### Pattern-Based Naming
```yaml
rules:
  - model_name: "cpu_predictor"
    inputs: ["system.cpu.utilization"]
    output_pattern: "predicted.{input}"
    outputs:
      - name: "value"
# Result: "predicted.system.cpu.utilization"
```

## Continuous Integration

All three test tiers should be run in CI:
1. Unit tests on every commit
2. Integration tests on PR merges
3. E2E tests nightly or on release candidates

## Adding New Tests

When adding new functionality:
1. Add unit tests with mock server
2. Add integration test cases if gRPC behavior changes
3. Update e2e tests if user-visible behavior changes
4. Ensure both naming strategies are tested where applicable