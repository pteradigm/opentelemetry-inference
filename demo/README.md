# OpenTelemetry Metrics Inference Demo Pipeline

This demo showcases a complete OpenTelemetry pipeline that demonstrates real-time metrics inference using machine learning models.

## Architecture

The pipeline consists of:

- **Host Metrics**: OpenTelemetry Collector's hostmetrics receiver for real system metrics
- **MLServer**: KServe v2 compatible inference server with enhanced Kalman Filter model
- **Custom OTel Collector**: Our collector with the metrics inference processor
- **VictoriaMetrics**: High-performance time-series database with Prometheus compatibility
- **Grafana**: Real-time visualization dashboard with intelligent metric naming

## Quick Start

```bash
# From project root
make demo-start
```

This will:
1. Build our custom OpenTelemetry Collector
2. Start all services with Docker Compose
3. Display access URLs for all components

## Access URLs

- **Grafana Dashboard**: http://localhost:3000 (admin/temp1234)
- **VictoriaMetrics**: http://localhost:8428
- **MLServer Inference**: http://localhost:8082 (HTTP) / http://localhost:8081 (gRPC)
- **OTel Collector Metrics**: http://localhost:8888/metrics

## What You'll See

### 1. Enhanced Kalman Filter Dashboard

The main Grafana dashboard demonstrates advanced multi-feature CPU prediction:

- **Multi-Feature Prediction**: Real-time CPU forecasting using 5D state vector
- **Prediction Variance**: Uncertainty bounds showing model confidence
- **Innovation Tracking**: Prediction residuals for accuracy assessment
- **CPU Trend Analysis**: Velocity/trend of CPU utilization changes
- **Model Confidence**: Overall confidence metric (0-100%)
- **System Health**: Real-time monitoring of all pipeline components

### 2. Real-Time Inference

- Host metrics receiver collects CPU, memory, and load average data
- Inference processor sends multiple metrics to the enhanced Kalman Filter
- Model tracks CPU usage, trend, memory, load average, and context switches
- Output includes 5 distinct metrics for comprehensive analysis
- Dashboard visualizes all aspects of the prediction system

### 3. Pipeline Demonstration

- **End-to-End Flow**: Demonstrates complete metrics pipeline from collection to storage
- **Inference Integration**: Shows MLServer integration with OpenTelemetry Collector
- **Real-Time Processing**: Displays live metrics processing and system monitoring

## Models Used

### Enhanced Kalman Filter Model (`kalman-filter`)

- **Purpose**: Advanced CPU utilization prediction using multi-feature state-space modeling
- **State Vector**: 5D - [cpu_usage, cpu_trend, memory_usage, load_average, context_switches]
- **Inputs**:
  - `system.memory.utilization` (memory usage percentage)
  - `system.cpu.load_average.15m` (15-minute load average)
  - `system.cpu.load_average.1m` (1-minute load average)
- **Outputs** (auto-discovered via metadata):
  - `cpu_prediction` - Predicted CPU utilization (0-1)
  - `prediction_variance` - Uncertainty in prediction
  - `innovation` - Prediction residuals for accuracy tracking
  - `cpu_trend` - Rate of change in CPU utilization
  - `model_confidence` - Overall model confidence (0-1)
- **Features**:
  - Cross-correlation modeling between system metrics
  - Adaptive noise estimation for dynamic environments
  - Preprocessing pipeline for data quality
  - Target accuracy: 75-90% for 5-minute predictions

### Scaling Model (`simple-scaler`)

- **Purpose**: Demonstrates simple scaling inference on metrics
- **Available for**: Testing and development
- **Output**: Scaled metric values

### Sum Model (`simple-sum`)

- **Purpose**: Demonstrates aggregation of multiple metrics
- **Available for**: Custom metric combinations
- **Output**: Aggregated metric values

## Management Commands

```bash
# View pipeline status
make demo-status

# Watch logs in real-time
make demo-logs

# Stop the pipeline
make demo-stop
```

## Configuration

Key configuration files:

- `configs/otel-collector-config.yaml`: Collector configuration with inference rules
- `configs/grafana/`: Grafana datasources and dashboards
- `docker-compose.yml`: Complete service orchestration

## Troubleshooting

### Services Not Starting

```bash
# Check service health
make demo-status

# View specific service logs
cd demo/pipeline
podman-compose logs mlserver
podman-compose logs otel-collector
```

### No Metrics in Grafana

1. Verify collector is receiving metrics: <http://localhost:8888/metrics>
2. Check VictoriaMetrics has data: <http://localhost:8428/api/v1/query?query=up>
3. Ensure Grafana datasources are connected (should be automatic)

### Inference Not Working

1. Check MLServer health: <http://localhost:8082/v2/health/live>
2. View collector logs: `podman-compose logs otel-collector`
3. Verify models are loaded: `podman-compose logs mlserver`

### Generate Test Metrics

The sample application continuously generates metrics. For manual testing:

```bash
# Check if metrics are flowing
curl http://localhost:8428/api/v1/query?query=up
```

## Performance Targets

- **Startup Time**: < 2 minutes for complete stack
- **Visualization Latency**: < 5 seconds from generation to dashboard
- **Resource Usage**: < 4GB RAM total
- **Inference Latency**: < 100ms per metric enhancement

## Next Steps

This demo provides a foundation for:

- Adding more sophisticated ML models
- Implementing trace and log inference processors
- Scaling to production environments
- Integrating with existing observability infrastructure
