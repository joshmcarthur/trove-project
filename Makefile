.PHONY: fmt lint test build build-http-ingest build-mqtt-source docs docs-serve check proto

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

build: build-http-ingest build-mqtt-source
	go build -o bin/trove ./cmd/trove

build-http-ingest:
	go build -o modules/http-ingest/module ./modules/http-ingest
	chmod +x modules/http-ingest/module

build-mqtt-source:
	go build -o modules/mqtt-source/module ./modules/mqtt-source
	chmod +x modules/mqtt-source/module

docs:
	cd docs && deno task build

docs-serve:
	cd docs && deno task serve

check: fmt lint test
