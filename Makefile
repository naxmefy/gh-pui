SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c
.DELETE_ON_ERROR:
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules

default: link

pre-check:
	@go version >/dev/null 2>&1 || (echo "ERROR: golang is required."; exit 1)
	@gh version >/dev/null 2>&1 || (echo "ERROR: github cli is required."; exit 1)

build: pre-check
	go build -ldflags "-s -w" -o gh-pui .

link: build
	gh extension remove pui 2>/dev/null || true
	gh extension install .

test:
	go test ./...

lint:
	go vet ./...
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run

run:
	@go run .
