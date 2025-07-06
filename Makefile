# -----------------------------------------------------------------------------
# This Makefile is used for building your AVS application.
#
# It contains basic targets for building the application, installing dependencies,
# and building a Docker container.
#
# Modify each target as needed to suit your application's requirements.
# -----------------------------------------------------------------------------

GO = $(shell which go)
OUT = ./bin

build: deps
	@mkdir -p $(OUT) || true
	@echo "Building binaries..."
	go build -o $(OUT)/performer ./cmd/main.go

build-publisher: deps
	@mkdir -p $(OUT) || true
	@echo "Building publisher..."
	go build -o $(OUT)/publisher ./cmd/publisher/

build-demo: deps
	@mkdir -p $(OUT) || true
	@echo "Building demo..."
	go build -o $(OUT)/demo ./cmd/demo/

build-all: build build-publisher build-demo

deps:
	GOPRIVATE=github.com/Layr-Labs/* go mod tidy

build/container:
	./.hourglass/scripts/buildContainer.sh

test: test-go test-forge

test-go::
	go test ./... -v -p 1

test-forge:
	cd .devkit/contracts && forge test

# Demo targets
demo: build-demo
	@echo "Running full Polymarket AVS demo..."
	$(OUT)/demo -mode=full

demo-publish: build-publisher
	@echo "Publishing sample snapshot..."
	$(OUT)/publisher -generate -output=./demo-snapshots -task-file=./demo-snapshots/task_input.json

demo-watch: build-demo
	@echo "Starting snapshot watcher..."
	$(OUT)/demo -mode=watch -snapshot-dir=./demo-snapshots

clean:
	rm -rf $(OUT)
	rm -rf demo-snapshots snapshots

# DevNet targets
devnet-start:
	@echo "🚀 Starting Polymarket AVS DevNet..."
	@./scripts/start-devnet.sh

devnet-stop:
	@echo "🛑 Stopping Polymarket AVS DevNet..."
	@./scripts/stop-devnet.sh

devnet-status:
	@echo "📊 DevNet Status:"
	@docker ps --filter "network=hourglass-network" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" || echo "No DevNet containers running"

devnet-logs:
	@echo "📋 AVS Performer Logs:"
	@docker logs -f polymarket-avs-performer || echo "AVS performer not running"

.PHONY: build build-publisher build-demo build-all deps build/container test test-go test-forge demo demo-publish demo-watch clean devnet-start devnet-stop devnet-status devnet-logs
