# TASK-0016 Design: Deterministic Output Metric Naming

## Current Implementation Analysis

### Current Naming Logic
The processor uses `decorateOutputName()` in processor.go:1461 with this logic:
- **Single input**: `{input_name}.{output_name}`
- **Multiple inputs**: `{first_input}_multi.{output_name}`
- **Fallback**: `{model_name}_{index}.{output_name}` or `output_{index}.{output_name}`

### Problems Identified
1. Long metric names when using namespaced inputs (e.g., `system.memory.utilization_multi.prediction`)
2. The `_multi` suffix is not descriptive
3. No user control over naming patterns
4. Can't extract parts of input names for cleaner output names
5. Default behavior creates unnecessarily long metric names

## Proposed Solution

### Configuration Changes

Add `output_pattern` field to the Rule struct in config.go:

```go
type Rule struct {
    // ... existing fields ...
    
    // OutputPattern specifies a template for generating output metric names.
    // If not specified, uses default behavior for backward compatibility.
    // Template variables:
    //   {input} or {input[0]} - First input metric name
    //   {input[N]} - Nth input metric name (0-based)
    //   {output} - Output tensor name from model
    //   {model} - Model name
    //   {version} - Model version (empty string if not specified)
    // Example: "ml.{model}.{output}" → "ml.cpu_predictor.prediction"
    OutputPattern string `mapstructure:"output_pattern"`
}
```

### Pattern Syntax Design

#### Basic Template Variables
- `{input}` - First input metric name (alias for `{input[0]}`)
- `{input[N]}` - Nth input metric name (0-based index)
- `{output}` - Current output tensor name
- `{model}` - Model name from rule
- `{version}` - Model version (empty string if not specified)

#### Advanced Features (Phase 2)
- `{input:extract:regex}` - Extract substring using regex
- `{input[N]:extract:regex}` - Extract from specific input
- Conditional expressions: `{condition ? true_value : false_value}`

### Implementation Approach

#### Phase 1: Basic Template Support
1. Add `OutputPattern` field to Rule struct
2. Create `PatternEvaluator` type:
   ```go
   type PatternEvaluator struct {
       pattern string
       rule    *internalRule
   }
   
   func (pe *PatternEvaluator) Evaluate(outputName string) (string, error)
   ```

3. Update `decorateOutputName()` to use pattern if specified:
   ```go
   func (mp *metricsinferenceprocessor) decorateOutputName(rule *internalRule, outputName string, outputIndex int) string {
       if rule.outputPattern != "" {
           evaluator := &PatternEvaluator{
               pattern: rule.outputPattern,
               rule:    rule,
           }
           name, err := evaluator.Evaluate(outputName)
           if err != nil {
               // Log error and fall back to default behavior
               mp.logger.Warn("Failed to evaluate output pattern", 
                   zap.String("pattern", rule.outputPattern), 
                   zap.Error(err))
               return mp.defaultDecorateOutputName(rule, outputName, outputIndex)
           }
           return name
       }
       return mp.defaultDecorateOutputName(rule, outputName, outputIndex)
   }
   
   // New default naming logic
   func (mp *metricsinferenceprocessor) defaultDecorateOutputName(rule *internalRule, outputName string, outputIndex int) string {
       var stems []string
       
       for _, input := range rule.inputs {
           parts := strings.Split(input, ".")
           var stem string
           
           if len(parts) == 2 {
               // Use both parts joined by underscore
               stem = parts[0] + "_" + parts[1]
           } else if len(parts) >= 3 {
               // Use last three parts joined by underscore
               n := len(parts)
               stem = parts[n-3] + "_" + parts[n-2] + "_" + parts[n-1]
           } else {
               // Single part, use as is
               stem = parts[0]
           }
           
           stems = append(stems, stem)
       }
       
       // Join all stems with underscores and append output name
       return strings.Join(stems, "_") + "." + outputName
   }
   ```

#### Phase 2: Regex Extraction Support
Add support for extracting parts of input names:
```go
// Pattern: "{input:extract:system\\.(\\w+)}.{output}"
// Input: "system.cpu.utilization"
// Result: "cpu.prediction"
```

### Validation

Add validation in `Validate()` method:
```go
func (cfg *Config) Validate() error {
    // ... existing validation ...
    
    for i, rule := range cfg.Rules {
        // ... existing validation ...
        
        if rule.OutputPattern != "" {
            if err := validateOutputPattern(rule.OutputPattern); err != nil {
                return fmt.Errorf("invalid output_pattern in rule %d: %w", i, err)
            }
        }
    }
    return nil
}
```

### Examples

#### Example 1: Simple Model-Based Naming
```yaml
rules:
  - model_name: "cpu-predictor"
    inputs: ["system.cpu.utilization"]
    output_pattern: "ml.{model}.{output}"
    # Results in: "ml.cpu-predictor.prediction"
```

#### Example 2: Input-Based Naming
```yaml
rules:
  - model_name: "scaler"
    inputs: ["request.rate"]
    output_pattern: "{input}.scaled"
    # Results in: "request.rate.scaled"
```

#### Example 3: Multi-Input Custom Naming
```yaml
rules:
  - model_name: "correlator"
    inputs: ["cpu.usage", "memory.usage", "disk.io"]
    output_pattern: "system.health.{output}"
    # Results in: "system.health.anomaly_score"
```

#### Example 4: Extraction (Phase 2)
```yaml
rules:
  - model_name: "predictor"
    inputs: ["service.api.latency"]
    output_pattern: "{input:extract:service\\.(\\w+)}.predicted.{output}"
    # Results in: "api.predicted.p95"
```

### Default Naming Behavior (No Pattern Specified)

When `output_pattern` is not specified, the processor uses a new smart default:

1. Extract "stems" from each input metric name using the LAST parts
2. Apply the following rules:
   - If metric has 1 part: use as is
   - If metric has 2 parts: use both parts joined by underscore
   - If metric has ≥3 parts: use last three parts joined by underscore
3. Join all input stems with underscores
4. Append output name

#### Examples:
- Single input `system.cpu.utilization` → `system_cpu_utilization.prediction`
- Single input `cpu.usage` → `cpu_usage.prediction`
- Single input `request.rate` → `request_rate.prediction`
- Single input `temperature` → `temperature.prediction`
- Single input `app.service.api.requests.rate` → `api_requests_rate.prediction`
- Multiple inputs:
  - `["system.cpu.utilization", "system.memory.usage"]` → `system_cpu_utilization_system_memory_usage.prediction`
  - `["cpu.usage", "memory.usage"]` → `cpu_usage_memory_usage.prediction`
  - `["app.api.latency", "db.connections"]` → `app_api_latency_db_connections.prediction`
  - `["service.api.requests", "service.api.errors", "service.api.latency"]` → `api_requests_api_errors_api_latency.prediction`

Note: The last example shows why custom patterns are useful for multiple inputs with common prefixes.

### Error Handling

1. **Invalid Pattern**: Log warning and fall back to default naming
2. **Missing Variables**: Return error during validation
3. **Index Out of Bounds**: Log warning and use first input
4. **Regex Errors**: Log warning and skip extraction

### Testing Strategy

1. **Unit Tests**:
   - Pattern parsing and validation
   - Variable substitution
   - Error cases
   - Backward compatibility

2. **Integration Tests**:
   - End-to-end with various patterns
   - Performance with complex patterns
   - Multiple rules with different patterns

### Migration Path

1. **Breaking Change**: Default naming behavior changes
   - Old: `system.cpu.utilization.prediction`
   - New: `cpu_utilization.prediction`

2. Users who want old behavior can use explicit patterns:
   ```yaml
   # To maintain old single-input behavior
   output_pattern: "{input}.{output}"
   
   # To maintain old multi-input behavior  
   output_pattern: "{input}_multi.{output}"
   ```

3. Documentation will include:
   - Clear examples of new default behavior
   - Migration patterns for existing users
   - Benefits of shorter metric names

## Timeline

- **Week 1**: Implement basic template support (Phase 1)
- **Week 2**: Add validation and error handling
- **Week 3**: Write tests and documentation
- **Week 4**: Phase 2 features (regex extraction) - optional

## Open Questions

1. Should we support nested variables like `{model}.{version}`?
2. Should patterns be validated at config time or runtime?
3. Do we need escaping for literal `{` `}` characters?
4. Should we add a `{timestamp}` variable for unique names?