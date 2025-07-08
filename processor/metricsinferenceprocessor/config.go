// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsinferenceprocessor // import "github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config defines the configuration for the metrics inference processor.
type Config struct {
	// GRPCClientSettings defines the gRPC connection settings for the inference service.
	GRPCClientSettings GRPCClientSettings `mapstructure:"grpc"`

	// Rules define how to process metrics and which inference model to use.
	Rules []Rule `mapstructure:"rules"`

	// Timeout for inference requests in seconds. Default is 10 seconds.
	Timeout int `mapstructure:"timeout"`

	// Naming configures the naming strategy for output metrics
	Naming NamingConfig `mapstructure:"naming"`
}

// GRPCClientSettings defines the configuration for the gRPC client.
type GRPCClientSettings struct {
	// Endpoint for the inference service (e.g., "localhost:50051")
	Endpoint string `mapstructure:"endpoint"`

	// UseSSL indicates whether to use SSL/TLS for the connection
	UseSSL bool `mapstructure:"use_ssl"`

	// Compression indicates whether to use gRPC compression
	Compression bool `mapstructure:"compression"`

	// MaxReceiveMessageSize sets the maximum message size in bytes the client can receive
	MaxReceiveMessageSize int `mapstructure:"max_receive_message_size"`

	// Headers to be sent with gRPC requests
	Headers map[string]string `mapstructure:"headers"`

	// KeepAlive settings for the gRPC client
	KeepAlive *KeepAliveClientConfig `mapstructure:"keepalive"`
}

// KeepAliveClientConfig defines the configuration for gRPC client keep-alive.
type KeepAliveClientConfig struct {
	// Time is the duration after which if there's no activity a keepalive ping is sent
	Time time.Duration `mapstructure:"time"`

	// Timeout is the duration the client waits for a response to the keepalive ping
	Timeout time.Duration `mapstructure:"timeout"`

	// PermitWithoutStream if true allows keepalive pings to be sent even when there are no active streams
	PermitWithoutStream bool `mapstructure:"permit_without_stream"`
}

var _ component.Config = (*Config)(nil)

// Validate checks whether the input configuration has all of the required fields for the processor.
// An error is returned if there are any invalid inputs.
func (cfg *Config) Validate() error {
	if cfg.GRPCClientSettings.Endpoint == "" {
		return fmt.Errorf("gRPC endpoint must be specified")
	}

	for i, rule := range cfg.Rules {
		if rule.ModelName == "" {
			return fmt.Errorf("missing required field \"model_name\" for rule at index %d", i)
		}
		if len(rule.Inputs) == 0 {
			return fmt.Errorf("missing required field \"inputs\" for rule at index %d", i)
		}
		// Outputs are now optional - they can be discovered from model metadata
		// We'll validate at runtime if neither configured nor discovered outputs exist
		
		// Validate output pattern if specified
		if rule.OutputPattern != "" {
			if err := validateOutputPattern(rule.OutputPattern); err != nil {
				return fmt.Errorf("invalid output_pattern in rule %d: %w", i, err)
			}
		}
	}

	return nil
}

// OutputSpec defines the specification for a single output from the inference model.
type OutputSpec struct {
	// Name specifies the name to use for the output metric.
	// If not provided, the output tensor name from the inference response will be used.
	Name string `mapstructure:"name"`

	// DataType specifies the expected data type of the model output.
	// Valid values: "float", "int", "bool", "string"
	// If not provided, the data type will be inferred from the inference response.
	DataType string `mapstructure:"data_type"`

	// Description specifies a description for the output metric.
	Description string `mapstructure:"description"`

	// Unit specifies the unit for the output metric.
	Unit string `mapstructure:"unit"`

	// OutputIndex specifies which output tensor to use (0-based index).
	// If not specified, defaults to 0 for single output or matches by name.
	OutputIndex *int `mapstructure:"output_index"`
}

// Rule defines a processing rule for metrics inference.
type Rule struct {
	// ModelName specifies the model to use for inference.
	ModelName string `mapstructure:"model_name"`

	// ModelVersion specifies the version of the model to use. If empty, the server will choose.
	ModelVersion string `mapstructure:"model_version"`

	// Inputs specifies the list of metric names required as input for the model.
	Inputs []string `mapstructure:"inputs"`

	// Outputs specifies the list of outputs to create from the inference results.
	// Each output represents a metric that will be created from the inference response.
	Outputs []OutputSpec `mapstructure:"outputs"`

	// OutputPattern specifies a template for generating output metric names.
	// If not specified, uses default smart stem extraction.
	// Template variables:
	//   {input} or {input[0]} - First input metric name
	//   {input[N]} - Nth input metric name (0-based)
	//   {output} - Output tensor name from model
	//   {model} - Model name
	//   {version} - Model version (empty string if not specified)
	// Example: "ml.{model}.{output}" → "ml.cpu_predictor.prediction"
	OutputPattern string `mapstructure:"output_pattern"`

	// Parameters contains additional parameters to pass to the inference service.
	Parameters map[string]interface{} `mapstructure:"parameters"`
}
