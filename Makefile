.DEFAULT_GOAL := help

## Shell config
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

## Tool Binaries
GO ?= go
KUBECTL ?= $(GO) tool k8s.io/kubernetes/cmd/kubectl
KIND ?= $(GO) tool sigs.k8s.io/kind
KUSTOMIZE ?= $(GO) tool sigs.k8s.io/kustomize/kustomize/v5
GOLANGCI_LINT ?= $(GO) tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint
KO ?= $(GO) tool github.com/google/ko
KUBEBUILDER ?= $(GO) tool sigs.k8s.io/kubebuilder/v4
CONTROLLER_GEN ?= $(GO) tool sigs.k8s.io/controller-tools/cmd/controller-gen
HELM ?= $(GO) tool helm.sh/helm/v3/cmd/helm
HELMIFY ?= $(GO) tool github.com/arttor/helmify/cmd/helmify

# KIND
KIND_CLUSTER_NAME = "kind-scheduler-test"

CMD := $(CURDIR)/cmd/scheduler

## Location for build artifacts
BUILD_DIR := $(CURDIR)/build

# Binary
LOCALBIN_DIR := $(BUILD_DIR)/bin

# Image
IMG_DIR := $(BUILD_DIR)/image
IMG_TAR_FILE := $(IMG_DIR)/scheduler.tar

# Helm
CHARTS_DIR := $(BUILD_DIR)/charts

# CRD
CRD_DIR := $(BUILD_DIR)/crds

$(BUILD_DIR):
	mkdir -p "$(BUILD_DIR)"

$(LOCALBIN_DIR):
	mkdir -p "$(LOCALBIN_DIR)"

$(IMG_DIR):
	mkdir -p "$(IMG_DIR)"

$(CRD_DIR):
	mkdir -p "$(CRD_DIR)"

$(CHARTS_DIR):
	mkdir -p "$(CHARTS_DIR)"

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: clean
clean: ## Clean up files.
	find . -name .DS_Store -type f -delete
	find . -name zz_generated.*.go -type f -delete
	rm -rf $(BUILD_DIR)

##@ Development

.PHONY: generate
generate: controller-gen-objects controller-gen-manifests ## Generate code.

.PHONY: controller-gen-objects
controller-gen-objects:
	$(CONTROLLER_GEN) object paths=./...

.PHONY: controller-gen-manifests
controller-gen-manifests: $(CRD_DIR)
	$(CONTROLLER_GEN) paths="./..." \
		crd:crdVersions=v1 output:crd:artifacts:config=$(CRD_DIR) \
		rbac:roleName=custom-scheduler output:rbac:artifacts:config=$(CRD_DIR)	

.PHONY: go-tidy
go-tidy: generate ## Tidy go.mod and go.sum.
	$(GO) mod tidy

.PHONY: go-tidy-check
go-tidy-check: generate ## Check if go.mod and go.sum are tidy.
	$(GO) mod tidy --diff

.PHONY: go-mod-download
go-mod-download: generate ## Download dependencies from go.mod and go.sum.
	$(GO) mod download

.PHONY: install-deps
install-deps: go-mod-download ## Install dependencies.

.PHONY: lint
lint: generate ## Run linters.
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: generate ## Run linters and perform fixes.
	$(GOLANGCI_LINT) run --fix

.PHONY: test
test: TESTFLAGS := -v -race
test: TESTTARGET := ./...
test: generate ## Run unit tests.
	$(GO) test $(TESTFLAGS) $(TESTTARGET)

##@ Build

.PHONY: build
build: generate $(LOCALBIN_DIR) ## Build manager binary.
	$(GO) build -o $(LOCALBIN_DIR)/scheduler $(CMD)

.PHONY: run
run: generate ## Run a controller from your host.
	$(GO) run $(CMD)

.PHONY: image
image: PUSH := false
image: generate $(IMG_DIR) ## Build an image and optionally push it.
	KO_DOCKER_REPO=kaschnit-custom-scheduler \
		$(KO) build \
			--push=$(PUSH) \
			--platform=linux/$(shell $(GO) env GOARCH) \
			--tarball=$(IMG_TAR_FILE) \
			--bare \
			--tags=development \
			$(CMD)

.PHONY: chart
chart: generate image
	find manifests/crds/ manifests/scheduler/ \
		-type f \
		-name "*.yaml" \
		-exec cat {} + \
		| \
		$(HELMIFY) -crd-dir -image-pull-secrets $(CHARTS_DIR)/custom-scheduler

.PHONY: kind-delete
kind-delete:
	$(KIND) delete cluster --name "$(KIND_CLUSTER_NAME)"

.PHONY: kind-create
kind-create: kind-delete
	$(KIND) create cluster --name "$(KIND_CLUSTER_NAME)"

.PHONY: kind-deploy
kind-deploy: chart kind-create
	$(KIND) load image-archive $(IMG_TAR_FILE) --name "$(KIND_CLUSTER_NAME)"
	$(KUBECTL) apply -f test/kind/namespace.yaml
	$(KUBECTL) apply -f test/kind/pc.yaml
# 	TODO REPLACE WITH HELM COMMAND
	$(KUBECTL) apply -k test/kind/scheduler
