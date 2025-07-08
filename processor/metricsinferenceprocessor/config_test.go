// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsinferenceprocessor

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/internal/metadata"
)

// Field name constants for validation error messages
const (
	modelNameFieldName = "model_name"
	inputsFieldName    = "inputs"
	outputsFieldName   = "outputs"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				GRPCClientSettings: GRPCClientSettings{
					Endpoint: "localhost:12345",
				},
				Rules: []Rule{
					{
						ModelName: "calculate_percent",
						Inputs:    []string{"metric1", "metric2"},
						Outputs: []OutputSpec{
							{Name: "calculated_percent"},
						},
					},
					{
						ModelName: "scale_1000",
						Inputs:    []string{"metric1"},
						Outputs: []OutputSpec{
							{Name: "scaled_metric"},
						},
					},
				},
				Timeout: 10,
				Naming:  DefaultNamingConfig(),
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "missing_model_name"),
			errorMessage: fmt.Sprintf("missing required field %q for rule at index 0", modelNameFieldName),
		},
		{
			id:           component.NewIDWithName(metadata.Type, "missing_inputs"),
			errorMessage: fmt.Sprintf("missing required field %q for rule at index 0", inputsFieldName),
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expected == nil {
				assert.EqualError(t, cfg.(*Config).Validate(), tt.errorMessage)
				return
			}
			assert.NoError(t, cfg.(*Config).Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
