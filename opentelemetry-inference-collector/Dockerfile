# Multi-stage build for OpenTelemetry Inference Collector
FROM ubuntu:22.04 as runtime

# Install ca-certificates and other dependencies
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

# Create non-root user for security
RUN groupadd -r otel && useradd -r -g otel otel

# Copy the collector binary built by OCB
COPY opentelemetry-inference-collector /otelcol

# Make sure the binary is executable and owned by otel user
RUN chmod +x /otelcol && chown otel:otel /otelcol

USER otel

# Set the binary as entrypoint
ENTRYPOINT ["/otelcol"]
CMD ["--config=/etc/otelcol/otel-collector-config.yaml"]

# Expose standard OpenTelemetry ports
EXPOSE 4317 4318 8888