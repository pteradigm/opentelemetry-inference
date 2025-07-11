# OpenTelemetry Inference Collector - Simple Build System
# Based on metricsgenreceiver pattern for multi-platform support

# Platform detection for OCB download
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

# Default to linux/amd64 if detection fails
OCB_OS := linux
OCB_ARCH := amd64

# macOS detection
ifeq ($(UNAME_S),Darwin)
    OCB_OS := darwin
    ifeq ($(UNAME_M),arm64)
        OCB_ARCH := arm64
    else
        OCB_ARCH := amd64
    endif
endif

# Linux architecture detection
ifeq ($(UNAME_S),Linux)
    OCB_OS := linux
    ifeq ($(UNAME_M),x86_64)
        OCB_ARCH := amd64
    else ifeq ($(UNAME_M),aarch64)
        OCB_ARCH := arm64
    else ifeq ($(UNAME_M),arm64)
        OCB_ARCH := arm64
    endif
endif

OCB_VERSION := 0.127.0
OCB_URL := https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/cmd%2Fbuilder%2Fv$(OCB_VERSION)/ocb_$(OCB_VERSION)_$(OCB_OS)_$(OCB_ARCH)

.PHONY: install-ocb
install-ocb:
	@echo "Installing OCB $(OCB_VERSION) for $(OCB_OS)/$(OCB_ARCH)..."
	curl --proto '=https' --tlsv1.2 -fL -o ocb $(OCB_URL)
	chmod +x ocb

.PHONY: build
build:
	@echo "Building OpenTelemetry Inference Collector..."
	./ocb --config builder-config.yaml

.PHONY: tidy
tidy:
	@echo "Running go mod tidy in build output..."
	cd opentelemetry-inference-collector && go mod tidy

.PHONY: install
install: build
	@echo "Installing collector binary to GOPATH/bin..."
	cp ./opentelemetry-inference-collector/opentelemetry-inference-collector $(GOPATH)/bin/otelcol-inference

.PHONY: run
run: build
	@echo "Running collector with otelcol.yaml configuration..."
	./opentelemetry-inference-collector/opentelemetry-inference-collector --config ./otelcol.yaml

.PHONY: test
test:
	@echo "Running processor unit tests..."
	cd processor/metricsinferenceprocessor && go test -v ./...

.PHONY: test-integration
test-integration:
	@echo "Running KServe integration tests..."
	cd processor/metricsinferenceprocessor && make integration-test-kserve

.PHONY: test-e2e
test-e2e:
	@echo "Running end-to-end tests..."
	cd processor/metricsinferenceprocessor && make test-e2e

.PHONY: test-all
test-all: test test-integration test-e2e
	@echo "All tests completed!"


.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -f ocb
	find ./opentelemetry-inference-collector -mindepth 1 ! -name 'Dockerfile' -delete 2>/dev/null || true

.PHONY: demo-start
demo-start: build
	@echo "Starting OpenTelemetry Metrics Inference Demo Pipeline..."
	cd demo && podman-compose up --build -d
	@echo "🚀 Demo pipeline started!"
	@echo "   • Grafana: http://localhost:3000 (admin/temp1234)"
	@echo "   • VictoriaMetrics: http://localhost:8428"
	@echo "   • MLServer: http://localhost:8082"
	@echo "   • OTel Collector: http://localhost:8888/metrics"

.PHONY: demo-stop
demo-stop:
	@echo "Stopping OpenTelemetry Metrics Inference Demo Pipeline..."
	cd demo && podman-compose down
	@echo "🛑 Demo pipeline stopped!"

.PHONY: demo-logs
demo-logs:
	@echo "Showing demo pipeline logs..."
	cd demo && podman-compose logs -f

.PHONY: demo-status
demo-status:
	@echo "Demo pipeline status:"
	cd demo && podman-compose ps

.PHONY: demo-restart
demo-restart:
	@echo "Restarting OpenTelemetry Metrics Inference Demo Pipeline..."
	cd demo && podman-compose restart
	@echo "🔄 Demo pipeline restarted!"

.PHONY: demo-clean
demo-clean:
	@echo "Cleaning up OpenTelemetry Metrics Inference Demo Pipeline..."
	cd demo && podman-compose down -v --remove-orphans
	@echo "🧹 Demo pipeline cleaned up (containers, volumes, and networks removed)!"

.PHONY: demo-reset
demo-reset: demo-clean demo-start
	@echo "🔄 Demo pipeline reset complete!"

.PHONY: demo-test
demo-test:
	@echo "Testing demo pipeline connectivity..."
	@echo "Checking VictoriaMetrics API: http://localhost:8428/api/v1/query?query=up"
	@curl -s "http://localhost:8428/api/v1/query?query=up" | grep -q "success" && echo "✅ VictoriaMetrics is responding" || echo "❌ VictoriaMetrics connection failed"
	@echo "Checking MLServer health: http://localhost:8082/v2/health/live"  
	@curl -s "http://localhost:8082/v2/health/live" | grep -q "live" && echo "✅ MLServer is healthy" || echo "❌ MLServer health check failed"
	@echo "✅ Demo pipeline test complete!"

.PHONY: help
help:
	@echo "OpenTelemetry Inference Collector Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  install-ocb      - Download and install OpenTelemetry Collector Builder"
	@echo "  build           - Build the collector using OCB"
	@echo "  tidy            - Run go mod tidy in build output"
	@echo "  install         - Build and install collector to GOPATH/bin"
	@echo "  run             - Build and run collector with otelcol.yaml"
	@echo "  test            - Run unit tests"
	@echo "  test-integration - Run KServe integration tests"
	@echo "  test-e2e        - Run end-to-end tests"
	@echo "  test-all        - Run all tests (unit, integration, e2e)"
	@echo "  clean           - Remove build artifacts"
	@echo ""
	@echo "Demo pipeline targets:"
	@echo "  demo-start       - Start complete inference demo pipeline"
	@echo "  demo-stop        - Stop demo pipeline (preserves volumes)"
	@echo "  demo-restart     - Restart demo pipeline (preserves volumes)"
	@echo "  demo-logs        - Show demo pipeline logs"
	@echo "  demo-status      - Show demo pipeline status"
	@echo "  demo-clean       - Remove everything (containers, volumes, networks)"
	@echo "  demo-reset       - Clean + start (complete reset)"
	@echo "  demo-test        - Test demo pipeline connectivity and health"
	@echo "  help            - Show this help message"