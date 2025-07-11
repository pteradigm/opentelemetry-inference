// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsinferenceprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLabelSelector(t *testing.T) {
	tests := []struct {
		name          string
		selector      string
		wantMetric    string
		wantLabels    map[string]string
		wantErr       bool
		errorContains string
	}{
		{
			name:       "metric name only",
			selector:   "system_cpu_utilization",
			wantMetric: "system_cpu_utilization",
			wantLabels: map[string]string{},
		},
		{
			name:       "metric with single label",
			selector:   `system_memory_usage_bytes{state="used"}`,
			wantMetric: "system_memory_usage_bytes",
			wantLabels: map[string]string{"state": "used"},
		},
		{
			name:       "metric with multiple labels",
			selector:   `system_disk_io_bytes{device="sda",direction="read"}`,
			wantMetric: "system_disk_io_bytes",
			wantLabels: map[string]string{"device": "sda", "direction": "read"},
		},
		{
			name:       "metric with spaces",
			selector:   `system_network_io_bytes { direction = "receive" }`,
			wantMetric: "system_network_io_bytes",
			wantLabels: map[string]string{"direction": "receive"},
		},
		{
			name:       "metric with comma in value",
			selector:   `custom_metric{description="value,with,commas"}`,
			wantMetric: "custom_metric",
			wantLabels: map[string]string{"description": "value,with,commas"},
		},
		{
			name:          "empty selector",
			selector:      "",
			wantErr:       true,
			errorContains: "empty selector",
		},
		{
			name:          "missing closing brace",
			selector:      "metric_name{label=\"value\"",
			wantErr:       true,
			errorContains: "missing or misplaced closing brace",
		},
		{
			name:       "missing opening brace",
			selector:   "metric_name label=\"value\"}",
			wantMetric: "metric_name label=\"value\"}",
			wantLabels: map[string]string{},
		},
		{
			name:          "empty metric name",
			selector:      "{label=\"value\"}",
			wantErr:       true,
			errorContains: "empty metric name",
		},
		{
			name:          "invalid label pair - no equals",
			selector:      "metric_name{label_only}",
			wantErr:       true,
			errorContains: "missing '='",
		},
		{
			name:          "invalid label pair - empty key",
			selector:      "metric_name{=\"value\"}",
			wantErr:       true,
			errorContains: "empty label key",
		},
		{
			name:       "empty label value is valid",
			selector:   `metric_name{label=""}`,
			wantMetric: "metric_name",
			wantLabels: map[string]string{"label": ""},
		},
		{
			name:       "multiple labels with various quotes",
			selector:   `metric{a="1",b="2",c="3"}`,
			wantMetric: "metric",
			wantLabels: map[string]string{"a": "1", "b": "2", "c": "3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ls, err := parseLabelSelector(tt.selector)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, ls)
			assert.Equal(t, tt.wantMetric, ls.metricName)
			assert.Equal(t, tt.wantLabels, ls.labels)
		})
	}
}

func TestSplitLabelPairs(t *testing.T) {
	tests := []struct {
		name      string
		labelPart string
		want      []string
	}{
		{
			name:      "single pair",
			labelPart: `label="value"`,
			want:      []string{`label="value"`},
		},
		{
			name:      "multiple pairs",
			labelPart: `a="1",b="2",c="3"`,
			want:      []string{`a="1"`, `b="2"`, `c="3"`},
		},
		{
			name:      "comma in quoted value",
			labelPart: `label="value,with,commas",other="normal"`,
			want:      []string{`label="value,with,commas"`, `other="normal"`},
		},
		{
			name:      "spaces around commas",
			labelPart: `a="1" , b="2" , c="3"`,
			want:      []string{`a="1" `, ` b="2" `, ` c="3"`},
		},
		{
			name:      "empty string",
			labelPart: "",
			want:      []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLabelPairs(tt.labelPart)
			assert.Equal(t, tt.want, got)
		})
	}
}
