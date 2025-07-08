// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsinferenceprocessor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// PatternEvaluator evaluates output naming patterns
type PatternEvaluator struct {
	pattern string
	rule    *internalRule
}

// NewPatternEvaluator creates a new pattern evaluator
func NewPatternEvaluator(pattern string, rule *internalRule) *PatternEvaluator {
	return &PatternEvaluator{
		pattern: pattern,
		rule:    rule,
	}
}

// Evaluate processes the pattern and returns the final metric name
func (pe *PatternEvaluator) Evaluate(outputName string) (string, error) {
	result := pe.pattern

	// Replace {output} with the actual output name
	result = strings.ReplaceAll(result, "{output}", outputName)

	// Replace {model} with the model name
	result = strings.ReplaceAll(result, "{model}", pe.rule.modelName)

	// Replace {version} with the model version
	result = strings.ReplaceAll(result, "{version}", pe.rule.modelVersion)

	// Replace {input} and {input[N]} patterns
	result = pe.replaceInputVariables(result)

	// Check for any remaining unreplaced variables
	if strings.Contains(result, "{") && strings.Contains(result, "}") {
		// Extract the variable name for better error message
		start := strings.Index(result, "{")
		end := strings.Index(result[start:], "}") + start
		if end > start {
			varName := result[start+1 : end]
			return "", fmt.Errorf("undefined variable: %s", varName)
		}
		return "", fmt.Errorf("invalid pattern: contains unreplaced variables")
	}

	return result, nil
}

// replaceInputVariables handles {input} and {input[N]} replacements
func (pe *PatternEvaluator) replaceInputVariables(pattern string) string {
	result := pattern

	// Replace {input} with {input[0]} for consistency
	result = strings.ReplaceAll(result, "{input}", "{input[0]}")

	// Regular expression to match {input[N]}
	inputRegex := regexp.MustCompile(`\{input\[(\d+)\]\}`)

	// Find all matches
	matches := inputRegex.FindAllStringSubmatch(result, -1)

	// Replace each match
	for _, match := range matches {
		if len(match) >= 2 {
			indexStr := match[1]
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				continue
			}

			// Check if index is valid
			if index >= 0 && index < len(pe.rule.inputs) {
				replacement := pe.rule.inputs[index]
				result = strings.ReplaceAll(result, match[0], replacement)
			} else {
				// Invalid index, use first input as fallback
				if len(pe.rule.inputs) > 0 {
					replacement := pe.rule.inputs[0]
					result = strings.ReplaceAll(result, match[0], replacement)
				}
			}
		}
	}

	return result
}

// validateOutputPattern validates the pattern syntax at configuration time
func validateOutputPattern(pattern string) error {
	if pattern == "" {
		return nil
	}

	// Check for balanced braces
	openCount := strings.Count(pattern, "{")
	closeCount := strings.Count(pattern, "}")
	if openCount != closeCount {
		return fmt.Errorf("unbalanced braces in pattern")
	}

	// Check for valid variable names
	validVars := map[string]bool{
		"output":  true,
		"model":   true,
		"version": true,
		"input":   true,
	}

	// Also allow input[N] patterns
	inputArrayRegex := regexp.MustCompile(`input\[\d+\]`)

	// Find all variables in the pattern
	varRegex := regexp.MustCompile(`\{([^}]+)\}`)
	matches := varRegex.FindAllStringSubmatch(pattern, -1)

	for _, match := range matches {
		if len(match) >= 2 {
			varName := match[1]
			// Check if it's a valid variable or matches input[N] pattern
			if !validVars[varName] && !inputArrayRegex.MatchString(varName) {
				return fmt.Errorf("invalid variable: %s", varName)
			}
		}
	}

	return nil
}