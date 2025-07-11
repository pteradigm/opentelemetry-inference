# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is an OpenTelemetry Collector distribution that includes a custom Metrics Inference Processor. The processor enables real-time machine learning inference on telemetry metrics by integrating with external inference servers using the KServe v2 inference protocol.

The project has been recently refactored (June 2025) to use a simplified build system based on OpenTelemetry Collector Builder (OCB), mirroring the metricsgenreceiver pattern while maintaining multi-module support for future trace and logs processors.

## Task Management

The project uses a structured task system in `./tasks/`:
- Task format: `TASK-XXXX-YY-ZZ` (ID-SubTask-Version)
- Directories: `active/`, `backlog/`, `completed/`
- Each task includes user stories, Gherkin specs, and implementation plans
- See `tasks/tasks-directive.md` for task creation guidelines
- Always use TodoWrite/TodoRead tools to track progress on active tasks

## Common Development Commands

### Building
```bash
# Install OpenTelemetry Collector Builder
make install-ocb

# Build the collector binary
make build

# Build and run with sample configuration
make run

# Install to GOPATH/bin
make install

# Clean build artifacts
make clean
```

### Testing
```bash
# Run all processor tests
make test

# Run KServe integration tests (requires podman-compose)
make integration-test

# Run specific test in processor directory
cd processor/metricsinferenceprocessor
go test -v -run TestSpecificFunction

# Run mock server tests (preferred approach)
go test -v -run TestMetricsInferenceProcessorWithMockServer
```

### Protocol Buffer Generation
```bash
# Generate protobuf code
cd processor/metricsinferenceprocessor
buf generate
```

## Architecture

### Simplified Project Structure
```
/
├── Makefile                     # Simple 9-target build system
├── builder-config.yaml          # OCB configuration
├── otelcol.yaml                 # Sample collector configuration
├── go.mod                       # Local module replacements
├── processor/                   # Multi-module directory
│   └── metricsinferenceprocessor/  # Core processor (unchanged)
│       ├── go.mod               # Independent module
│       ├── proto/v2/            # KServe v2 protocol definitions
│       ├── internal/testutil/   # Mock servers and test utilities
│       └── testenv/             # MLServer integration environment
├── tasks/                       # Structured task management
├── examples/                    # Configuration examples
└── otelcol-dev/                 # Build output (gitignored)
```

### Core Components

1. **Metrics Inference Processor** (`processor/metricsinferenceprocessor/`)
   - Implements gRPC client for KServe v2 inference protocol
   - Converts OpenTelemetry metrics to inference tensors
   - Creates new metrics from ML model inference results
   - Supports scaling and sum operations (with MLServer models)
   - **NEW**: Automatic metadata discovery using KServe v2 ModelMetadata RPC

2. **Protocol Definition** (`processor/metricsinferenceprocessor/proto/v2/`)
   - KServe v2 inference protocol implementation
   - GRPCInferenceService for model inference
   - Tensor-based data exchange

3. **Collector Distribution**
   - Built using OpenTelemetry Collector Builder (OCB)
   - Configuration in root `builder-config.yaml`
   - Output in `otelcol-dev/` directory

### Data Flow

1. Metrics received via OTLP receiver
2. Metrics Inference Processor groups metrics by configured rules
3. Processor converts metrics to tensors and sends to inference server
4. Inference results converted back to OpenTelemetry metrics
5. Enhanced metrics exported via configured exporters

### Configuration Structure

The processor uses rule-based configuration with automatic metadata discovery:
```yaml
processors:
  metricsinference:
    grpc:
      endpoint: "localhost:8081"  # MLServer gRPC endpoint
      use_ssl: false
      compression: true
    timeout: 30  # Inference timeout in seconds
    rules:
      - model_name: "simple-scale"     # Required
        model_version: "v1"            # Optional
        inputs: ["system.cpu.utilization"]  # Required
        # outputs: []                  # Optional - can be discovered from model metadata
        parameters:                    # Optional
          scale_factor: 2.0
```

**Metadata Discovery Feature (June 2025)**: The processor automatically discovers output specifications from model metadata using the KServe v2 ModelMetadata RPC. This eliminates the need to manually configure output names and types in most cases. The processor queries model metadata during startup and uses discovered tensor names and data types to create appropriate OpenTelemetry metrics.

**Output Name Decoration**: To prevent conflicts when multiple instances of the same model are used, discovered output names are automatically decorated with contextual information:
- Single input: `{input_name}.{output_name}` (e.g., `system.cpu.utilization.scaled_result`)
- Multiple inputs: `{first_input}_multi.{output_name}` (e.g., `cpu_usage_multi.anomaly_score`)
- This ensures each rule produces uniquely named metrics even when using the same model multiple times.

### Testing Infrastructure

The project uses two testing approaches:

1. **Mock Server Testing** (preferred for unit tests):
```go
mockServer := testutil.NewMockInferenceServer()
mockServer.Start(t)
defer mockServer.Stop()

// Configure mock responses
mockServer.SetModelResponse("model_name", testutil.CreateMockResponseForScaling("model_name", 2.0, 100.0))
```

2. **MLServer Integration Testing** (for end-to-end validation):
```bash
# MLServer with scaling and sum models runs in podman containers
make integration-test
```

Test utilities in `processor/metricsinferenceprocessor/internal/testutil/`:
- `mock_server.go`: Full KServe v2 protocol mock implementation
- `test_data.go`: Test metric generation utilities

### Key Design Patterns

- **Factory Pattern**: Component creation via `factory.go`
- **Pipeline Pattern**: Standard OpenTelemetry collector pipeline
- **Mock Server Testing**: No test mode in processor, use mock servers instead
- **Rule-Based Processing**: Flexible metric-to-model mapping
- **Multi-Module Support**: Ready for future traceinferenceprocessor/, loginferenceprocessor/

## Development Guidelines

### Multi-Module Expansion
When adding future processors:
1. Create `processor/[name]inferenceprocessor/` with own go.mod
2. Add entry to root `builder-config.yaml` processors section
3. No changes needed to root Makefile
4. Each processor remains independently testable and versioned

### Protocol Buffers
When modifying the inference protocol:
1. Update `processor/metricsinferenceprocessor/proto/v2/inference.proto`
2. Run `buf generate` from the processor directory
3. Regenerated Go code will be in the same directory

### Testing Best Practices
- Use mock servers for unit tests, never add test mode to production code
- Test error scenarios explicitly
- Verify gRPC communication patterns
- Component lifecycle tests use special "localhost:12345" endpoint handling

### Error Handling
- Preserve gRPC status codes in error messages
- Continue processing other metrics on individual inference failures
- Log errors with appropriate context including model names and endpoints

### Code Quality
- Comments should describe the "why", not the "what"
- Avoid frivolous comments - delete them if found
- Follow existing patterns for gRPC client management
- Use connection pooling and graceful shutdown patterns

## Current Status

- **Main branch**: `main`
- **Version**: 0.0.1 (early development)
- **Build System**: Simplified OCB-based system (June 2025 refactor)
- **Stability**: Development phase, not recommended for production
- **Testing**: Comprehensive mock server and MLServer integration tests

## Development Tools

- **OCB**: OpenTelemetry Collector Builder for custom collector creation
- **buf**: Protocol buffer generation (configured in `buf.gen.yaml`)
- **podman-compose**: Container orchestration for MLServer integration tests
- **MLServer**: KServe v2 compatible model serving for integration testing

The project structure is designed for simplicity and future expansion, with clear separation between the collector build system and individual processor development.

## Configuration and Tooling Recommendations

- Use `podman` rather than `docker`

### Python Development Setup

- For simplicity use the Python virtual environment found in the `./.venv` directory at the root of the project. If it doesn't exist, create it with `python -m venv .venv` in the root of the project. Always prepend the path to python or pip e.g., `./.venv/bin/python` and `./.venv/bin/pip` etc.

- **Python Virtual Environment Guidelines**:
  - The Python virtual environment is in the root of the project and all calls to python and pip should reflect that when they are being run in different directories.