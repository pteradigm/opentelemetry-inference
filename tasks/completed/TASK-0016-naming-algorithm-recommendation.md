# Automated Metric Naming Algorithm Recommendation

## Executive Summary

Based on analysis of the codebase and observability best practices, I recommend an intelligent prefix-aware naming algorithm that:
1. Detects and factors out common prefixes
2. Preserves semantic meaning from metric hierarchies
3. Creates concise yet descriptive output names
4. Handles both single and multiple input scenarios gracefully

## Analysis of Current Metric Patterns

### Common Metric Hierarchies
```
system.cpu.utilization      → domain.component.measurement
system.memory.usage         → domain.component.measurement
app.api.latency            → domain.service.measurement
network.bytes_per_second   → domain.measurement
```

### Key Observations
1. **Domain prefixes** (system, app, network) group related metrics
2. **Component names** (cpu, memory, api) identify specific subsystems
3. **Measurement types** (utilization, usage, latency) describe what's measured
4. **Most semantic meaning** is in the last 2-3 components

## Recommended Algorithm: Intelligent Prefix Extraction

### Core Principles
1. **Common Prefix Detection**: When multiple inputs share prefixes, factor them out
2. **Semantic Preservation**: Keep the most meaningful parts (usually last 2-3 components)
3. **Context Awareness**: Use model name or operation type to add context
4. **Brevity**: Avoid redundant repetition while maintaining clarity

### Algorithm Implementation

```go
func generateOutputName(inputs []string, outputName string, modelName string) string {
    if len(inputs) == 0 {
        return fmt.Sprintf("%s.%s", modelName, outputName)
    }
    
    if len(inputs) == 1 {
        return generateSingleInputName(inputs[0], outputName)
    }
    
    // Multiple inputs - use intelligent prefix handling
    return generateMultiInputName(inputs, outputName)
}

func generateSingleInputName(input string, outputName string) string {
    parts := strings.Split(input, ".")
    
    // Extract semantic stem based on part count
    var stem string
    switch len(parts) {
    case 1:
        stem = parts[0]
    case 2:
        stem = strings.Join(parts, "_")
    default:
        // For 3+ parts, analyze semantic weight
        stem = extractSemanticStem(parts)
    }
    
    return fmt.Sprintf("%s.%s", stem, outputName)
}

func extractSemanticStem(parts []string) string {
    // Heuristic: domain prefixes are less specific than components
    commonDomains := map[string]bool{
        "system": true, "app": true, "service": true, 
        "network": true, "container": true, "process": true,
    }
    
    // Skip common domain prefix if present
    startIdx := 0
    if len(parts) > 2 && commonDomains[parts[0]] {
        startIdx = 1
    }
    
    // Take up to 2 most specific parts
    endIdx := len(parts)
    if endIdx - startIdx > 2 {
        startIdx = endIdx - 2
    }
    
    return strings.Join(parts[startIdx:endIdx], "_")
}

func generateMultiInputName(inputs []string, outputName string) string {
    // Find common prefix
    prefix := findCommonPrefix(inputs)
    
    // Extract unique parts from each input
    var uniqueParts []string
    for _, input := range inputs {
        parts := strings.Split(input, ".")
        
        // Remove common prefix
        if prefix != "" {
            prefixParts := strings.Split(prefix, ".")
            parts = parts[len(prefixParts):]
        }
        
        // Get semantic stem from remaining parts
        stem := extractSemanticStem(parts)
        if stem != "" && !contains(uniqueParts, stem) {
            uniqueParts = append(uniqueParts, stem)
        }
    }
    
    // Construct output name
    var baseName string
    if len(uniqueParts) <= 3 {
        baseName = strings.Join(uniqueParts, "_")
    } else {
        // Too many parts - use intelligent abbreviation
        baseName = abbreviateMultipleInputs(uniqueParts, prefix)
    }
    
    return fmt.Sprintf("%s.%s", baseName, outputName)
}

func abbreviateMultipleInputs(parts []string, prefix string) string {
    // Strategy 1: If there's a common prefix, use it as base
    if prefix != "" {
        prefixBase := strings.Replace(prefix, ".", "_", -1)
        
        // If not too many parts, just concatenate
        if len(parts) <= 5 {
            return fmt.Sprintf("%s_%s", prefixBase, strings.Join(parts, "_"))
        }
        
        // Otherwise use initials approach
        var initials []string
        for _, part := range parts {
            if len(part) > 0 {
                initials = append(initials, string(part[0]))
            }
        }
        return fmt.Sprintf("%s_%s", prefixBase, strings.Join(initials, ""))
    }
    
    // Strategy 2: Group by categories if possible
    categories := categorizeInputs(parts)
    if len(categories) > 1 && len(categories) <= 3 {
        var catNames []string
        for cat, items := range categories {
            if len(items) == 1 {
                catNames = append(catNames, items[0])
            } else {
                catNames = append(catNames, fmt.Sprintf("%s%d", cat, len(items)))
            }
        }
        return strings.Join(catNames, "_")
    }
    
    // Strategy 3: Use first significant word from each input
    var abbreviated []string
    for i, part := range parts {
        if i >= 4 {
            // Limit to 4 parts and add count
            abbreviated = append(abbreviated, fmt.Sprintf("plus%d", len(parts)-4))
            break
        }
        // Take first component or abbreviated version
        components := strings.Split(part, "_")
        if len(components[0]) > 4 {
            abbreviated = append(abbreviated, components[0][:4])
        } else {
            abbreviated = append(abbreviated, components[0])
        }
    }
    
    return strings.Join(abbreviated, "_")
}

func categorizeInputs(parts []string) map[string][]string {
    categories := make(map[string][]string)
    
    // Common categories in metrics
    categoryPatterns := map[string][]string{
        "cpu": {"cpu", "processor", "core"},
        "mem": {"memory", "mem", "heap", "ram"},
        "net": {"network", "net", "tcp", "udp", "http"},
        "disk": {"disk", "filesystem", "storage", "io"},
        "app": {"app", "application", "service", "api"},
        "db": {"database", "db", "sql", "query"},
    }
    
    for _, part := range parts {
        categorized := false
        lowerPart := strings.ToLower(part)
        
        for category, patterns := range categoryPatterns {
            for _, pattern := range patterns {
                if strings.Contains(lowerPart, pattern) {
                    categories[category] = append(categories[category], part)
                    categorized = true
                    break
                }
            }
            if categorized {
                break
            }
        }
        
        // If not categorized, use first 3 chars as category
        if !categorized {
            key := part
            if len(part) > 3 {
                key = part[:3]
            }
            categories[key] = append(categories[key], part)
        }
    }
    
    return categories
}

func findCommonPrefix(inputs []string) string {
    if len(inputs) < 2 {
        return ""
    }
    
    // Split all inputs into parts
    allParts := make([][]string, len(inputs))
    minLen := math.MaxInt
    for i, input := range inputs {
        allParts[i] = strings.Split(input, ".")
        if len(allParts[i]) < minLen {
            minLen = len(allParts[i])
        }
    }
    
    // Find common prefix parts
    var commonParts []string
    for i := 0; i < minLen; i++ {
        part := allParts[0][i]
        allMatch := true
        for j := 1; j < len(allParts); j++ {
            if allParts[j][i] != part {
                allMatch = false
                break
            }
        }
        if allMatch {
            commonParts = append(commonParts, part)
        } else {
            break
        }
    }
    
    return strings.Join(commonParts, ".")
}
```

### Examples

#### Single Input Cases
```
system.cpu.utilization → cpu_utilization.prediction
app.api.latency → api_latency.response_time
network.bytes_per_second → bytes_per_second.scaled
temperature → temperature.predicted
```

#### Multiple Inputs - Common Prefix
```
Inputs: [system.cpu.utilization, system.memory.utilization]
Common prefix: system
Output: cpu_memory.anomaly_score

Inputs: [app.api.requests, app.api.errors, app.api.latency]
Common prefix: app.api
Output: requests_errors_latency.health_score
```

#### Multiple Inputs - No Common Prefix
```
Inputs: [cpu.usage, memory.usage, disk.io]
No common prefix
Output: cpu_memory_disk.correlation

Inputs: [app.requests, network.bytes, system.load, db.connections]
No common prefix, too many parts
Output: app_netw_syst_db.prediction

Inputs: [service.api.requests, service.auth.failures, service.db.latency, service.cache.hits, service.queue.depth]
Common prefix: service, but still manageable
Output: service_api_auth_db_cache_queue.analysis

Inputs: [cpu.user, cpu.system, memory.used, memory.free, disk.read, disk.write, network.in, network.out]
Multiple categories detected
Output: cpu2_mem2_disk2_net2.resource_score (grouped by category)

Inputs: [app.frontend.requests, app.backend.requests, app.api.requests, db.queries, cache.hits]
Mixed categories
Output: app3_db_cache.performance
```

## Edge Cases and Special Handling

### Very Long Metric Names
```
Input: organization.department.team.service.component.subcomponent.measurement
Output: subcomponent_measurement.predicted (takes last 2 meaningful parts)
```

### Single Word Metrics
```
Input: temperature
Output: temperature.scaled
```

### Metrics with Numbers
```
Inputs: [cpu0.usage, cpu1.usage, cpu2.usage, cpu3.usage]
Common prefix detected as "cpu"
Output: cpu_0123.aggregated (combines numeric suffixes)
```

### Mixed Naming Conventions
```
Inputs: [systemCpuUsage, system.memory.usage, system-disk-usage]
Normalized first, then processed
Output: cpu_usage_memory_usage_disk_usage.combined
```

## Alternative Approaches Considered

### 1. Hash-Based Naming
```
Inputs: [system.cpu.utilization, system.memory.usage]
Output: sys_7a8b9c.prediction
```
**Pros**: Guaranteed unique, short
**Cons**: Not human-readable, loses semantic meaning

### 2. Full Concatenation
```
Inputs: [system.cpu.utilization, system.memory.usage]
Output: system_cpu_utilization_system_memory_usage.prediction
```
**Pros**: Preserves all information
**Cons**: Too verbose, redundant

### 3. Model-Centric Naming
```
Inputs: [any...]
Output: anomaly_detector.score
```
**Pros**: Simple, consistent
**Cons**: Not unique when same model used multiple times

## Configuration Options

The algorithm should support configuration for:

```yaml
processors:
  metricsinference:
    naming_strategy:
      # Strategy: "intelligent" (default), "full", "model_centric", "custom"
      type: "intelligent"
      
      # For intelligent strategy
      options:
        max_parts: 2                    # Max parts to keep from each input
        skip_common_domains: true       # Skip system/app/etc prefixes
        common_prefix_threshold: 0.5    # Min ratio to consider prefix common
        max_output_parts: 3             # Max parts in final output name
        fallback_name: "multi_input"    # When too complex
```

## Benefits of This Approach

1. **Human Readable**: Output names are descriptive and meaningful
2. **Concise**: Avoids redundancy while preserving key information  
3. **Predictable**: Developers can anticipate output names
4. **Flexible**: Handles various input patterns gracefully
5. **Configurable**: Can be tuned for different use cases

## Migration Strategy

1. Make new algorithm opt-in via configuration
2. Provide migration tool to update existing configurations
3. Support both old and new patterns during transition
4. Document clear examples and patterns

## Conclusion

The intelligent prefix extraction algorithm provides the best balance of:
- Semantic clarity
- Name brevity
- Handling of edge cases
- Flexibility for different scenarios

This approach will make metric names more intuitive while avoiding the verbosity of full concatenation or the ambiguity of overly simple schemes.