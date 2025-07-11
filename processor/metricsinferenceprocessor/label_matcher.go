// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsinferenceprocessor

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// matchesSelector checks if a metric matches the given label selector
func matchesSelector(metric pmetric.Metric, selector *labelSelector) bool {
	if selector == nil {
		return false
	}

	// First check metric name
	if metric.Name() != selector.metricName {
		return false
	}

	// If no label filters, metric name match is sufficient
	if len(selector.labels) == 0 {
		return true
	}

	// Check if any data point matches the label filters
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		return hasMatchingGaugeDataPoint(metric.Gauge(), selector.labels)
	case pmetric.MetricTypeSum:
		return hasMatchingSumDataPoint(metric.Sum(), selector.labels)
	case pmetric.MetricTypeHistogram:
		return hasMatchingHistogramDataPoint(metric.Histogram(), selector.labels)
	case pmetric.MetricTypeSummary:
		return hasMatchingSummaryDataPoint(metric.Summary(), selector.labels)
	default:
		return false
	}
}

// hasMatchingGaugeDataPoint checks if any gauge data point matches the label filters
func hasMatchingGaugeDataPoint(gauge pmetric.Gauge, labelFilters map[string]string) bool {
	dps := gauge.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		if dataPointMatchesLabels(dps.At(i).Attributes(), labelFilters) {
			return true
		}
	}
	return false
}

// hasMatchingSumDataPoint checks if any sum data point matches the label filters
func hasMatchingSumDataPoint(sum pmetric.Sum, labelFilters map[string]string) bool {
	dps := sum.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		if dataPointMatchesLabels(dps.At(i).Attributes(), labelFilters) {
			return true
		}
	}
	return false
}

// hasMatchingHistogramDataPoint checks if any histogram data point matches the label filters
func hasMatchingHistogramDataPoint(histogram pmetric.Histogram, labelFilters map[string]string) bool {
	dps := histogram.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		if dataPointMatchesLabels(dps.At(i).Attributes(), labelFilters) {
			return true
		}
	}
	return false
}

// hasMatchingSummaryDataPoint checks if any summary data point matches the label filters
func hasMatchingSummaryDataPoint(summary pmetric.Summary, labelFilters map[string]string) bool {
	dps := summary.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		if dataPointMatchesLabels(dps.At(i).Attributes(), labelFilters) {
			return true
		}
	}
	return false
}

// dataPointMatchesLabels checks if data point attributes match all label filters
func dataPointMatchesLabels(attributes pcommon.Map, labelFilters map[string]string) bool {
	for key, expectedValue := range labelFilters {
		actualValue, exists := attributes.Get(key)
		if !exists {
			return false
		}
		if actualValue.AsString() != expectedValue {
			return false
		}
	}
	return true
}

// filterMetricByLabels creates a new metric containing only data points that match the label filters
func filterMetricByLabels(metric pmetric.Metric, labelFilters map[string]string) pmetric.Metric {
	filtered := pmetric.NewMetric()
	metric.CopyTo(filtered)

	// If no label filters, return the whole metric
	if len(labelFilters) == 0 {
		return filtered
	}

	// Filter data points based on metric type
	switch filtered.Type() {
	case pmetric.MetricTypeGauge:
		filterGaugeDataPoints(filtered.Gauge(), labelFilters)
	case pmetric.MetricTypeSum:
		filterSumDataPoints(filtered.Sum(), labelFilters)
	case pmetric.MetricTypeHistogram:
		filterHistogramDataPoints(filtered.Histogram(), labelFilters)
	case pmetric.MetricTypeSummary:
		filterSummaryDataPoints(filtered.Summary(), labelFilters)
	}

	return filtered
}

// filterGaugeDataPoints removes data points that don't match the label filters
func filterGaugeDataPoints(gauge pmetric.Gauge, labelFilters map[string]string) {
	dps := gauge.DataPoints()
	dps.RemoveIf(func(dp pmetric.NumberDataPoint) bool {
		return !dataPointMatchesLabels(dp.Attributes(), labelFilters)
	})
}

// filterSumDataPoints removes data points that don't match the label filters
func filterSumDataPoints(sum pmetric.Sum, labelFilters map[string]string) {
	dps := sum.DataPoints()
	dps.RemoveIf(func(dp pmetric.NumberDataPoint) bool {
		return !dataPointMatchesLabels(dp.Attributes(), labelFilters)
	})
}

// filterHistogramDataPoints removes data points that don't match the label filters
func filterHistogramDataPoints(histogram pmetric.Histogram, labelFilters map[string]string) {
	dps := histogram.DataPoints()
	dps.RemoveIf(func(dp pmetric.HistogramDataPoint) bool {
		return !dataPointMatchesLabels(dp.Attributes(), labelFilters)
	})
}

// filterSummaryDataPoints removes data points that don't match the label filters
func filterSummaryDataPoints(summary pmetric.Summary, labelFilters map[string]string) {
	dps := summary.DataPoints()
	dps.RemoveIf(func(dp pmetric.SummaryDataPoint) bool {
		return !dataPointMatchesLabels(dp.Attributes(), labelFilters)
	})
}
