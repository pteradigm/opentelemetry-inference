// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// TestMetric represents test metric data with names and values
type TestMetric struct {
	MetricNames  []string
	MetricValues [][]float64
}

// TestMetricIntGauge represents test metric data with integer gauge values
type TestMetricIntGauge struct {
	MetricNames  []string
	MetricValues [][]int64
}

// GenerateTestMetrics creates pmetric.Metrics from TestMetric data
func GenerateTestMetrics(tm TestMetric) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	for i, name := range tm.MetricNames {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(name)
		metric.SetEmptyGauge()

		if i < len(tm.MetricValues) {
			for _, value := range tm.MetricValues[i] {
				dp := metric.Gauge().DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
				dp.SetDoubleValue(value)
			}
		}
	}

	return md
}

// GenerateTestMetricsIntGauge creates pmetric.Metrics with integer gauge values
func GenerateTestMetricsIntGauge(tm TestMetricIntGauge) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	for i, name := range tm.MetricNames {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(name)
		metric.SetEmptyGauge()

		if i < len(tm.MetricValues) {
			for _, value := range tm.MetricValues[i] {
				dp := metric.Gauge().DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
				dp.SetIntValue(value)
			}
		}
	}

	return md
}

// GenerateTestMetricsWithAttributes creates test metrics with custom attributes
func GenerateTestMetricsWithAttributes(tm TestMetric, attributes map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	for i, name := range tm.MetricNames {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(name)
		metric.SetEmptyGauge()

		if i < len(tm.MetricValues) {
			for _, value := range tm.MetricValues[i] {
				dp := metric.Gauge().DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
				dp.SetDoubleValue(value)

				// Add attributes
				for k, v := range attributes {
					dp.Attributes().PutStr(k, v)
				}
			}
		}
	}

	return md
}

// GenerateTestSumMetrics creates test sum metric data
func GenerateTestSumMetrics(tm TestMetric) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	for i, name := range tm.MetricNames {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(name)
		sum := metric.SetEmptySum()
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		sum.SetIsMonotonic(true)

		if i < len(tm.MetricValues) {
			for _, value := range tm.MetricValues[i] {
				dp := sum.DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
				dp.SetDoubleValue(value)
			}
		}
	}

	return md
}

// GenerateTestHistogramMetrics creates test histogram metric data
func GenerateTestHistogramMetrics(name string, count uint64, sum float64, bucketCounts []uint64, bounds []float64) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	metric := sm.Metrics().AppendEmpty()
	metric.SetName(name)
	hist := metric.SetEmptyHistogram()
	hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	dp := hist.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetCount(count)
	dp.SetSum(sum)

	// Set bucket counts
	for _, bucketCount := range bucketCounts {
		dp.BucketCounts().Append(bucketCount)
	}

	// Set explicit bounds
	for _, bound := range bounds {
		dp.ExplicitBounds().Append(bound)
	}

	return md
}

// TestMetricWithAttributes represents test metric data with attributes per data point
type TestMetricWithAttributes struct {
	MetricName string
	DataPoints []TestDataPoint
}

// TestDataPoint represents a single data point with value and attributes
type TestDataPoint struct {
	Value      float64
	IntValue   int64
	Attributes map[string]string
	Timestamp  time.Time
}

// GenerateTestMetricsMultiDataPoints creates metrics with multiple data points each with different attributes
func GenerateTestMetricsMultiDataPoints(metrics []TestMetricWithAttributes) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	now := time.Now()

	for _, tm := range metrics {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(tm.MetricName)
		gauge := metric.SetEmptyGauge()

		for _, dp := range tm.DataPoints {
			dataPoint := gauge.DataPoints().AppendEmpty()

			// Set timestamp
			if dp.Timestamp.IsZero() {
				dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(now))
			} else {
				dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(dp.Timestamp))
			}

			// Set value
			if dp.IntValue != 0 {
				dataPoint.SetIntValue(dp.IntValue)
			} else {
				dataPoint.SetDoubleValue(dp.Value)
			}

			// Set attributes
			for k, v := range dp.Attributes {
				dataPoint.Attributes().PutStr(k, v)
			}
		}
	}

	return md
}

// GenerateTestMetricsWithResource creates metrics with resource attributes
func GenerateTestMetricsWithResource(tm TestMetric, resourceAttrs map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()

	// Set resource attributes
	for k, v := range resourceAttrs {
		rm.Resource().Attributes().PutStr(k, v)
	}

	sm := rm.ScopeMetrics().AppendEmpty()

	for i, name := range tm.MetricNames {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(name)
		metric.SetEmptyGauge()

		if i < len(tm.MetricValues) {
			for _, value := range tm.MetricValues[i] {
				dp := metric.Gauge().DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
				dp.SetDoubleValue(value)
			}
		}
	}

	return md
}

// GenerateComplexTestMetrics creates a more complex set of metrics for testing
func GenerateComplexTestMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()

	// First resource with CPU metrics
	rm1 := md.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr("host.name", "host1")
	rm1.Resource().Attributes().PutStr("service.name", "test-service")

	sm1 := rm1.ScopeMetrics().AppendEmpty()
	sm1.Scope().SetName("otelcol/metricsinferenceprocessor")
	sm1.Scope().SetVersion("0.0.1")

	// CPU utilization gauge metric
	cpuMetric := sm1.Metrics().AppendEmpty()
	cpuMetric.SetName("system.cpu.utilization")
	cpuMetric.SetUnit("1")
	cpuGauge := cpuMetric.SetEmptyGauge()

	// Multiple data points with different CPU core attributes
	cores := []string{"0", "1", "2", "3"}
	values := []float64{0.45, 0.67, 0.23, 0.89}

	for i, core := range cores {
		dp := cpuGauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetDoubleValue(values[i])
		dp.Attributes().PutStr("cpu", core)
		dp.Attributes().PutStr("state", "user")
	}

	// Memory usage sum metric
	memMetric := sm1.Metrics().AppendEmpty()
	memMetric.SetName("system.memory.usage")
	memMetric.SetUnit("By")
	memSum := memMetric.SetEmptySum()
	memSum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	memSum.SetIsMonotonic(false)

	memDp := memSum.DataPoints().AppendEmpty()
	memDp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	memDp.SetIntValue(8589934592) // 8GB in bytes
	memDp.Attributes().PutStr("state", "used")

	// Second resource with different metrics
	rm2 := md.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("host.name", "host2")
	rm2.Resource().Attributes().PutStr("service.name", "test-service")

	sm2 := rm2.ScopeMetrics().AppendEmpty()
	sm2.Scope().SetName("otelcol/metricsinferenceprocessor")

	// Network I/O counter
	netMetric := sm2.Metrics().AppendEmpty()
	netMetric.SetName("system.network.io")
	netMetric.SetUnit("By")
	netSum := netMetric.SetEmptySum()
	netSum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	netSum.SetIsMonotonic(true)

	interfaces := []string{"eth0", "eth1"}
	directions := []string{"receive", "transmit"}

	for _, iface := range interfaces {
		for _, dir := range directions {
			dp := netSum.DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			dp.SetIntValue(int64(1024 * 1024 * 100)) // 100MB
			dp.Attributes().PutStr("interface", iface)
			dp.Attributes().PutStr("direction", dir)
		}
	}

	return md
}

// GetSampleMetricsForInference returns pre-defined metrics useful for inference testing
func GetSampleMetricsForInference() pmetric.Metrics {
	return GenerateTestMetrics(TestMetric{
		MetricNames: []string{
			"system.cpu.utilization",
			"system.memory.utilization",
			"system.disk.utilization",
			"system.network.packets",
		},
		MetricValues: [][]float64{
			{0.75, 0.82, 0.91, 0.68, 0.79}, // CPU values over time
			{0.45, 0.48, 0.52, 0.56, 0.61}, // Memory values
			{0.23, 0.23, 0.24, 0.25, 0.26}, // Disk values
			{1000, 1200, 1150, 1300, 1250}, // Network packet counts
		},
	})
}
