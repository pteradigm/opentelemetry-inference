// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsinferenceprocessor // import "github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func getNameToMetricMap(rm pmetric.ResourceMetrics) map[string]pmetric.Metric {
	ilms := rm.ScopeMetrics()
	metricMap := make(map[string]pmetric.Metric)

	for i := 0; i < ilms.Len(); i++ {
		ilm := ilms.At(i)
		metricSlice := ilm.Metrics()
		for j := 0; j < metricSlice.Len(); j++ {
			metric := metricSlice.At(j)
			metricMap[metric.Name()] = metric
		}
	}
	return metricMap
}

// getMetricValue returns the value of the first data point from the given metric.
func getMetricValue(metric pmetric.Metric) float64 {
	var val float64
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps := metric.Gauge().DataPoints()
		if dps.Len() > 0 {
			// Handle int or double gauge value
			dp := dps.At(0)
			switch dp.ValueType() {
			case pmetric.NumberDataPointValueTypeInt:
				val = float64(dp.IntValue())
			case pmetric.NumberDataPointValueTypeDouble:
				val = dp.DoubleValue()
			}
		}
	case pmetric.MetricTypeSum:
		dps := metric.Sum().DataPoints()
		if dps.Len() > 0 {
			// Handle int or double sum value
			dp := dps.At(0)
			switch dp.ValueType() {
			case pmetric.NumberDataPointValueTypeInt:
				val = float64(dp.IntValue())
			case pmetric.NumberDataPointValueTypeDouble:
				val = dp.DoubleValue()
			}
		}
	// TODO: Handle other metric types like Histogram, Summary, ExponentialHistogram if needed.
	default:
		// Log or handle unsupported metric type? Returning 0 for now.
	}
	return val
}

// Calculates a new metric based on the calculation-type rule specified. New data points will be generated for each
// calculation of the input metrics where overlapping attributes have matching values.
func generateMetricFromMatchingAttributes(metric1 pmetric.Metric, metric2 pmetric.Metric, operation string, logger *zap.Logger) pmetric.Metric {
	var metric1DataPoints pmetric.NumberDataPointSlice
	var toDataPoints pmetric.NumberDataPointSlice
	to := pmetric.NewMetric()

	// Setup to metric and get metric1 data points
	switch metricType := metric1.Type(); metricType {
	case pmetric.MetricTypeGauge:
		to.SetEmptyGauge()
		metric1DataPoints = metric1.Gauge().DataPoints()
		toDataPoints = to.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		to.SetEmptySum()
		metric1DataPoints = metric1.Sum().DataPoints()
		toDataPoints = to.Sum().DataPoints()
	default:
		logger.Debug(fmt.Sprintf("Calculations are only supported on gauge or sum metric types. Given metric '%s' is of type `%s`", metric1.Name(), metricType.String()))
		return pmetric.NewMetric()
	}

	// Get metric2 data points
	var metric2DataPoints pmetric.NumberDataPointSlice
	switch metricType := metric2.Type(); metricType {
	case pmetric.MetricTypeGauge:
		metric2DataPoints = metric2.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		metric2DataPoints = metric2.Sum().DataPoints()
	default:
		logger.Debug(fmt.Sprintf("Calculations are only supported on gauge or sum metric types. Given metric '%s' is of type `%s`", metric2.Name(), metricType.String()))
		return pmetric.NewMetric()
	}

	for i := 0; i < metric1DataPoints.Len(); i++ {
		metric1DP := metric1DataPoints.At(i)

		for j := 0; j < metric2DataPoints.Len(); j++ {
			metric2DP := metric2DataPoints.At(j)
			if dataPointAttributesMatch(metric1DP, metric2DP) {
				val, err := calculateValue(dataPointValue(metric1DP), dataPointValue(metric2DP), operation, to.Name())

				if err != nil {
					logger.Debug(err.Error())
				} else {
					newDP := toDataPoints.AppendEmpty()
					metric1DP.CopyTo(newDP)
					newDP.SetDoubleValue(val)

					metric2DP.Attributes().Range(func(k string, v pcommon.Value) bool {
						v.CopyTo(newDP.Attributes().PutEmpty(k))
						// Always return true to ensure iteration over all attributes
						return true
					})
				}
			}
		}
	}

	return to
}

func dataPointValue(dp pmetric.NumberDataPoint) float64 {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		return dp.DoubleValue()
	case pmetric.NumberDataPointValueTypeInt:
		return float64(dp.IntValue())
	default:
		return 0
	}
}

func dataPointAttributesMatch(dp1, dp2 pmetric.NumberDataPoint) bool {
	attributesMatch := true
	dp1.Attributes().Range(func(key string, dp1Val pcommon.Value) bool {
		dp1Val.Type()
		if dp2Val, keyExists := dp2.Attributes().Get(key); keyExists && dp1Val.AsRaw() != dp2Val.AsRaw() {
			attributesMatch = false
			return false
		}
		return true
	})

	return attributesMatch
}

func generateMetricFromOperand(from pmetric.Metric, operand2 float64, operation string, logger *zap.Logger) pmetric.Metric {
	var dataPoints pmetric.NumberDataPointSlice
	to := pmetric.NewMetric()

	switch metricType := from.Type(); metricType {
	case pmetric.MetricTypeGauge:
		to.SetEmptyGauge()
		dataPoints = from.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		to.SetEmptySum()
		dataPoints = from.Sum().DataPoints()
	default:
		logger.Debug(fmt.Sprintf("Calculations are only supported on gauge or sum metric types. Given metric '%s' is of type `%s`", from.Name(), metricType.String()))
		return pmetric.NewMetric()
	}

	for i := 0; i < dataPoints.Len(); i++ {
		fromDataPoint := dataPoints.At(i)
		var operand1 float64
		switch fromDataPoint.ValueType() {
		case pmetric.NumberDataPointValueTypeDouble:
			operand1 = fromDataPoint.DoubleValue()
		case pmetric.NumberDataPointValueTypeInt:
			operand1 = float64(fromDataPoint.IntValue())
		}
		value, err := calculateValue(operand1, operand2, operation, to.Name())

		// Only add a new data point if it was a valid operation
		if err != nil {
			logger.Debug(err.Error())
		} else {
			var newDoubleDataPoint pmetric.NumberDataPoint
			switch to.Type() {
			case pmetric.MetricTypeGauge:
				newDoubleDataPoint = to.Gauge().DataPoints().AppendEmpty()
			case pmetric.MetricTypeSum:
				newDoubleDataPoint = to.Sum().DataPoints().AppendEmpty()
			}

			fromDataPoint.CopyTo(newDoubleDataPoint)
			newDoubleDataPoint.SetDoubleValue(value)
		}
	}

	return to
}

// Append the new metric to the scope metrics. This will only append the new metric if it
// has data points.
func appendNewMetric(ilm pmetric.ScopeMetrics, newMetric pmetric.Metric, name, unit string) {
	dataPointCount := 0
	switch newMetric.Type() {
	case pmetric.MetricTypeSum:
		dataPointCount = newMetric.Sum().DataPoints().Len()
	case pmetric.MetricTypeGauge:
		dataPointCount = newMetric.Gauge().DataPoints().Len()
	}

	// Only create a new metric if valid data points were calculated successfully
	if dataPointCount > 0 {
		metric := ilm.Metrics().AppendEmpty()
		newMetric.MoveTo(metric)

		metric.SetUnit(unit)
		metric.SetName(name)
	}
}

// Operation types for metric calculations
const (
	operationAdd      = "add"
	operationSubtract = "subtract"
	operationMultiply = "multiply"
	operationDivide   = "divide"
	operationPercent  = "percent"
)

func calculateValue(operand1 float64, operand2 float64, operation string, metricName string) (float64, error) {
	switch operation {
	case operationAdd:
		return operand1 + operand2, nil
	case operationSubtract:
		return operand1 - operand2, nil
	case operationMultiply:
		return operand1 * operand2, nil
	case operationDivide:
		if operand2 == 0 {
			return 0, fmt.Errorf("divide by zero in metric: %s", metricName)
		}
		return operand1 / operand2, nil
	case operationPercent:
		if operand2 == 0 {
			return 0, fmt.Errorf("divide by zero in metric: %s", metricName)
		}
		return (operand1 / operand2) * 100, nil
	default:
		return 0, fmt.Errorf("unknown operation %s in metric: %s", operation, metricName)
	}
}
