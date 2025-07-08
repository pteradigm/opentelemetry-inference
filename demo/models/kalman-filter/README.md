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

- `memory_utilization`: System memory usage percentage
- `load_average_15m`: 15-minute load average
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
- **Current Status**: Model is functional but requires calibration
- **Confidence Levels**: Typically 80-86% after convergence

## Usage

### Running Tests

```bash
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
pytest test_model.py -v
```

### Quick Validation

```bash
python simple_check.py
```

## Configuration

The model can be tuned by adjusting:

- Process noise covariance (Q matrix)
- Measurement noise covariance (R matrix)
- Cross-correlation coefficients in state transition matrix
- Innovation window size for adaptive estimation

## Future Improvements

1. **CPU Estimation**: Improve CPU estimation from load average
2. **Direct Metrics**: Use direct CPU metrics when available
3. **Online Learning**: Implement online parameter estimation
4. **Multi-CPU Support**: Better handling of multi-core systems

## Files

- `model.py`: Main model implementation
- `test_model.py`: Comprehensive unit tests
- `simple_check.py`: Quick accuracy validation
- `requirements.txt`: Python dependencies
