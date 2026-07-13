.PHONY: fmt lint test build build-http-gateway build-http-ingest build-mqtt-source build-telegram-source build-mcp-query build-type-catalog docs docs-serve check proto

GOIMPORTS := go run golang.org/x/tools/cmd/goimports

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

proto:
	protoc -I api/proto \
		--go_out=internal/modules/rpc --go_opt=paths=source_relative \
		--go-grpc_out=internal/modules/rpc --go-grpc_opt=paths=source_relative \
		api/proto/trove/v1/module.proto

fmt:
	go fmt ./...
	@$(GOIMPORTS) -w .

lint:
	golangci-lint run

test:
	go test -race -cover ./...

build: build-http-gateway build-http-ingest build-mqtt-source build-telegram-source build-mcp-query build-type-catalog
	go build -ldflags "$(LDFLAGS)" -o bin/trove ./cmd/trove

build-http-gateway:
	go build -o modules/http-gateway/module ./modules/http-gateway
	chmod +x modules/http-gateway/module

build-http-ingest:
	go build -o modules/http-ingest/module ./modules/http-ingest/cmd
	chmod +x modules/http-ingest/module

build-mqtt-source:
	go build -o modules/mqtt-source/module ./modules/mqtt-source
	chmod +x modules/mqtt-source/module

build-telegram-source:
	go build -o modules/telegram-source/module ./modules/telegram-source
	chmod +x modules/telegram-source/module

build-mcp-query:
	go build -o modules/mcp-query/module ./modules/mcp-query/cmd
	chmod +x modules/mcp-query/module

build-type-catalog:
	go build -o modules/type-catalog/module ./modules/type-catalog/cmd
	chmod +x modules/type-catalog/module

docs:
	cd docs && deno task build

docs-serve:
	cd docs && deno task serve

check: fmt lint test
