# AKS Health Monitor Makefile

# Project variables
PROJECT_NAME := aks-health-monitor
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Go variables
GO_VERSION := 1.21
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
CGO_ENABLED ?= 0

# Docker variables
DOCKER_REGISTRY ?= 
DOCKER_IMAGE := $(DOCKER_REGISTRY)$(PROJECT_NAME)
DOCKER_TAG ?= $(VERSION)

# Build variables
BINARY_NAME := $(PROJECT_NAME)
BUILD_DIR := ./bin
CMD_DIR := ./cmd/controller
MAIN_FILE := $(CMD_DIR)/main.go

# Kubernetes variables
NAMESPACE ?= kube-system
KUBECONFIG ?= ~/.kube/config

# Test variables
COVERAGE_FILE := coverage.out
COVERAGE_HTML := coverage.html

# Linting
GOLANGCI_LINT_VERSION := v1.54.2

# LDFLAGS for build information
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE) -w -s"

.PHONY: help
help: ## Show this help message
	@echo "AKS Health Monitor - Makefile commands:"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

## Development targets

.PHONY: deps
deps: ## Download Go module dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

.PHONY: deps-update
deps-update: ## Update Go module dependencies
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

.PHONY: build
build: ## Build the binary for current platform
	@echo "Building $(BINARY_NAME) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_FILE)

.PHONY: build-linux
build-linux: ## Build the binary for Linux
	@echo "Building $(BINARY_NAME) for linux/amd64..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_FILE)

.PHONY: build-all
build-all: ## Build binaries for all platforms
	@echo "Building binaries for all platforms..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_FILE)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_FILE)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_FILE)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_FILE)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_FILE)

.PHONY: run
run: build ## Build and run the application locally
	@echo "Running $(BINARY_NAME)..."
	$(BUILD_DIR)/$(BINARY_NAME) -config=config/config.yaml -kubeconfig=$(KUBECONFIG)

.PHONY: run-dev
run-dev: ## Run the application in development mode with verbose logging
	@echo "Running $(BINARY_NAME) in development mode..."
	go run $(MAIN_FILE) -config=config/config.yaml -kubeconfig=$(KUBECONFIG) -v=2

## Testing targets

.PHONY: test
test: ## Run unit tests
	@echo "Running tests..."
	go test -v ./...

.PHONY: test-race
test-race: ## Run tests with race condition detection
	@echo "Running tests with race detection..."
	go test -race -v ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	go test -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report generated: $(COVERAGE_HTML)"

.PHONY: test-integration
test-integration: ## Run integration tests (requires cluster access)
	@echo "Running integration tests..."
	go test -tags=integration -v ./test/integration/...

.PHONY: benchmark
benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

## Code quality targets

.PHONY: fmt
fmt: ## Format Go code
	@echo "Formatting code..."
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint
	@echo "Running golangci-lint..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin $(GOLANGCI_LINT_VERSION); \
	fi
	golangci-lint run

.PHONY: lint-fix
lint-fix: ## Run golangci-lint with auto-fix
	@echo "Running golangci-lint with auto-fix..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin $(GOLANGCI_LINT_VERSION); \
	fi
	golangci-lint run --fix

.PHONY: check
check: fmt vet lint test ## Run all checks (format, vet, lint, test)

## Docker targets

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "Building Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest

.PHONY: docker-push
docker-push: docker-build ## Build and push Docker image
	@echo "Pushing Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest

.PHONY: docker-run
docker-run: docker-build ## Build and run Docker container locally
	@echo "Running Docker container..."
	docker run --rm -it \
		-v $(HOME)/.kube:/root/.kube:ro \
		-v $(PWD)/config:/etc/config:ro \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

## Kubernetes targets

.PHONY: k8s-deploy
k8s-deploy: ## Deploy to Kubernetes
	@echo "Deploying to Kubernetes namespace $(NAMESPACE)..."
	kubectl apply -f deploy/kubernetes.yaml -n $(NAMESPACE)

.PHONY: k8s-delete
k8s-delete: ## Delete from Kubernetes
	@echo "Deleting from Kubernetes namespace $(NAMESPACE)..."
	kubectl delete -f deploy/kubernetes.yaml -n $(NAMESPACE) --ignore-not-found=true

.PHONY: k8s-restart
k8s-restart: ## Restart the deployment
	@echo "Restarting deployment in namespace $(NAMESPACE)..."
	kubectl rollout restart deployment/$(PROJECT_NAME) -n $(NAMESPACE)

.PHONY: k8s-logs
k8s-logs: ## Show application logs
	@echo "Showing logs for $(PROJECT_NAME) in namespace $(NAMESPACE)..."
	kubectl logs -f deployment/$(PROJECT_NAME) -n $(NAMESPACE)

.PHONY: k8s-status
k8s-status: ## Show deployment status
	@echo "Showing status for $(PROJECT_NAME) in namespace $(NAMESPACE)..."
	kubectl get pods,svc,configmap -l app=$(PROJECT_NAME) -n $(NAMESPACE)

.PHONY: k8s-describe
k8s-describe: ## Describe the deployment
	@echo "Describing deployment $(PROJECT_NAME) in namespace $(NAMESPACE)..."
	kubectl describe deployment/$(PROJECT_NAME) -n $(NAMESPACE)

.PHONY: k8s-config-create
k8s-config-create: ## Create ConfigMap from config file
	@echo "Creating ConfigMap from config/config.yaml..."
	kubectl create configmap $(PROJECT_NAME)-config \
		--from-file=config.yaml=config/config.yaml \
		-n $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -

.PHONY: k8s-secret-create
k8s-secret-create: ## Create Azure credentials secret (interactive)
	@echo "Creating Azure credentials secret..."
	@read -p "Azure Subscription ID: " SUBSCRIPTION_ID; \
	read -p "Resource Group: " RESOURCE_GROUP; \
	read -p "Cluster Name: " CLUSTER_NAME; \
	read -p "Tenant ID: " TENANT_ID; \
	read -p "Client ID: " CLIENT_ID; \
	read -s -p "Client Secret: " CLIENT_SECRET; \
	echo ""; \
	kubectl create secret generic azure-credentials \
		--from-literal=subscription-id="$$SUBSCRIPTION_ID" \
		--from-literal=resource-group="$$RESOURCE_GROUP" \
		--from-literal=cluster-name="$$CLUSTER_NAME" \
		--from-literal=tenant-id="$$TENANT_ID" \
		--from-literal=client-id="$$CLIENT_ID" \
		--from-literal=client-secret="$$CLIENT_SECRET" \
		-n $(NAMESPACE)

## Release targets

.PHONY: release-prepare
release-prepare: check ## Prepare for release (run checks)
	@echo "Preparing for release $(VERSION)..."
	@if [ -z "$(VERSION)" ] || [ "$(VERSION)" = "dev" ]; then \
		echo "Error: VERSION must be set for release"; \
		exit 1; \
	fi

.PHONY: release-build
release-build: release-prepare build-all ## Build release binaries
	@echo "Building release artifacts for version $(VERSION)..."
	@mkdir -p release
	@cd $(BUILD_DIR) && \
	for binary in $(BINARY_NAME)-*; do \
		if [ -f "$$binary" ]; then \
			tar -czf ../release/$$binary-$(VERSION).tar.gz $$binary; \
		fi; \
	done
	@echo "Release artifacts created in ./release/"

.PHONY: release-docker
release-docker: release-prepare docker-push ## Build and push release Docker image

## Utility targets

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -rf release
	rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
	docker image prune -f --filter label=stage=builder 2>/dev/null || true

.PHONY: clean-all
clean-all: clean ## Clean all artifacts including Docker images
	@echo "Cleaning all artifacts..."
	docker rmi $(DOCKER_IMAGE):$(DOCKER_TAG) 2>/dev/null || true
	docker rmi $(DOCKER_IMAGE):latest 2>/dev/null || true

.PHONY: version
version: ## Show version information
	@echo "Project: $(PROJECT_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"
	@echo "Go Version: $(GO_VERSION)"

.PHONY: env
env: ## Show environment variables
	@echo "Environment variables:"
	@echo "GOOS=$(GOOS)"
	@echo "GOARCH=$(GOARCH)"
	@echo "CGO_ENABLED=$(CGO_ENABLED)"
	@echo "DOCKER_REGISTRY=$(DOCKER_REGISTRY)"
	@echo "DOCKER_IMAGE=$(DOCKER_IMAGE)"
	@echo "DOCKER_TAG=$(DOCKER_TAG)"
	@echo "NAMESPACE=$(NAMESPACE)"
	@echo "KUBECONFIG=$(KUBECONFIG)"

# Default target
.DEFAULT_GOAL := help
