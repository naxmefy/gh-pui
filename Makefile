SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c
.DELETE_ON_ERROR:
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules
TEMP_GH_EXT_PUI_INSTALLED := $(gh ext list | grep "gh pui")

default: link

pre-check:
	@go version >/dev/null 2>&1 || (echo "ERROR: golang is required."; exit 1)
	@gh version >/dev/null 2>&1 || (echo "ERROR: github cli is required."; exit 1)

build: pre-check
	go build -ldflags "-s -w" -o gh-pui main.go

link: build
	@if [ -z "$(TEMP_GH_EXT_PUI_INSTALLED)" ]; then \
		gh extension remove pui || (exit 0); \
	else \
		echo "Empty"; \
	fi
	gh extension install .

run:
	@go run main.go