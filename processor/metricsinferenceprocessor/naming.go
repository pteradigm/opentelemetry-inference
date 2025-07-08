// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsinferenceprocessor

import (
	"fmt"
	"sort"
	"strings"
)

// NamingConfig configures the intelligent naming options for output metrics
type NamingConfig struct {
	MaxStemParts           int  `mapstructure:"max_stem_parts"`
	SkipCommonDomains      bool `mapstructure:"skip_common_domains"`
	EnableCategoryGrouping bool `mapstructure:"enable_category_grouping"`
	AbbreviationThreshold  int  `mapstructure:"abbreviation_threshold"`
}

// DefaultNamingConfig returns the default naming configuration
func DefaultNamingConfig() NamingConfig {
	return NamingConfig{
		MaxStemParts:           2,
		SkipCommonDomains:      true,
		EnableCategoryGrouping: true,
		AbbreviationThreshold:  4,
	}
}

// GenerateIntelligentName generates an output metric name using intelligent naming
func GenerateIntelligentName(inputs []string, outputName string, modelName string, config NamingConfig) string {
	if len(inputs) == 0 {
		// If no model name either, just return output name
		if modelName == "" {
			return outputName
		}
		return fmt.Sprintf("%s.%s", modelName, outputName)
	}

	if len(inputs) == 1 {
		return generateSingleInputName(inputs[0], outputName, config)
	}

	// Multiple inputs - use intelligent prefix handling
	return generateMultiInputName(inputs, outputName, config)
}

func generateSingleInputName(input string, outputName string, config NamingConfig) string {
	parts := strings.Split(input, ".")

	// Extract semantic stem based on part count
	stem := extractSemanticStem(parts, config)
	return fmt.Sprintf("%s.%s", stem, outputName)
}

func extractSemanticStem(parts []string, config NamingConfig) string {
	if len(parts) == 0 {
		return ""
	}

	if len(parts) == 1 {
		return parts[0]
	}

	// For 2+ parts, use intelligent extraction
	originalParts := parts
	if config.SkipCommonDomains && len(parts) > 2 {
		parts = skipCommonDomainPrefix(parts)
	}

	// If we removed all parts, use original
	if len(parts) == 0 {
		parts = originalParts
	}

	// For 2 parts, always use both
	if len(parts) == 2 {
		return strings.Join(parts, "_")
	}

	// Take the most meaningful parts (usually last N)
	maxParts := config.MaxStemParts
	if maxParts <= 0 {
		maxParts = 2
	}

	if len(parts) > maxParts {
		parts = parts[len(parts)-maxParts:]
	}

	return strings.Join(parts, "_")
}

func skipCommonDomainPrefix(parts []string) []string {
	if len(parts) <= 2 {
		return parts
	}

	commonDomains := map[string]bool{
		"system":    true,
		"app":       true,
		"service":   true,
		"network":   true,
		"container": true,
		"process":   true,
		"host":      true,
		"cloud":     true,
		"k8s":       true,
	}

	if commonDomains[parts[0]] {
		return parts[1:]
	}
	return parts
}

func generateMultiInputName(inputs []string, outputName string, config NamingConfig) string {
	// Find common prefix
	prefix := findCommonPrefix(inputs)

	// Extract unique parts from each input
	var uniqueParts []string
	for _, input := range inputs {
		parts := strings.Split(input, ".")

		// Remove common prefix
		if prefix != "" {
			prefixParts := strings.Split(prefix, ".")
			if len(parts) >= len(prefixParts) {
				parts = parts[len(prefixParts):]
			}
		}

		// Get semantic stem from remaining parts
		if len(parts) > 0 {
			stem := extractSemanticStem(parts, config)
			if stem != "" && !contains(uniqueParts, stem) {
				uniqueParts = append(uniqueParts, stem)
			}
		}
	}

	// If no unique parts after prefix removal, use the full inputs
	if len(uniqueParts) == 0 {
		for _, input := range inputs {
			parts := strings.Split(input, ".")
			stem := extractSemanticStem(parts, config)
			if stem != "" && !contains(uniqueParts, stem) {
				uniqueParts = append(uniqueParts, stem)
			}
		}
	}

	// Construct output name
	var baseName string
	threshold := config.AbbreviationThreshold
	if threshold <= 0 {
		threshold = 4
	}

	if len(uniqueParts) <= threshold {
		baseName = strings.Join(uniqueParts, "_")
	} else {
		// Too many parts - use intelligent abbreviation
		baseName = abbreviateMultipleInputs(uniqueParts, prefix, config)
	}

	return fmt.Sprintf("%s.%s", baseName, outputName)
}

func findCommonPrefix(inputs []string) string {
	if len(inputs) < 2 {
		return ""
	}

	// Split all inputs into parts
	allParts := make([][]string, len(inputs))
	minLen := len(strings.Split(inputs[0], "."))
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

func abbreviateMultipleInputs(parts []string, prefix string, config NamingConfig) string {
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

	// Strategy 2: Group by categories if enabled
	if config.EnableCategoryGrouping {
		categories := categorizeInputs(parts)
		if len(categories) > 1 && len(categories) <= 3 {
			return formatCategorizedInputs(categories)
		}
	}

	// Strategy 3: Use first significant chars from each input
	return abbreviateParts(parts)
}

func categorizeInputs(parts []string) map[string][]string {
	categories := make(map[string][]string)

	// Common categories in metrics
	categoryPatterns := map[string][]string{
		"cpu":  {"cpu", "processor", "core"},
		"mem":  {"memory", "mem", "heap", "ram"},
		"net":  {"network", "net", "tcp", "udp", "http", "request", "response"},
		"disk": {"disk", "filesystem", "storage", "io", "volume"},
		"app":  {"app", "application", "service", "api", "endpoint"},
		"db":   {"database", "db", "sql", "query", "transaction"},
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

func formatCategorizedInputs(categories map[string][]string) string {
	// Sort categories for consistent output
	var catKeys []string
	for k := range categories {
		catKeys = append(catKeys, k)
	}
	sort.Strings(catKeys)

	var catNames []string
	for _, cat := range catKeys {
		items := categories[cat]
		if len(items) == 1 {
			catNames = append(catNames, items[0])
		} else {
			catNames = append(catNames, fmt.Sprintf("%s%d", cat, len(items)))
		}
	}
	return strings.Join(catNames, "_")
}

func abbreviateParts(parts []string) string {
	var abbreviated []string
	for i, part := range parts {
		if i >= 4 {
			// Limit to 4 parts and add count
			abbreviated = append(abbreviated, fmt.Sprintf("plus%d", len(parts)-4))
			break
		}
		// Skip empty parts
		if part == "" {
			continue
		}
		// Take first 4 chars or whole part if shorter
		if len(part) > 4 {
			abbreviated = append(abbreviated, part[:4])
		} else {
			abbreviated = append(abbreviated, part)
		}
	}
	return strings.Join(abbreviated, "_")
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}