GOLANGCI_LINT_VERSION := v2.12.1
GO_ARCH_LINT_VERSION  := v1.15.0
WIRE_VERSION          := v0.7.0
PROTOC_GEN_GO_VERSION := v1.36.11
PROTOC_GEN_GO_GRPC_VERSION := v1.6.1
BUF_VERSION           := v1.71.0

COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)

.PHONY: help setup fmt lint arch test build generate run clean

help: ## Show available targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | sed -E 's/:.*## /\t/' | awk -F '\t' '{ printf "  %-12s %s\n", $$1, $$2 }'

setrun: ## Run locally against the infrastructure of ../docker-compose.yml; export .env.example first
	go run ./cmd/vote

clean: ## Remove build output
	rm -rf bin
