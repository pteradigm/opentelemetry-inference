#!/usr/bin/env python3
"""
Test script to validate Kalman filter variance tuning improvements.
Simulates various scenarios to ensure variance remains bounded.
"""

import numpy as np
import matplotlib.pyplot as plt
from model import KalmanFilterModel
from mlserver.types import InferenceRequest, RequestInput, Parameters
from mlserver.codecs import NumpyCodec
import asyncio


async def test_variance_bounds():
    """Test that variance remains bounded under various conditions."""
    print("Testing variance bounds...")
    
    # Initialize model with proper settings
    from mlserver.settings import ModelSettings
    settings = ModelSettings(
        name="kalman-filter",
        implementation="model.KalmanFilterModel"
    )
    model = KalmanFilterModel(settings)
    await model.load()
    
    # Test scenarios
    scenarios = [
        {
            "name": "Normal operation",
            "cpu": np.random.normal(0.5, 0.1, 100),
            "memory": np.random.normal(0.6, 0.05, 100),
            "load": np.random.normal(2.0, 0.5, 100)
        },
        {
            "name": "High variance input",
            "cpu": np.random.normal(0.5, 0.3, 100),
            "memory": np.random.normal(0.6, 0.2, 100),
            "load": np.random.normal(2.0, 2.0, 100)
        },
        {
            "name": "Sudden jumps",
            "cpu": np.concatenate([np.ones(50) * 0.2, np.ones(50) * 0.9]),
            "memory": np.concatenate([np.ones(50) * 0.3, np.ones(50) * 0.8]),
            "load": np.concatenate([np.ones(50) * 1.0, np.ones(50) * 8.0])
        },
        {
            "name": "Oscillating pattern",
            "cpu": 0.5 + 0.3 * np.sin(np.linspace(0, 10*np.pi, 100)),
            "memory": 0.6 + 0.2 * np.sin(np.linspace(0, 10*np.pi, 100) + np.pi/4),
            "load": 2.0 + 1.5 * np.sin(np.linspace(0, 10*np.pi, 100) + np.pi/2)
        }
    ]
    
    results = {}
    
    for scenario in scenarios:
        print(f"\nTesting scenario: {scenario['name']}")
        
        # Ensure values are in valid ranges
        cpu_data = np.clip(scenario['cpu'], 0.0, 1.0)
        mem_data = np.clip(scenario['memory'], 0.0, 1.0)
        load_data = np.clip(scenario['load'], 0.0, 100.0)
        
        # Create request
        inputs = [
            RequestInput(
                name="cpu_utilization",
                datatype="FP64",
                shape=[len(cpu_data)],
                data=cpu_data.tolist()
            ),
            RequestInput(
                name="memory_utilization",
                datatype="FP64",
                shape=[len(mem_data)],
                data=mem_data.tolist()
            ),
            RequestInput(
                name="load_average_1m",
                datatype="FP64",
                shape=[len(load_data)],
                data=load_data.tolist()
            )
        ]
        
        request = InferenceRequest(
            inputs=inputs,
            parameters=Parameters(content_type="np")
        )
        
        # Run inference
        response = await model.predict(request)
        
        # Extract results
        variances = NumpyCodec.decode_output(response.outputs[1])  # prediction_variance
        predictions = NumpyCodec.decode_output(response.outputs[0])  # cpu_prediction
        innovations = NumpyCodec.decode_output(response.outputs[2])  # innovation
        confidences = NumpyCodec.decode_output(response.outputs[4])  # model_confidence
        
        results[scenario['name']] = {
            'variances': variances,
            'predictions': predictions,
            'innovations': innovations,
            'confidences': confidences,
            'max_variance': np.max(variances),
            'mean_variance': np.mean(variances),
            'min_variance': np.min(variances)
        }
        
        print(f"  Max variance: {np.max(variances):.6f}")
        print(f"  Mean variance: {np.mean(variances):.6f}")
        print(f"  Min variance: {np.min(variances):.6f}")
        print(f"  Variance properly bounded: {np.max(variances) <= model.max_variance}")
    
    return results


async def test_adaptive_noise():
    """Test adaptive noise estimation behavior."""
    print("\n\nTesting adaptive noise estimation...")
    
    from mlserver.settings import ModelSettings
    settings = ModelSettings(
        name="kalman-filter",
        implementation="model.KalmanFilterModel"
    )
    model = KalmanFilterModel(settings)
    await model.load()
    
    # Track Q matrix evolution
    Q_history = []
    
    # Generate data with changing noise characteristics
    n_samples = 200
    t = np.arange(n_samples)
    
    # Low noise period, then high noise, then low again
    noise_schedule = np.concatenate([
        np.ones(50) * 0.01,
        np.ones(100) * 0.1,
        np.ones(50) * 0.01
    ])
    
    cpu_base = 0.5 + 0.1 * np.sin(0.1 * t)
    cpu_data = cpu_base + np.random.normal(0, noise_schedule)
    cpu_data = np.clip(cpu_data, 0.0, 1.0)
    
    memory_data = 0.6 * np.ones(n_samples) + np.random.normal(0, 0.05, n_samples)
    memory_data = np.clip(memory_data, 0.0, 1.0)
    
    load_data = 2.0 * np.ones(n_samples) + np.random.normal(0, 0.5, n_samples)
    load_data = np.clip(load_data, 0.0, 100.0)
    
    # Process in batches to track Q evolution
    batch_size = 10
    for i in range(0, n_samples, batch_size):
        batch_cpu = cpu_data[i:i+batch_size]
        batch_mem = memory_data[i:i+batch_size]
        batch_load = load_data[i:i+batch_size]
        
        inputs = [
            RequestInput(
                name="cpu_utilization",
                datatype="FP64",
                shape=[len(batch_cpu)],
                data=batch_cpu.tolist()
            ),
            RequestInput(
                name="memory_utilization",
                datatype="FP64",
                shape=[len(batch_mem)],
                data=batch_mem.tolist()
            ),
            RequestInput(
                name="load_average_1m",
                datatype="FP64",
                shape=[len(batch_load)],
                data=batch_load.tolist()
            )
        ]
        
        request = InferenceRequest(
            inputs=inputs,
            parameters=Parameters(content_type="np")
        )
        
        await model.predict(request)
        
        # Record Q matrix diagonal
        Q_history.append(np.diag(model.filter.Q).copy())
    
    Q_history = np.array(Q_history)
    
    print(f"Q matrix evolution:")
    print(f"  Initial Q[0,0]: {Q_history[0, 0]:.6f}")
    print(f"  Max Q[0,0]: {np.max(Q_history[:, 0]):.6f}")
    print(f"  Final Q[0,0]: {Q_history[-1, 0]:.6f}")
    print(f"  Q properly bounded: {np.max(Q_history) <= model.max_process_noise}")
    
    return Q_history, noise_schedule


def plot_results(variance_results, Q_history, noise_schedule):
    """Plot test results."""
    fig, axes = plt.subplots(2, 2, figsize=(12, 10))
    
    # Plot 1: Variance evolution for different scenarios
    ax = axes[0, 0]
    for name, data in variance_results.items():
        ax.plot(data['variances'], label=name, alpha=0.7)
    ax.axhline(y=10.0, color='r', linestyle='--', label='Max variance limit')
    ax.set_xlabel('Time step')
    ax.set_ylabel('Prediction Variance')
    ax.set_title('Variance Evolution by Scenario')
    ax.legend()
    ax.set_yscale('log')
    ax.grid(True, alpha=0.3)
    
    # Plot 2: Confidence scores
    ax = axes[0, 1]
    for name, data in variance_results.items():
        ax.plot(data['confidences'], label=name, alpha=0.7)
    ax.set_xlabel('Time step')
    ax.set_ylabel('Model Confidence')
    ax.set_title('Model Confidence by Scenario')
    ax.legend()
    ax.set_ylim(0, 1.1)
    ax.grid(True, alpha=0.3)
    
    # Plot 3: Q matrix adaptation
    ax = axes[1, 0]
    ax.plot(Q_history[:, 0], label='Q[0,0] (CPU state)', linewidth=2)
    ax.plot(Q_history[:, 1], label='Q[1,1] (CPU trend)', linewidth=2)
    ax2 = ax.twinx()
    ax2.plot(np.repeat(noise_schedule[::10], 1)[:len(Q_history)], 
             'r--', alpha=0.5, label='Input noise level')
    ax.set_xlabel('Batch number')
    ax.set_ylabel('Process noise (Q diagonal)')
    ax2.set_ylabel('Input noise level', color='r')
    ax.set_title('Adaptive Process Noise Evolution')
    ax.legend(loc='upper left')
    ax2.legend(loc='upper right')
    ax.grid(True, alpha=0.3)
    
    # Plot 4: Summary statistics
    ax = axes[1, 1]
    scenarios = list(variance_results.keys())
    max_vars = [variance_results[s]['max_variance'] for s in scenarios]
    mean_vars = [variance_results[s]['mean_variance'] for s in scenarios]
    
    x = np.arange(len(scenarios))
    width = 0.35
    
    ax.bar(x - width/2, max_vars, width, label='Max variance', alpha=0.7)
    ax.bar(x + width/2, mean_vars, width, label='Mean variance', alpha=0.7)
    ax.axhline(y=10.0, color='r', linestyle='--', label='Max limit')
    
    ax.set_xlabel('Scenario')
    ax.set_ylabel('Variance')
    ax.set_title('Variance Summary by Scenario')
    ax.set_xticks(x)
    ax.set_xticklabels(scenarios, rotation=45, ha='right')
    ax.legend()
    ax.set_yscale('log')
    ax.grid(True, alpha=0.3)
    
    plt.tight_layout()
    plt.savefig('kalman_variance_tuning_results.png', dpi=150)
    print("\nResults saved to kalman_variance_tuning_results.png")


async def main():
    """Run all tests."""
    print("=" * 60)
    print("Kalman Filter Variance Tuning Test")
    print("=" * 60)
    
    # Test 1: Variance bounds
    variance_results = await test_variance_bounds()
    
    # Test 2: Adaptive noise
    Q_history, noise_schedule = await test_adaptive_noise()
    
    # Plot results
    plot_results(variance_results, Q_history, noise_schedule)
    
    print("\n" + "=" * 60)
    print("Test Summary:")
    print("-" * 60)
    
    all_bounded = True
    for scenario, results in variance_results.items():
        if results['max_variance'] > 10.0:
            all_bounded = False
            print(f"❌ {scenario}: Max variance {results['max_variance']:.6f} exceeds limit!")
        else:
            print(f"✅ {scenario}: Max variance {results['max_variance']:.6f} within bounds")
    
    if np.max(Q_history) > 0.1:
        print(f"❌ Process noise: Max Q {np.max(Q_history):.6f} exceeds limit!")
    else:
        print(f"✅ Process noise: Max Q {np.max(Q_history):.6f} within bounds")
    
    print("-" * 60)
    if all_bounded and np.max(Q_history) <= 0.1:
        print("✅ All tests PASSED! Variance tuning is working correctly.")
    else:
        print("❌ Some tests FAILED. Further tuning needed.")


if __name__ == "__main__":
    asyncio.run(main())