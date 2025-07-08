#!/usr/bin/env python3
"""Simple accuracy check for Kalman Filter"""

import requests
import json

# Get prediction
pred_resp = requests.get('http://localhost:8428/api/v1/query?query=system_memory_utilization_multi_cpu_prediction')
pred_val = float(pred_resp.json()['data']['result'][0]['value'][1])

# Get actual CPU (average across all CPUs)
cpu_resp = requests.get('http://localhost:8428/api/v1/query?query=avg(system_cpu_utilization_ratio{state="user"})')
cpu_data = cpu_resp.json()

# Get confidence
conf_resp = requests.get('http://localhost:8428/api/v1/query?query=system_memory_utilization_multi_model_confidence')
conf_val = float(conf_resp.json()['data']['result'][0]['value'][1])

print("🎯 Kalman Filter Quick Check")
print("=" * 40)
print(f"Predicted CPU: {pred_val:.1%}")

# Try to get actual CPU - might need to aggregate
if cpu_data['data']['result']:
    cpu_val = float(cpu_data['data']['result'][0]['value'][1])
    print(f"Actual CPU:    {cpu_val:.1%}")
    error = abs(pred_val - cpu_val)
    accuracy = 100 - (error / max(cpu_val, 0.01) * 100)
    print(f"Error:         {error:.1%}")
    print(f"Accuracy:      {accuracy:.1f}%")
else:
    # Try direct query without avg
    direct_resp = requests.get('http://localhost:8428/api/v1/query?query=system_cpu_utilization_ratio{state="user"}')
    direct_data = direct_resp.json()
    if direct_data['data']['result']:
        # Average manually
        cpu_values = [float(r['value'][1]) for r in direct_data['data']['result']]
        cpu_val = sum(cpu_values) / len(cpu_values)
        print(f"Actual CPU:    {cpu_val:.1%} (avg of {len(cpu_values)} CPUs)")
        error = abs(pred_val - cpu_val)
        accuracy = 100 - (error / max(cpu_val, 0.01) * 100)
        print(f"Error:         {error:.1%}")
        print(f"Accuracy:      {accuracy:.1f}%")

print(f"Confidence:    {conf_val:.1%}")
print("=" * 40)