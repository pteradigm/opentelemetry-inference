# Simple KServe Test Environment

This directory contains a minimal KServe v2 inference server setup using MLServer for testing the metrics inference processor with a real inference server.

## Overview

- **Model**: Simple scaling model that multiplies input values by 2.0
- **Server**: MLServer with KServe v2 protocol support
- **Deployment**: Docker Compose for easy setup/teardown

## Quick Start

### Prerequisites

- Podman and podman-compose installed
- curl (for health checks)

### Start the Test Environment

```bash
# From the testenv directory
podman-compose up --build -d

# Check if the server is ready
curl http://localhost:8080/v2/health/live
curl http://localhost:8080/v2/health/ready

# Check model metadata
curl http://localhost:8080/v2/models/simple-scaler
```

### Test the Model

```bash
# Test inference with a simple HTTP request
curl -X POST http://localhost:8080/v2/models/simple-scaler/infer \
  -H "Content-Type: application/json" \
  -d '{
    "inputs": [
      {
        "name": "input_data",
        "shape": [1],
        "datatype": "FP64",
        "data": [5.0]
      }
    ]
  }'

# Expected response: {"outputs": [{"name": "input_data_scaled", "shape": [1], "datatype": "FP64", "data": [10.0]}]}
```

### Stop the Test Environment

```bash
podman-compose down
```

## Integration with OpenTelemetry Processor

To use this test environment with the metrics inference processor:

1. Start the test environment: `podman-compose up -d`
2. Configure the processor with:
   ```yaml
   processors:
     metricsinference:
       grpc:
         endpoint: "localhost:8081"  # gRPC endpoint
       rules:
         - model_name: "simple-scaler"
           inputs: ["your.metric.name"]
           outputs:
             - name: "your.metric.name.scaled"
   ```
3. Run your collector or tests

## Model Details

### Input Format
- Accepts any numeric tensor data
- Supports FP32, FP64, INT32, INT64 data types
- Variable tensor shapes supported

### Output Format
- Returns scaled input values (multiplied by 2.0)
- Output tensor name: `{input_name}_scaled`
- Same data type and shape as input

### Customization

To change the scaling factor, modify `models/simple-scaler/model-settings.json`:

```json
{
  "parameters": {
    "scale_factor": 3.0
  }
}
```

## Troubleshooting

### Common Issues

1. **Port conflicts**: Ensure ports 8080 and 8081 are available
2. **Container build fails**: Check Podman service is running
3. **Health checks fail**: Wait 30 seconds for MLServer to fully start

### Logs

View MLServer logs:
```bash
podman-compose logs mlserver
```

### Debugging

Connect to the running container:
```bash
podman-compose exec mlserver bash
```

## Architecture

```
┌─────────────────┐    gRPC     ┌─────────────────┐
│ OTEL Processor  │────────────▶│ MLServer        │
│                 │   :8081     │ (KServe v2)     │
└─────────────────┘             └─────────────────┘
                                         │
                                         ▼
                                ┌─────────────────┐
                                │ Simple Scaler   │
                                │ Model           │
                                │ (scale by 2.0)  │
                                └─────────────────┘
```

## Next Steps

This simple test environment can be extended with:
- More complex models (multiple inputs/outputs)
- Different model runtimes (TensorFlow, PyTorch, etc.)
- Authentication and security
- Model versioning and A/B testing
- Performance monitoring