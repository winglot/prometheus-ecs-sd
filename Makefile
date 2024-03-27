SHELL := /bin/sh
IMAGE=winglot/prometheus-ecs-sd
REPO=$(IMAGE)
VERSION=v1.0.0
.DEFAULT_GOAL := help

# Colors
ifeq ($(TERM),xterm-256color)
TXT_RED := $(shell tput setaf 1)
TXT_GREEN := $(shell tput setaf 2)
TXT_YELLOW := $(shell tput setaf 3)
TXT_RESET := $(shell tput sgr0)
endif

.PHONY: build
build: ## Build the binary
	mkdir -p bin
	CGO_ENABLED=0 go build -o bin/prometheus-ecs-sd cmd/sd/main.go

.PHONY: image
image: ## Build the latest image (for development)
	docker build -t $(IMAGE):latest .

.PHONY: image-release
image-release: ## Build the versioned image
	docker buildx build --platform linux/amd64,linux/arm64 -t $(IMAGE):$(VERSION) .

.PHONY: push-release
push-release: ## Push the versioned image to the registry
	docker buildx build --push --platform linux/amd64,linux/arm64 -t $(REPO):$(VERSION) .

.PHONY: test
test: ## Run Go tests
	go test ./...

.PHONY: clean
clean: ## Clean up the project
	rm -rf ./bin

.PHONY: help
help: ## Display help prompt
	@echo
	@echo Usage: make \<command\>
	@echo
	@echo Available commands:
	@grep -E '^[a-zA-Z_-]+%?:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "$(TXT_YELLOW)%-25s $(TXT_RESET) %s\n", $$1, $$2}'
	@echo