// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsinferenceprocessor

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateIntelligentName(t *testing.T) {
	config := DefaultNamingConfig()

	tests := []struct {
		name       string
		inputs     []string
		outputName string
		modelName  string
		expected   string
	}{
		// Single input tests
		{
			name:       "single input with 3 parts",
			inputs:     []string{"system.cpu.utilization"},
			outputName: "prediction",
			modelName:  "cpu-model",
			expected:   "cpu_utilization.prediction",
		},
		{
			name:       "single input with 2 parts",
			inputs:     []string{"cpu.usage"},
			outputName: "scaled",
			modelName:  "scaler",
			expected:   "cpu_usage.scaled",
		},
		{
			name:       "single input with 1 part",
			inputs:     []string{"temperature"},
			outputName: "celsius",
			modelName:  "converter",
			expected:   "temperature.celsius",
		},
		{
			name:       "single input with 4+ parts",
			inputs:     []string{"app.service.api.latency"},
			outputName: "p95",
			modelName:  "percentile",
			expected:   "api_latency.p95",
		},
		// Multiple inputs with common prefix
		{
			name:       "multiple inputs with system prefix",
			inputs:     []string{"system.cpu.utilization", "system.memory.usage"},
			outputName: "anomaly_score",
			modelName:  "anomaly-detector",
			expected:   "cpu_utilization_memory_usage.anomaly_score",
		},
		{
			name:       "multiple inputs with app.api prefix",
			inputs:     []string{"app.api.requests", "app.api.errors", "app.api.latency"},
			outputName: "health_score",
			modelName:  "health-checker",
			expected:   "requests_errors_latency.health_score",
		},
		// Multiple inputs without common prefix
		{
			name:       "multiple diverse inputs",
			inputs:     []string{"cpu.usage", "memory.usage", "disk.io"},
			outputName: "correlation",
			modelName:  "correlator",
			expected:   "cpu_usage_memory_usage_disk_io.correlation",
		},
		// No inputs
		{
			name:       "no inputs uses model name",
			inputs:     []string{},
			outputName: "result",
			modelName:  "predictor",
			expected:   "predictor.result",
		},
		// Very long metric names
		{
			name:       "very long single input",
			inputs:     []string{"organization.department.team.service.component.subcomponent.measurement"},
			outputName: "processed",
			modelName:  "processor",
			expected:   "subcomponent_measurement.processed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateIntelligentName(tt.inputs, tt.outputName, tt.modelName, config)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCategoryGrouping(t *testing.T) {
	config := DefaultNamingConfig()
	// Lower threshold to trigger abbreviation
	config.AbbreviationThreshold = 3

	tests := []struct {
		name       string
		inputs     []string
		outputName string
		expected   string
	}{
		{
			name:       "simple case without abbreviation",
			inputs:     []string{"cpu.user", "memory.used"},
			outputName: "resource_score",
			expected:   "cpu_user_memory_used.resource_score",
		},
		{
			name:       "triggers category grouping",
			inputs:     []string{"cpu.user", "cpu.system", "memory.used", "memory.free", "disk.read"},
			outputName: "resource_score",
			expected:   "cpu2_disk_read_mem2.resource_score",
		},
		{
			name: "mixed categories with abbreviation",
			inputs: []string{
				"app.frontend.requests",
				"app.backend.requests",
				"app.api.requests",
				"db.queries",
				"cache.hits",
			},
			outputName: "performance",
			expected:   "cache_hits_db_queries_net3.performance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateIntelligentName(tt.inputs, tt.outputName, "model", config)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAbbreviation(t *testing.T) {
	config := DefaultNamingConfig()
	config.AbbreviationThreshold = 3 // Lower threshold for testing

	tests := []struct {
		name       string
		inputs     []string
		outputName string
		validate   func(t *testing.T, result string)
	}{
		{
			name: "too many inputs without common prefix",
			inputs: []string{
				"app.requests",
				"network.bytes",
				"system.load",
				"db.connections",
			},
			outputName: "analysis",
			validate: func(t *testing.T, result string) {
				// Should contain abbreviated forms and end with .analysis
				assert.Contains(t, result, ".analysis")
				// Should be reasonably short
				assert.Less(t, len(result), 50)
				// Should contain some recognizable parts
				assert.True(t, 
					strings.Contains(result, "app") || 
					strings.Contains(result, "net") ||
					strings.Contains(result, "db"))
			},
		},
		{
			name: "too many inputs with common prefix",
			inputs: []string{
				"service.api.requests",
				"service.auth.failures",
				"service.db.latency",
				"service.cache.hits",
				"service.queue.depth",
			},
			outputName: "monitoring",
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, ".monitoring")
				assert.Contains(t, result, "service")
				// Should preserve the common prefix
				assert.Less(t, len(result), 80)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateIntelligentName(tt.inputs, tt.outputName, "model", config)
			tt.validate(t, result)
		})
	}
}

func TestFindCommonPrefix(t *testing.T) {
	tests := []struct {
		name     string
		inputs   []string
		expected string
	}{
		{
			name:     "system prefix",
			inputs:   []string{"system.cpu.utilization", "system.memory.usage"},
			expected: "system",
		},
		{
			name:     "multi-level prefix",
			inputs:   []string{"app.api.requests", "app.api.errors", "app.api.latency"},
			expected: "app.api",
		},
		{
			name:     "no common prefix",
			inputs:   []string{"cpu.usage", "memory.usage", "disk.io"},
			expected: "",
		},
		{
			name:     "single input",
			inputs:   []string{"system.cpu.utilization"},
			expected: "",
		},
		{
			name:     "identical inputs",
			inputs:   []string{"metric.name", "metric.name"},
			expected: "metric.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findCommonPrefix(tt.inputs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractSemanticStem(t *testing.T) {
	config := DefaultNamingConfig()

	tests := []struct {
		name     string
		parts    []string
		expected string
	}{
		{
			name:     "single part",
			parts:    []string{"temperature"},
			expected: "temperature",
		},
		{
			name:     "two parts",
			parts:    []string{"cpu", "usage"},
			expected: "cpu_usage",
		},
		{
			name:     "three parts with common domain",
			parts:    []string{"system", "cpu", "utilization"},
			expected: "cpu_utilization",
		},
		{
			name:     "four parts",
			parts:    []string{"app", "service", "api", "latency"},
			expected: "api_latency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSemanticStem(tt.parts, config)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCustomNamingConfig(t *testing.T) {
	// Test with custom configuration
	config := NamingConfig{
		MaxStemParts:           3,
		SkipCommonDomains:      false,
		EnableCategoryGrouping: false,
		AbbreviationThreshold:  2,
	}

	tests := []struct {
		name       string
		inputs     []string
		outputName string
		expected   string
	}{
		{
			name:       "three parts kept with custom config",
			inputs:     []string{"system.cpu.utilization.percentage"},
			outputName: "scaled",
			expected:   "cpu_utilization_percentage.scaled",
		},
		{
			name:       "common domain not skipped",
			inputs:     []string{"system.cpu.utilization"},
			outputName: "prediction",
			expected:   "system_cpu_utilization.prediction",
		},
		{
			name:       "abbreviation with low threshold",
			inputs:     []string{"metric1", "metric2", "metric3"},
			outputName: "combined",
			expected:   "metr_metr_metr.combined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateIntelligentName(tt.inputs, tt.outputName, "model", config)
			assert.Equal(t, tt.expected, result)
		})
	}
}