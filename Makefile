DEFAULT_GOAL: help

install: ;@ ## Install Setup
	@./scripts/install_dev.sh
.PHONY: install

fmt: ;@ ## Format Code
	@go fmt ./...
.PHONY: fmt

lint: ;@ ## Run Linter
	@@golangci-lint run ./...
.PHONY: lint

test: lint fmt ;@ ## Run Tests
	go test ./...
.PHONY: test

build: ;@ ## Run Build
	@go build -o reporter main.go
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
