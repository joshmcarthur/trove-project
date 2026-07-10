.PHONY: fmt lint test build docs docs-serve check

GOIMPORTS := go run golang.org/x/tools/cmd/goimports

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
