// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsinferenceprocessor // import "github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

	"github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor/internal/metadata"
)

// This file handles the creation of the processor factory and the processor instances.

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory returns a new factory for the Metrics Inference processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,       // Type of the processor
		createDefaultConfig, // Function to create default configuration
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability), // Specify it's a metrics processor
	)
}

// createDefaultConfig creates the default configuration for the processor.
func createDefaultConfig() component.Config {
	return &Config{
		GRPCClientSettings: GRPCClientSettings{
			// Endpoint is empty by default, requiring user configuration
			Endpoint:    "",
			UseSSL:      false,
			Compression: false,
			Headers:     nil,
		},
		Rules:   nil, // Set to nil instead of empty slice to match test expectations
		Timeout: 10,  // Default timeout of 10 seconds
		Naming:  DefaultNamingConfig(), // Use intelligent naming by default
	}
}

// createMetricsProcessor creates the metrics processor based on the config.
func createMetricsProcessor(
	ctx context.Context, // Keep ctx for potential future use in processor creation/start
	set processor.Settings, // Settings for creating the processor
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	processorCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("configuration parsing error")
	}

	// Create the processor instance
	mp, err := newMetricsProcessor(processorCfg, nextConsumer, set.Logger)
	if err != nil {
		set.Logger.Error("Failed to create metrics inference processor", zap.Error(err))
		return nil, fmt.Errorf("failed to create metrics inference processor: %w", err)
	}

	// Return the processor directly since it already implements processor.Metrics
	return mp, nil
}
