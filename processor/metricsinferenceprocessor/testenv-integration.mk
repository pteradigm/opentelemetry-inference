# KServe Test Environment Integration Targets

.PHONY: testenv-start
testenv-start:
	@echo "🚀 Starting KServe test environment..."
	@echo "   Cleaning up any existing containers..."
	@podman rm -f $$(podman ps -aq --filter name=testenv) 2>/dev/null || true
	cd ./testenv && podman-compose up --build -d
	@echo "⏳ Waiting for MLServer to be ready..."
	@sleep 10
	@curl -s --retry 30 --retry-delay 2 --retry-connrefused http://localhost:9080/v2/health/live > /dev/null
	@echo "✅ MLServer is ready!"

.PHONY: testenv-stop
testenv-stop:
	@echo "🛑 Stopping KServe test environment..."
	cd ./testenv && podman-compose down

.PHONY: testenv-test
testenv-test:
	@echo "🧪 Running KServe test environment validation..."
	cd ./testenv && ./test-setup.sh

.PHONY: integration-test-kserve
integration-test-kserve: testenv-start
	@echo "🔗 Running integration tests with real KServe environment..."
	INTEGRATION_TEST=1 go test -tags=integration -v -run TestMLServerIntegration
	$(MAKE) testenv-stop

.PHONY: integration-test-kserve-keep-running
integration-test-kserve-keep-running:
	@echo "🔗 Running integration tests with existing KServe environment..."
	@echo "Note: Make sure testenv is already running with 'make testenv-start'"
	INTEGRATION_TEST=1 go test -tags=integration -v -run TestMLServerIntegration

.PHONY: testenv-logs
testenv-logs:
	@echo "📋 Showing MLServer logs..."
	cd ./testenv && podman-compose logs mlserver

.PHONY: testenv-status
testenv-status:
	@echo "📊 Checking KServe test environment status..."
	cd ./testenv && podman-compose ps
	@echo ""
	@echo "Health checks:"
	@curl -s http://localhost:9080/v2/health/live > /dev/null 2>&1 && echo "✅ Server is alive" || echo "❌ Server is not alive"
	@curl -s http://localhost:9080/v2/health/ready > /dev/null 2>&1 && echo "✅ Server is ready" || echo "❌ Server is not ready"
