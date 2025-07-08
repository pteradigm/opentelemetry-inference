# Metrics Inference Processor Makefile
include ./testenv-integration.mk

.PHONY: buf
buf:
	@echo "running buf generate"
	buf generate

.PHONY: protodot
protodot:
	@echo "running protodot"
	protodot

.PHONY: test
test:
	go test -v ./...

.PHONY: test-unit
test-unit:
	@echo "Running unit tests..."
	go test -v ./...

.PHONY: test-integration
test-integration:
	@echo "Running integration tests..."
	INTEGRATION_TEST=1 go test -tags=integration -v ./...

.PHONY: test-e2e
test-e2e: testenv-test
	@echo "Running E2E tests..."
	cd testenv && ./test-naming-strategies.sh

.PHONY: test-all
test-all: test-unit test-integration test-e2e
	@echo "All tests completed!"

.PHONY: test-golden-update
test-golden-update:
	@echo "Updating golden files..."
	@echo "Uncomment golden.WriteMetrics line in processor_test.go and run tests"
	@echo "Remember to re-comment the line after updating!"
