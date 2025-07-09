# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache make git

WORKDIR /app

# Copy go mod files
COPY go.mod ./
COPY go.sum* ./
COPY processor/metricsinferenceprocessor/go.mod processor/metricsinferenceprocessor/go.sum ./processor/metricsinferenceprocessor/

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Install OCB and build
RUN make install-ocb && make build

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates

# Create non-root user
RUN adduser -D -u 10001 otel

WORKDIR /

# Copy binary from builder
COPY --from=builder /app/opentelemetry-inference-collector/opentelemetry-inference-collector /opentelemetry-inference

# Copy default configuration
COPY --from=builder /app/otelcol.yaml /etc/otel/config.yaml

# Change ownership
RUN chown -R otel:otel /opentelemetry-inference /etc/otel

USER otel

EXPOSE 4317 4318 8888

ENTRYPOINT ["/opentelemetry-inference"]
CMD ["--config", "/etc/otel/config.yaml"]