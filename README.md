# OpenTelemetry Inference Collector

An OpenTelemetry Collector distribution that includes a custom Metrics Inference Processor. This processor enables real-time machine learning inference on telemetry metrics by integrating with external inference servers using the KServe v2 inference protocol.

## Features

- **Real-time ML Inference**: Process OpenTelemetry metrics through ML models in real-time
- **KServe v2 Protocol**: Compatible with any inference server that implements the KServe v2 protocol (MLServer, Triton, etc.)
- **Automatic Metadata Discovery**: Automatically discovers model output specifications from inference servers
- **Broadcast Semantics**: Intelligently handles multi-input scenarios where inputs have different attribute schemas
- **Attribute Preservation**: Maintains data lineage by preserving metric attributes through inference operations
- **Flexible Configuration**: Rule-based system for routing metrics to different models
- **Graceful Error Handling**: Input metrics always pass through unchanged, even on inference failures
- **Model Input Validation**: Validates rule inputs against model signatures for compatibility

## Quick Start

### Prerequisites

- Go 1.21+
- Podman or Docker
- Make

### Building

```bash
# Install OpenTelemetry Collector Builder
make install-ocb

# Build the collector
make build

# Run with sample configuration
make run
```

### Running the Demo

```bash
# Start the complete demo environment
make demo-start

# Check status
make demo-status

# View logs
make demo-logs

# Stop the demo
make demo-stop
```

Access the demo services:
- Grafana: http://localhost:3000 (admin/temp1234)
  - **Enhanced Kalman Filter Dashboard**: Real-time CPU prediction using host metrics
  - Shows actual vs predicted CPU utilization with intelligent metric naming
  - Displays prediction variance, innovation, trend, and model confidence
- VictoriaMetrics: http://localhost:8428
- MLServer: http://localhost:8082
- Collector Metrics: http://localhost:8888/metrics

## Metrics Inference Processor

The Metrics Inference Processor sends OpenTelemetry metrics to ML models for inference and creates new metrics from the results.

### Key Features

1. **Automatic Output Discovery**: The processor queries model metadata during startup to automatically discover output tensor specifications
2. **Intelligent Output Naming**: Automatically generates meaningful names for output metrics:
   - Detects and removes common prefixes to avoid redundancy
   - Preserves semantic meaning from input metric names
   - Groups metrics by category when dealing with many inputs
   - Supports custom pattern-based naming for specific requirements
3. **Inference Metadata Labels**: All output metrics include minimal labels for low cardinality:
   - `otel.inference.model.name`: The model name used for inference
   - `otel.inference.model.version`: The model version (if specified)
4. **Attribute Preservation**: Output metrics inherit attributes from their primary input metric
5. **Rule-Based Processing**: Configure multiple rules to route different metrics to different models

### Configuration Example

```yaml
processors:
  metricsinference:
    grpc:
      endpoint: "localhost:8081"
      use_ssl: false
    timeout: 30
    # Optional: Configure output metric naming
    naming:
      max_stem_parts: 2          # Maximum parts to keep from each input
      skip_common_domains: true  # Skip common prefixes like "system", "app"
      enable_category_grouping: true  # Group by categories when many inputs
      abbreviation_threshold: 4  # Number of inputs before abbreviation
    rules:
      - model_name: "simple-scaler"
        inputs: ["gen"]
        parameters:
          scale_factor: 2.0
        # Output: gen.scaled_output (intelligent naming preserves semantic meaning)
      
      - model_name: "simple-product"
        inputs: ["system.memory.utilization", "system.memory.limit"]
        # Output: memory_utilization_memory_limit.product_result (removes "system" prefix)
      
      - model_name: "anomaly-detector"
        inputs: ["system.cpu.utilization"]
        output_pattern: "ml.{model}.{output}"  # Custom pattern override
        # Output: ml.anomaly-detector.anomaly_score
```

### Broadcast Semantics

The processor intelligently handles multi-input scenarios where inputs have different attribute schemas through **broadcast semantics**:

#### Example: Memory Usage Calculation

```yaml
# Configuration
- model_name: "memory-calculator"
  inputs: ["system.memory.utilization", "system.memory.limit"]
```

**Input Metrics:**
- `system.memory.utilization` has multiple data points with `state` labels (used, free, cached, etc.)
- `system.memory.limit` has a single data point with no `state` label (total memory)

**Broadcast Behavior:**
The processor automatically broadcasts the single `system.memory.limit` value to all `state` combinations from `system.memory.utilization`, creating inference requests for:
- `state=used`: utilization=45%, limit=8GB → used_memory=3.6GB
- `state=free`: utilization=30%, limit=8GB → free_memory=2.4GB  
- `state=cached`: utilization=25%, limit=8GB → cached_memory=2.0GB

**Output Metrics:**
Each result preserves the original attributes:
- `memory.calculated{state="used"}`: 3.6GB
- `memory.calculated{state="free"}`: 2.4GB
- `memory.calculated{state="cached"}`: 2.0GB

This ensures proper data lineage and enables complex multi-dimensional inference while maintaining metric cardinality.

## Demo Models

### Enhanced Kalman Filter

The demo includes an advanced Kalman Filter implementation for CPU utilization prediction:

- **5D State Vector**: Tracks CPU usage, trend, memory utilization, load average, and context switches
- **Cross-Correlation Modeling**: Captures realistic interactions between system metrics
- **Adaptive Noise Estimation**: Dynamically adjusts to system behavior changes
- **Multi-Metric Inputs**: Uses memory utilization and load averages for improved accuracy
- **Rich Outputs**: Provides predictions, variance, innovation, trend, and confidence metrics

The model demonstrates how the Metrics Inference Processor can handle complex ML workloads with multiple inputs and outputs, automatic metadata discovery, and real-time processing.

## Intelligent Output Naming

The processor includes an advanced naming system that automatically generates meaningful output metric names:

### Key Benefits

- **Cleaner Metrics**: Removes redundant prefixes like "system", "app", "network"
- **Semantic Preservation**: Keeps the most meaningful parts of metric names
- **Conflict Prevention**: Ensures unique names when using the same model multiple times
- **Simple Configuration**: Easy-to-use options for customizing naming behavior

### Examples

```yaml
# Input: system.cpu.utilization
# Output: cpu_utilization.prediction (removes redundant "system")

# Input: ["system.cpu.utilization", "system.memory.usage"]  
# Output: cpu_utilization_memory_usage.anomaly_score (common prefix removed)

# Input: ["cpu.user", "cpu.system", "memory.used", "memory.free"]
# Output: cpu2_mem2.resource_score (category grouping for many inputs)
```

### Configuration

```yaml
processors:
  metricsinference:
    naming:                          # Optional - works great with defaults
      max_stem_parts: 2              # Parts to keep from each input
      skip_common_domains: true      # Skip common prefixes  
      enable_category_grouping: true # Group similar metrics
      abbreviation_threshold: 4      # When to abbreviate
```

For detailed configuration and examples, see the [processor documentation](processor/metricsinferenceprocessor/README_naming.md).

## Architecture

The project uses a simplified build system based on OpenTelemetry Collector Builder (OCB):

```
/
├── Makefile                     # Build system
├── builder-config.yaml          # OCB configuration
├── otelcol.yaml                # Sample configuration
├── processor/                   
│   └── metricsinferenceprocessor/  # Core processor
├── demo/                        # Demo pipeline
└── tasks/                       # Development tasks
```

## Development

### Initial Setup

Set up your development environment with formatting tools and git hooks:

```bash
make setup
```

This will:
- Install Go formatting tools (goimports, gofumpt)
- Configure git pre-commit hooks for code quality
- Set up the OpenTelemetry Collector Builder

### Code Quality

The project enforces code formatting standards through:

1. **Automatic formatting** with `make fmt`:
   - `go fmt` - Standard Go formatting
   - `goimports` - Organizes imports
   - `gofumpt` - Stricter formatting rules

2. **Pre-commit hooks** that check:
   - Go formatting before each commit
   - Import ordering
   - File size limits (5MB)

3. **CI/CD checks** that verify:
   - Code formatting (fail on unformatted code)
   - Import ordering
   - `go vet` static analysis
   - `go mod tidy` consistency

```bash
# Check formatting without changing files
make fmt-check

# Auto-fix formatting issues
make fmt

# Run all linting checks
make lint
```

### Running Tests

```bash
# Run processor tests
make test

# Run integration tests (requires podman)
make integration-test

# Run all tests
make test-all
```

### Common Tasks

See all available make targets:
```bash
make help
```

## CI/CD Pipeline

The project uses a unified 3-workflow CI/CD strategy that provides clear separation of concerns:

### 1. Continuous Integration (CI)

The CI workflow validates code quality and functionality on every push and pull request:

- **Linting**: Go formatting (gofmt, goimports, gofumpt), static analysis, and conventional commit messages
- **Testing**: Unit tests with coverage reporting
- **Building**: Binary compilation with version injection
- **Integration Testing**: Service integration tests with MLServer
- **Docker Validation**: Container image build verification

Jobs run with intelligent dependencies: lint → test/build → integration-test/docker-build

### 2. Automated Releases

The release workflow runs after successful CI completion and handles:

- **Semantic Versioning**: Automated version determination based on conventional commits
  - `feat:` → minor version bump
  - `fix:`, `perf:`, `refactor:` → patch version bump
  - `BREAKING CHANGE:` → major version bump
- **Artifact Publishing**: Binary artifacts attached to GitHub releases
- **Container Registry**: Docker images published to GitHub Container Registry (GHCR)
- **Multi-Platform Support**: Currently linux/amd64, ready for multi-arch expansion

### 3. Documentation

The documentation workflow automatically:

- **Builds Go Documentation**: Generates API documentation from source
- **Deploys to GitHub Pages**: Publishes documentation on main branch and tag pushes
- **Maintains Version History**: Preserves documentation for each release

### GitHub Container Registry

Docker images are available at:
```
ghcr.io/pteradigm/opentelemetry-inference:latest
ghcr.io/pteradigm/opentelemetry-inference:<version>
```

## License

See [LICENSE](LICENSE) file.
