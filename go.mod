module github.com/rbellamy/opentelemetry-inference

// This go.mod provides local module replacements for the OpenTelemetry Collector Builder.
// The actual collector binary is built using OCB with builder-config.yaml.

go 1.23.4

// Replace references to modules that are in this repository with their relative paths
// so that we always build with current (latest) version of the source code.
replace github.com/rbellamy/opentelemetry-inference/processor/metricsinferenceprocessor => ./processor/metricsinferenceprocessor