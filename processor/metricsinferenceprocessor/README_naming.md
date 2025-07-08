# Intelligent Metric Naming

The Metrics Inference Processor automatically generates meaningful, concise metric names that preserve semantic information while removing redundancy. The intelligent naming system works out-of-the-box with sensible defaults and offers simple configuration options for customization.

## Intelligent Naming

The intelligent naming algorithm automatically:
- Detects and removes common prefixes to avoid redundancy
- Preserves the most semantically meaningful parts of metric names
- Groups metrics by category when dealing with many inputs
- Creates human-readable abbreviated names when necessary

**No configuration required** - the system works great with default settings. Configure only when you need specific behavior.

### Configuration

```yaml
processors:
  metricsinference:
    grpc:
      endpoint: "localhost:8081"
    naming:
      max_stem_parts: 2              # Maximum parts to keep from each input (default: 2)
      skip_common_domains: true      # Skip common prefixes like "system", "app" (default: true)
      enable_category_grouping: true # Group by categories when many inputs (default: true)
      abbreviation_threshold: 4      # Number of inputs before abbreviation (default: 4)
```

## Examples

### Single Input Metrics

```yaml
# Configuration
rules:
  - model_name: "cpu-predictor"
    inputs: ["system.cpu.utilization"]

# Generated output name
cpu_utilization.prediction  # Removes redundant "system" prefix
```

### Multiple Inputs with Common Prefix

```yaml
# Configuration
rules:
  - model_name: "anomaly-detector"
    inputs: 
      - "system.cpu.utilization"
      - "system.memory.usage"
      - "system.disk.io"

# Generated output name
cpu_utilization_memory_usage_disk_io.anomaly_score
```

### Many Diverse Inputs

```yaml
# Configuration
rules:
  - model_name: "resource-analyzer"
    inputs:
      - "cpu.user"
      - "cpu.system" 
      - "memory.used"
      - "memory.free"
      - "disk.read"
      - "disk.write"

# Generated output name (with category grouping)
cpu2_disk2_mem2.analysis  # Groups by category with counts
```

### Very Long Metric Names

```yaml
# Configuration
rules:
  - model_name: "api-monitor"
    inputs: ["organization.department.team.service.api.latency"]

# Generated output name
api_latency.response_time  # Takes last 2 meaningful parts
```


## Custom Pattern Naming

You can also specify custom naming patterns per rule:

```yaml
rules:
  - model_name: "my-model"
    inputs: ["cpu.usage", "memory.usage"]
    output_pattern: "resources.{model}.{output}"
    
# Results in: resources.my-model.prediction
```

## Migration Guide

Since this processor hasn't launched yet, intelligent naming is the default and only option:

1. **Design dashboards and alerts** using the automatically generated metric names
2. **Test in your environment** to verify naming meets your requirements
3. **Use custom patterns** for any specific naming requirements the algorithm doesn't handle
4. **Leverage the improved readability** for simpler metric queries

## Best Practices

1. **Let the algorithm work**: The intelligent naming usually produces good results without configuration
2. **Adjust thresholds carefully**: Lower abbreviation thresholds create shorter names but may lose clarity
3. **Use output patterns** for specific requirements that the algorithm doesn't handle
4. **Monitor the results**: Check that generated names are meaningful in your context

## Troubleshooting

### Names are too long
- Decrease `max_stem_parts` to use fewer parts from each input
- Lower `abbreviation_threshold` to trigger abbreviation sooner

### Names are too abbreviated
- Increase `abbreviation_threshold` to allow more parts before abbreviation
- Disable `enable_category_grouping` if grouping is too aggressive

### Missing important context
- Disable `skip_common_domains` to keep prefixes like "system", "app"
- Increase `max_stem_parts` to preserve more of each metric name

### Need specific naming
- Use `output_pattern` in individual rules for full control