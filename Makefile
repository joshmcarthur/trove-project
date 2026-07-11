.PHONY: fmt lint test build build-http-ingest build-mqtt-source build-telegram-source build-mcp-query build-capture-classifier docs docs-serve check proto

GOIMPORTS := go run golang.org/x/tools/cmd/goimports

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

build: build-http-ingest build-mqtt-source build-telegram-source build-mcp-query build-capture-classifier
	go build -o bin/trove ./cmd/trove

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

build-capture-classifier:
	go build -o modules/capture-classifier/module ./modules/capture-classifier
	chmod +x modules/capture-classifier/module

docs:
	cd docs && deno task build

docs-serve:
	cd docs && deno task serve

check: fmt lint test
