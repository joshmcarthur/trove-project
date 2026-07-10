.PHONY: fmt lint test build docs docs-serve check proto

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

build:
	go build -o bin/trove ./cmd/trove

docs:
	cd docs && deno task build

docs-serve:
	cd docs && deno task serve

check: fmt lint test
