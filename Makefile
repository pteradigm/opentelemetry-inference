# OpenTelemetry Inference Collector - Simple Build System
# Based on metricsgenreceiver pattern for multi-platform support

.DEFAULT_GOAL := help

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

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date +"%Y-%m-%d %H:%M:%S" || echo "unknown")

# Build flags for version injection
# Note: main.Version is set in the generated main.go file
LDFLAGS := -X 'main.Version=$(VERSION)'

.PHONY: build
build: ocb
	@echo "Building OpenTelemetry Inference Collector..."
	@echo "Version: $(VERSION)"
	VERSION=$(VERSION) ./ocb --config builder-config.yaml

# OCB dependency check
ocb:
	@if [ ! -f ./ocb ]; then \
		echo "OCB not found. Installing..."; \
		$(MAKE) install-ocb; \
	fi

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

.PHONY: install-tools
install-tools:
	@echo "Installing development tools..."
	go install golang.org/x/tools/cmd/goimports@latest
	go install mvdan.cc/gofumpt@latest
	@echo "Tools installed!"

.PHONY: setup
setup: install-ocb install-tools
	@echo "Setting up development environment..."
	./scripts/setup-hooks.sh
	@echo "✓ Development environment ready!"

.PHONY: fmt
fmt: install-tools
	@echo "Running Go formatters..."
	cd processor/metricsinferenceprocessor && go fmt ./...
	cd processor/metricsinferenceprocessor && goimports -w .
	cd processor/metricsinferenceprocessor && gofumpt -w .
	@echo "Formatting complete!"

.PHONY: fmt-check
fmt-check:
	@echo "Checking Go formatting..."
	@command -v goimports >/dev/null 2>&1 || (echo "goimports not found. Run 'make install-tools' first." && exit 1)
	@cd processor/metricsinferenceprocessor && \
	if [ -n "$$(go fmt ./...)" ]; then \
		echo "Error: Files need formatting. Run 'make fmt' to fix."; \
		exit 1; \
	fi
	@cd processor/metricsinferenceprocessor && \
	if [ -n "$$(goimports -l .)" ]; then \
		echo "Error: Import ordering needs fixing. Run 'make fmt' to fix."; \
		exit 1; \
	fi
	@echo "✓ All formatting checks passed!"

.PHONY: lint
lint: fmt-check
	@echo "Running linters..."
	cd processor/metricsinferenceprocessor && go vet ./...
	cd processor/metricsinferenceprocessor && go mod tidy && git diff --quiet go.mod go.sum || \
		(echo "Error: go.mod not tidy. Run 'go mod tidy' and commit changes." && exit 1)
	@echo "✓ All lint checks passed!"


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
	@echo "Setup and Installation:"
	@echo "  make setup           - Set up complete development environment"
	@echo "  make install-ocb     - Install OpenTelemetry Collector Builder"
	@echo "  make install-tools   - Install Go development tools"
	@echo ""
	@echo "Building and Running:"
	@echo "  make build          - Build the collector binary"
	@echo "  make run            - Run the collector with sample config"
	@echo "  make install        - Install collector to GOPATH/bin"
	@echo ""
	@echo "Code Quality:"
	@echo "  make fmt            - Auto-format Go code"
	@echo "  make fmt-check      - Check code formatting"
	@echo "  make lint           - Run all linting checks"
	@echo ""
	@echo "Testing:"
	@echo "  make test           - Run unit tests"
	@echo "  make test-integration - Run integration tests"
	@echo "  make test-e2e       - Run end-to-end tests"
	@echo "  make test-all       - Run all tests"
	@echo ""
	@echo "Demo:"
	@echo "  make demo-start     - Start demo environment"
	@echo "  make demo-stop      - Stop demo environment"
	@echo "  make demo-clean     - Clean demo environment"
	@echo "  make demo-logs      - View demo logs"
	@echo "  make demo-test      - Test demo connectivity"
	@echo ""
	@echo "Maintenance:"
	@echo "  make clean          - Clean build artifacts"
