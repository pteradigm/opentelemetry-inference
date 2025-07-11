// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsinferenceprocessor

import (
	"fmt"
	"strings"
)

// labelSelector represents a parsed label selector for metric filtering
type labelSelector struct {
	metricName string
	labels     map[string]string
}

// parseLabelSelector parses a Prometheus-style metric selector
// Examples:
//   - "metric_name" -> just the metric name, no label filtering
//   - "metric_name{label1=\"value1\"}" -> metric with single label filter
//   - "metric_name{label1=\"value1\",label2=\"value2\"}" -> metric with multiple label filters
func parseLabelSelector(selector string) (*labelSelector, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, fmt.Errorf("empty selector")
	}

	// Check if selector contains labels
	openBrace := strings.Index(selector, "{")
	if openBrace == -1 {
		// No labels, just metric name
		return &labelSelector{
			metricName: selector,
			labels:     make(map[string]string),
		}, nil
	}

	// Extract metric name
	metricName := strings.TrimSpace(selector[:openBrace])
	if metricName == "" {
		return nil, fmt.Errorf("empty metric name")
	}

	// Check for closing brace
	closeBrace := strings.LastIndex(selector, "}")
	if closeBrace == -1 || closeBrace <= openBrace {
		return nil, fmt.Errorf("invalid selector syntax: missing or misplaced closing brace")
	}

	// Extract label part
	labelPart := selector[openBrace+1 : closeBrace]
	labels, err := parseLabelPairs(labelPart)
	if err != nil {
		return nil, fmt.Errorf("failed to parse labels: %w", err)
	}

	return &labelSelector{
		metricName: metricName,
		labels:     labels,
	}, nil
}

// parseLabelPairs parses comma-separated label pairs
func parseLabelPairs(labelPart string) (map[string]string, error) {
	labels := make(map[string]string)
	labelPart = strings.TrimSpace(labelPart)

	if labelPart == "" {
		return labels, nil
	}

	// Split by comma, but need to handle commas within quotes
	pairs := splitLabelPairs(labelPart)

	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// Find the equals sign
		eqIndex := strings.Index(pair, "=")
		if eqIndex == -1 {
			return nil, fmt.Errorf("invalid label pair: %s (missing '=')", pair)
		}

		key := strings.TrimSpace(pair[:eqIndex])
		value := strings.TrimSpace(pair[eqIndex+1:])

		if key == "" {
			return nil, fmt.Errorf("empty label key in pair: %s", pair)
		}

		// Remove quotes from value
		value = strings.Trim(value, "\"")

		labels[key] = value
	}

	return labels, nil
}

// splitLabelPairs splits label pairs by comma, respecting quoted values
func splitLabelPairs(labelPart string) []string {
	if labelPart == "" {
		return []string{}
	}

	var pairs []string
	var current strings.Builder
	inQuotes := false

	for i, ch := range labelPart {
		switch ch {
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(ch)
		case ',':
			if !inQuotes {
				pairs = append(pairs, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}

		// Handle last pair
		if i == len(labelPart)-1 {
			if current.Len() > 0 {
				pairs = append(pairs, current.String())
			}
		}
	}

	return pairs
}
