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
CLIENT_GEN ?= $(GO) tool k8s.io/code-generator/cmd/client-gen
LISTER_GEN ?= $(GO) tool k8s.io/code-generator/cmd/lister-gen
INFORMER_GEN ?= $(GO) tool k8s.io/code-generator/cmd/informer-gen
HELM ?= $(GO) tool helm.sh/helm/v3/cmd/helm
HELMIFY ?= $(GO) tool github.com/arttor/helmify/cmd/helmify

MODULE := $(shell $(GO) list -m)

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

# Generated manifests
BUILD_MANIFEST_DIR := $(BUILD_DIR)/manifests

$(BUILD_DIR):
	mkdir -p "$(BUILD_DIR)"

$(LOCALBIN_DIR):
	mkdir -p "$(LOCALBIN_DIR)"

$(IMG_DIR):
	mkdir -p "$(IMG_DIR)"

$(BUILD_MANIFEST_DIR):
	mkdir -p "$(BUILD_MANIFEST_DIR)"

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
	rm -rf $(CURDIR)/internal/generated
	rm -rf $(BUILD_DIR)

##@ Development

.PHONY: generate
generate: controller-gen-objects generate-k8s-clients controller-gen-manifests ## Generate code.

.PHONY: controller-gen-objects
controller-gen-objects:
	$(CONTROLLER_GEN) object paths=./apis/...

.PHONY: controller-gen-manifests
controller-gen-manifests: controller-gen-objects generate-k8s-clients $(BUILD_MANIFEST_DIR)
	$(CONTROLLER_GEN) paths=./apis/... crd:crdVersions=v1 output:crd:artifacts:config=$(BUILD_MANIFEST_DIR)
	$(CONTROLLER_GEN) paths=./cmd/... rbac:roleName=kaschnit-scheduler output:rbac:artifacts:config=$(BUILD_MANIFEST_DIR)	

.PHONY: generate-k8s-clients
generate-k8s-clients: k8s-client-gen k8s-lister-gen k8s-informer-gen

.PHONY: k8s-client-gen
k8s-client-gen: controller-gen-objects
	$(CLIENT_GEN) \
		--clientset-name "scheduling" \
		--input-base $(MODULE)/apis \
		--input scheduling/v1 \
		--output-dir ./internal/generated/clients \
		--output-pkg $(MODULE)/internal/generated/clients

.PHONY: k8s-lister-gen
k8s-lister-gen: controller-gen-objects
	$(LISTER_GEN) \
		--output-dir ./internal/generated/listers \
		--output-pkg $(MODULE)/internal/generated/listers \
		./apis/scheduling/v1

.PHONY: k8s-informer-gen
k8s-informer-gen: controller-gen-objects k8s-client-gen k8s-lister-gen
	$(INFORMER_GEN) \
        --versioned-clientset-package $(MODULE)/internal/generated/clients/scheduling \
        --listers-package $(MODULE)/internal/generated/listers \
        --output-dir ./internal/generated/informers \
        --output-pkg $(MODULE)/internal/generated/informers \
        ./apis/scheduling/v1

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
	KO_DOCKER_REPO=kaschnit-scheduler \
		$(KO) build \
			--push=$(PUSH) \
			--platform=linux/$(shell $(GO) env GOARCH) \
			--tarball=$(IMG_TAR_FILE) \
			--bare \
			--tags=development \
			$(CMD)

.PHONY: chart
chart: generate image
	find $(BUILD_MANIFEST_DIR)/ manifests/ \
		-type f \
		-name "*.yaml" \
		-exec cat {} + \
		| \
		$(HELMIFY) -crd-dir -image-pull-secrets $(CHARTS_DIR)/kaschnit-scheduler

.PHONY: kind-delete
kind-delete:
	$(KIND) delete cluster --name "$(KIND_CLUSTER_NAME)"

.PHONY: kind-create
kind-create: kind-delete
	$(KIND) create cluster --name "$(KIND_CLUSTER_NAME)"

.PHONY: kind-deploy
kind-deploy: image chart kind-create
	$(KIND) load image-archive $(IMG_TAR_FILE) --name "$(KIND_CLUSTER_NAME)"
	$(HELM) install kaschnit-scheduler $(CHARTS_DIR)/kaschnit-scheduler \
		--values test/kind/values.yaml \
		--namespace kaschnit-scheduler \
		--create-namespace
	$(KUBECTL) apply -f test/kind/base
