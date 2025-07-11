# Enhanced Kalman Filter Model

An advanced Kalman Filter implementation for multi-feature CPU utilization prediction in OpenTelemetry environments.

## Overview

This model implements a 5-dimensional Kalman Filter that predicts CPU utilization by analyzing multiple system metrics and their interactions. It goes beyond simple time-series prediction by incorporating cross-correlations between CPU, memory, and system load.

## Features

- **5D State Vector**: Tracks CPU usage, trend, memory, load average, and context switches
- **Cross-Correlation Modeling**: Captures realistic interactions between system metrics
- **Adaptive Noise Estimation**: Dynamically adjusts to changing system behavior
- **Preprocessing Pipeline**: Handles missing data and outliers
- **Multi-Metric Fusion**: Combines memory and load average data for improved predictions

## Technical Details

### State Vector Components

1. **CPU Usage** (0-1): Current CPU utilization estimate
2. **CPU Trend**: Rate of change in CPU utilization
3. **Memory Usage** (0-1): Current memory utilization
4. **Load Average**: System load average (normalized)
5. **Context Switches**: Estimated context switch rate

### State Transition Matrix

The model uses a sophisticated transition matrix that models:

- CPU evolution with trend and cross-effects
- Memory impact on CPU (garbage collection, swapping)
- Load average persistence and influence
- Context switch indicators of system contention

### Inputs

- `cpu_utilization`: Direct CPU usage percentage (primary input)
- `memory_utilization`: System memory usage percentage
- `load_average_1m`: 1-minute load average

### Outputs

- `cpu_prediction`: Predicted CPU utilization (0-1)
- `prediction_variance`: Uncertainty in the prediction
- `innovation`: Prediction residuals
- `cpu_trend`: CPU utilization velocity
- `model_confidence`: Overall confidence (0-1)

## Implementation

The model is implemented using:

- **FilterPy**: For Kalman Filter mathematics
- **MLServer**: For KServe v2 protocol compatibility
- **NumPy**: For numerical computations

## Performance

- **Target Accuracy**: 75-90% for 5-minute predictions
- **Current Status**: Model is tuned and performing well
- **Confidence Levels**: Typically 80-90% after convergence
- **Variance Bounds**: Maximum variance < 0.03 (well below 10.0 limit)
- **Numerical Stability**: Excellent - all scenarios pass variance tests

## Usage

### Running Tests

```bash
# Use the project's Python 3.12 virtual environment
/path/to/project/.venv/bin/python test_model.py -v
```

### Variance Tuning Validation

```bash
# Test variance bounds and adaptive behavior
/path/to/project/.venv/bin/python test_variance_tuning.py
```

### Quick Validation

```bash
/path/to/project/.venv/bin/python simple_check.py
```

## Configuration

The model has been optimized with the following parameters:

- **Process noise covariance (Q)**: Diagonal [0.001, 0.0001, 0.001, 0.001, 0.01]
- **Measurement noise covariance (R)**: Diagonal [0.01, 0.01, 0.05] (optimized from [0.02, 0.03, 0.1])
- **State transition matrix (F)**: Cross-correlations tuned for realistic system behavior
- **Innovation window**: 50 samples for adaptive estimation
- **Variance limits**: Max 10.0, Min 1e-6 with automatic reset threshold at 100.0

See `TUNING_GUIDE.md` for detailed tuning information.

## Recent Improvements (January 2025)

1. **Fixed Input Metrics**: Now uses direct CPU utilization instead of estimation from load average
2. **Optimized Measurement Noise**: Reduced R matrix values based on tuning experiments (29% variance reduction)
3. **Python 3.12 Compatibility**: Updated to work with MLServer 1.7.1 and Python 3.12
4. **Comprehensive Testing**: Added variance tuning tests to ensure numerical stability

## Future Improvements

1. **Online Learning**: Implement online parameter estimation
2. **Multi-CPU Support**: Better handling of multi-core systems
3. **Long-term Predictions**: Extend prediction horizon beyond 5 minutes
4. **Anomaly Detection**: Add outlier detection capabilities

## Files

- `model.py`: Main model implementation with optimized parameters
- `test_model.py`: Comprehensive unit tests
- `test_variance_tuning.py`: Variance bounds and stability tests
- `simple_check.py`: Quick accuracy validation
- `requirements.txt`: Python dependencies (MLServer 1.7.1+)
- `TUNING_GUIDE.md`: Detailed tuning documentation and results
- `model-settings.json`: MLServer configuration
