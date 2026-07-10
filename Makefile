DEFAULT_GOAL: help

VERSION ?= v0.0.1

install: ;@ ## Install Setup
	@./scripts/install_dev.sh
.PHONY: install

fmt: ;@ ## Format Code
	@go fmt ./...
.PHONY: fmt

lint: ;@ ## Run Linter
	golangci-lint run ./...
.PHONY: lint

test: fmt lint ;@ ## Run Tests
	go test ./... -v
	@./scripts/coverage.sh
.PHONY: test

build: ;@ ## Run Build
	@go build -ldflags "-X main.Version=$(VERSION)" -o rp .
.PHONY: build

help:
	@echo
	@echo
	@echo reporter
	@echo
	@echo Commands
	@echo
	@grep -hE '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo
	@echo
.PHONY: help
