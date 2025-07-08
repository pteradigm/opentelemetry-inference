//go:build integration

package metricsinferenceprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// TestMLServerNamingStrategies tests both intelligent naming and output patterns
// with a real MLServer instance
func TestMLServerNamingStrategies(t *testing.T) {
	// This test requires MLServer to be running on localhost:9081
	// Start it with: cd testenv && podman-compose up -d

	tests := []struct {
		name          string
		rule          Rule
		inputMetric   string
		inputValue    float64
		expectedName  string
		description   string
	}{
		// Intelligent naming tests
		{
			name: "intelligent_single_input",
			rule: Rule{
				ModelName: "simple-scaler",
				Inputs:    []string{"cpu.usage"},
				// No OutputPattern - use intelligent naming
				Outputs: []OutputSpec{
					{Name: "scaled_output"},
				},
			},
			inputMetric:  "cpu.usage",
			inputValue:   50.0,
			expectedName: "cpu_usage.scaled_output",
			description:  "Intelligent naming with single input",
		},
		{
			name: "intelligent_complex_input",
			rule: Rule{
				ModelName: "simple-scaler",
				Inputs:    []string{"system.cpu.utilization"},
				// No OutputPattern
				Outputs: []OutputSpec{
					{Name: "predicted"},
				},
			},
			inputMetric:  "system.cpu.utilization",
			inputValue:   75.0,
			expectedName: "cpu_utilization.predicted",
			description:  "Intelligent naming simplifies complex input names",
		},
		// Output pattern tests
		{
			name: "pattern_exact_output",
			rule: Rule{
				ModelName:     "simple-scaler",
				Inputs:        []string{"memory.usage"},
				OutputPattern: "{output}",
				Outputs: []OutputSpec{
					{Name: "memory.scaled"},
				},
			},
			inputMetric:  "memory.usage",
			inputValue:   30.0,
			expectedName: "memory.scaled",
			description:  "Output pattern preserves exact name",
		},
		{
			name: "pattern_with_model",
			rule: Rule{
				ModelName:     "simple-scaler",
				Inputs:        []string{"disk.usage"},
				OutputPattern: "{model}.{output}",
				Outputs: []OutputSpec{
					{Name: "result"},
				},
			},
			inputMetric:  "disk.usage",
			inputValue:   80.0,
			expectedName: "simple-scaler.result",
			description:  "Output pattern with model placeholder",
		},
		{
			name: "pattern_with_input",
			rule: Rule{
				ModelName:     "simple-scaler",
				Inputs:        []string{"network.throughput"},
				OutputPattern: "{input}.scaled",
				Outputs: []OutputSpec{
					{Name: "value"},
				},
			},
			inputMetric:  "network.throughput",
			inputValue:   1000.0,
			expectedName: "network.throughput.scaled",
			description:  "Output pattern with input placeholder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create processor config
			cfg := &Config{
				GRPCClientSettings: GRPCClientSettings{
					Endpoint: "localhost:9081", // MLServer gRPC endpoint
				},
				Rules: []Rule{tt.rule},
			}

			// Create processor
			sink := new(consumertest.MetricsSink)
			processor, err := newMetricsProcessor(cfg, sink, zap.NewNop())
			require.NoError(t, err)

			err = processor.Start(context.Background(), nil)
			require.NoError(t, err)
			defer processor.Shutdown(context.Background())

			// Create input metric
			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()
			sm := rm.ScopeMetrics().AppendEmpty()
			metric := sm.Metrics().AppendEmpty()
			metric.SetName(tt.inputMetric)
			metric.SetEmptyGauge()
			dp := metric.Gauge().DataPoints().AppendEmpty()
			dp.SetDoubleValue(tt.inputValue)

			// Process metrics
			err = processor.ConsumeMetrics(context.Background(), md)
			require.NoError(t, err)

			// Verify output
			allMetrics := sink.AllMetrics()
			require.Len(t, allMetrics, 1)

			// Find the output metric
			found := false
			var actualValue float64
			result := allMetrics[0]
			for i := 0; i < result.ResourceMetrics().Len(); i++ {
				rm := result.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						metric := sm.Metrics().At(k)
						if metric.Name() == tt.expectedName {
							found = true
							// Verify the value is scaled by 2 (simple-scaler multiplies by 2)
							if metric.Type() == pmetric.MetricTypeGauge {
								dp := metric.Gauge().DataPoints().At(0)
								actualValue = dp.DoubleValue()
							}
							break
						}
					}
				}
			}

			assert.True(t, found, 
				"%s: Expected metric '%s' not found. Description: %s", 
				tt.name, tt.expectedName, tt.description)
			
			// Verify the scaled value (simple-scaler multiplies by 2)
			if found {
				expectedValue := tt.inputValue * 2.0
				assert.InDelta(t, expectedValue, actualValue, 0.01,
					"%s: Expected scaled value %f, got %f",
					tt.name, expectedValue, actualValue)
			}
		})
	}
}

// TestMLServerMultiInputNaming tests naming with multiple inputs using simple-sum model
func TestMLServerMultiInputNaming(t *testing.T) {
	tests := []struct {
		name         string
		rule         Rule
		expectedName string
	}{
		{
			name: "intelligent_multi_input",
			rule: Rule{
				ModelName: "simple-sum",
				Inputs:    []string{"cpu.usage", "memory.usage"},
				// No OutputPattern
				Outputs: []OutputSpec{
					{Name: "total"},
				},
			},
			expectedName: "cpu_usage_memory_usage.total",
		},
		{
			name: "pattern_multi_input",
			rule: Rule{
				ModelName:     "simple-sum",
				Inputs:        []string{"cpu.usage", "memory.usage"},
				OutputPattern: "system.{output}",
				Outputs: []OutputSpec{
					{Name: "resource_total"},
				},
			},
			expectedName: "system.resource_total",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				GRPCClientSettings: GRPCClientSettings{
					Endpoint: "localhost:9081",
				},
				Rules: []Rule{tt.rule},
			}

			sink := new(consumertest.MetricsSink)
			processor, err := newMetricsProcessor(cfg, sink, zap.NewNop())
			require.NoError(t, err)

			err = processor.Start(context.Background(), nil)
			require.NoError(t, err)
			defer processor.Shutdown(context.Background())

			// Create input metrics
			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()
			sm := rm.ScopeMetrics().AppendEmpty()
			
			// CPU metric
			metric1 := sm.Metrics().AppendEmpty()
			metric1.SetName("cpu.usage")
			metric1.SetEmptyGauge()
			dp1 := metric1.Gauge().DataPoints().AppendEmpty()
			dp1.SetDoubleValue(40.0)
			
			// Memory metric
			metric2 := sm.Metrics().AppendEmpty()
			metric2.SetName("memory.usage")
			metric2.SetEmptyGauge()
			dp2 := metric2.Gauge().DataPoints().AppendEmpty()
			dp2.SetDoubleValue(60.0)

			// Process
			err = processor.ConsumeMetrics(context.Background(), md)
			require.NoError(t, err)

			// Check output
			allMetrics := sink.AllMetrics()
			require.Len(t, allMetrics, 1)

			// Find the output metric
			found := false
			result := allMetrics[0]
			for i := 0; i < result.ResourceMetrics().Len(); i++ {
				rm := result.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						metric := sm.Metrics().At(k)
						if metric.Name() == tt.expectedName {
							found = true
							// Verify sum value
							if metric.Type() == pmetric.MetricTypeGauge {
								dp := metric.Gauge().DataPoints().At(0)
								sumValue := dp.DoubleValue()
								assert.InDelta(t, 100.0, sumValue, 0.01, 
									"Expected sum of 40+60=100, got %f", sumValue)
							}
							break
						}
					}
				}
			}

			assert.True(t, found, "Expected metric '%s' not found", tt.expectedName)
		})
	}
}