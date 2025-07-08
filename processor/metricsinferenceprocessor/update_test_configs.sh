#!/bin/bash

# Script to add output_pattern to test configs for backward compatibility

# Function to add output_pattern after inputs line
add_output_pattern() {
    local file=$1
    local pattern=$2
    
    # Check if file already has output_pattern
    if grep -q "output_pattern:" "$file"; then
        echo "Skipping $file - already has output_pattern"
        return
    fi
    
    # Add output_pattern after inputs line
    sed -i '/inputs:.*\]/a\      output_pattern: "'"$pattern"'"' "$file"
    echo "Updated $file with pattern: $pattern"
}

# Update basic inference configs
add_output_pattern "testdata/basic_inference/config.yaml" "{output}"

# Update data types configs to preserve exact names
add_output_pattern "testdata/data_types/config.yaml" "{output}"

# Update input metric types configs
add_output_pattern "testdata/input_metric_types/config.yaml" "{output}"

# Update multi model configs
add_output_pattern "testdata/multi_model/config.yaml" "{output}"

# Update error handling configs
add_output_pattern "testdata/error_handling/config.yaml" "{output}"

echo "Done updating test configurations"