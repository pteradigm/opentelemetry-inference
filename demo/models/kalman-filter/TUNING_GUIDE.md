# Kalman Filter Tuning Guide

This guide explains the tuning improvements made to address variance explosion issues.

## Problem Statement

The original Kalman filter implementation suffered from:
- Prediction variance growing to very large values (>1000)
- Numerical instability in the covariance matrix
- Poor predictions when using indirect CPU estimation from load average

## Root Causes

1. **Wrong Input Metrics**: The model was trying to predict CPU utilization without observing it directly
2. **Initial Covariance Too High**: P = 100 caused immediate variance explosion
3. **Unbounded Adaptive Noise**: Q matrix could grow without limit
4. **No Variance Limiting**: Covariance matrix P could grow indefinitely
5. **Numerical Issues**: No conditioning of covariance matrices

## Solutions Implemented

### 1. Correct Input Metrics
Changed from:
```yaml
inputs: ["system.memory.utilization", "system.cpu.load_average.15m", "system.cpu.load_average.1m"]
```

To:
```yaml
inputs: ["system.cpu.utilization", "system.memory.utilization", "system.cpu.load_average.1m"]
```

Now the model observes actual CPU utilization, making predictions much more accurate.

### 2. Conservative Initial Values
- Initial state covariance: P = 1.0 (was 100.0)
- Initial process noise: Q = 0.001 (was 0.01)
- More gradual adaptive adjustments

### 3. Variance Bounds
Added explicit limits:
- `max_variance = 10.0` - Maximum allowed variance
- `min_variance = 1e-6` - Minimum to prevent singularity
- `variance_reset_threshold = 100.0` - Reset if exceeded

### 4. Bounded Process Noise
- `max_process_noise = 0.1` - Upper bound for Q diagonal
- `min_process_noise = 1e-6` - Lower bound
- Smaller adaptation rates: 1.05/0.98 (was 1.1/0.95)

### 5. Numerical Conditioning
- Symmetrization: `P = 0.5 * (P + P.T)`
- Eigenvalue clipping to ensure positive definiteness
- Regular covariance matrix conditioning

## Tuning Parameters

### Process Noise (Q)
Controls how much the state can change between updates:
- CPU state: 0.001 (tracks actual measurements closely)
- CPU trend: 0.0001 (smooths velocity estimates)
- Context switches: 0.01 (allows more variation)

### Measurement Noise (R)
Represents sensor accuracy (optimized January 2025):
- CPU: 0.01 (1% measurement uncertainty) - reduced from 0.02
- Memory: 0.01 (1% measurement uncertainty) - reduced from 0.03
- Load average: 0.05 (5% measurement uncertainty) - reduced from 0.1

These optimized values resulted in:
- 29% reduction in maximum variance
- Improved numerical stability
- Better MSE across all test scenarios

### Adaptive Parameters
- Innovation window: 50 samples
- Adaptation interval: Every 10 observations
- Learning rates: 0.1 for measurement noise blending

## Validation

Run the test script to validate tuning:
```bash
cd demo/models/kalman-filter
python test_variance_tuning.py
```

Expected results (verified January 2025):
- Maximum variance < 0.03 in all scenarios (was < 10.0)
- Process noise remains bounded < 0.026 (was < 0.1)
- Model confidence typically 80-90% (improved from 70-85%)
- Smooth adaptation to changing noise levels

Latest test results:
- Normal operation: Max variance 0.020320
- High variance input: Max variance 0.025577
- Sudden jumps: Max variance 0.028451
- Oscillating pattern: Max variance 0.028963

## Further Tuning

If you need to adjust the filter further:

1. **For more responsive predictions**: Increase Q slightly (but keep < 0.01)
2. **For smoother predictions**: Decrease Q or increase R
3. **For faster adaptation**: Increase learning rate alpha (currently 0.1)
4. **For more stable long-term behavior**: Decrease max_variance

## Monitoring

Watch these metrics in Grafana:
- `prediction_variance`: Should stay below 10.0
- `innovation`: Should be white noise (no patterns)
- `model_confidence`: Should be > 0.5 most of the time

High variance indicates:
- Poor model fit (check state transition matrix F)
- Incorrect noise parameters
- Numerical issues (check matrix conditioning)