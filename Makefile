.PHONY: lint test build all

lint:
	golangci-lint run ./...

test:
	gotestsum --format testname ./...

build:
	go build -o wc3ts ./cmd/wc3ts

all: lint test build
