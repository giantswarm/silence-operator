# Kubernetes API Code Generation and Manifest Management
# Enhanced version following modern Kubernetes operator best practices
# 
# This file manages:
# - Custom Resource Definition (CRD) generation
# - Go code generation (deepcopy, client, etc.)
# - RBAC manifest generation
# - Webhook configuration generation
# - Kubernetes manifest validation
# - Code quality tools (linting, formatting, vetting)
# - Container and build tools integration

##@ Code Generation and Kubernetes Manifests

# ==================================================================================
# Configuration and Tool Versions
# ==================================================================================

# Core tool versions (aligned with kubebuilder defaults and auto-detection)
CONTROLLER_TOOLS_VERSION ?= v0.18.0
KUSTOMIZE_VERSION ?= v5.6.0
# Auto-detect ENVTEST version from controller-runtime dependency
ENVTEST_VERSION ?= $(shell go list -m -f "{{ .Version }}" sigs.k8s.io/controller-runtime 2>/dev/null | awk -F'[v.]' '{printf "release-%d.%d", $$2, $$3}')
# Auto-detect Kubernetes version from k8s.io/api dependency  
ENVTEST_K8S_VERSION ?= $(shell go list -m -f "{{ .Version }}" k8s.io/api 2>/dev/null | awk -F'[v.]' '{printf "1.%d", $$3}')

# Code quality and development tool versions
GOLANGCI_LINT_VERSION ?= v2.1.6
GINKGO_VERSION ?= v2.23.4

# Directories
API_DIR := $(shell [ -d api ] && echo api || echo pkg/apis)
CRD_DIR := config/crd/bases
CRD_OPTIONS := config/crd
RBAC_DIR := config/rbac
WEBHOOK_DIR := config/webhook
SCRIPTS_DIR := hack

# Tool installation directory
LOCALBIN ?= $(shell pwd)/bin

# Tool binaries
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
KUSTOMIZE ?= $(LOCALBIN)/kustomize
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
GINKGO ?= $(LOCALBIN)/ginkgo

# Generation options
BOILERPLATE := $(SCRIPTS_DIR)/boilerplate.go.txt
CURRENT_YEAR := $(shell date +'%Y')

# Output formatting and colors - simplified for bash compatibility
BUILD_COLOR := $(shell printf '\033[0;34m')
GEN_COLOR := $(shell printf '\033[0;32m')
WARN_COLOR := $(shell printf '\033[0;33m')
NO_COLOR := $(shell printf '\033[0m')

# Helper functions for output
define log_info
@echo "$(GEN_COLOR)ℹ️  $(1)$(NO_COLOR)"
endef

define log_warn
@echo "$(WARN_COLOR)⚠️  $(1)$(NO_COLOR)"
endef

define log_build
@echo "$(BUILD_COLOR)🔨 $(1)$(NO_COLOR)"
endef

# Generated file patterns
DEEPCOPY_BASE := zz_generated.deepcopy
DEEPCOPY_FILES := $(shell find $(API_DIR) -name "$(DEEPCOPY_BASE).go" 2>/dev/null)

# Manifest output directories
MANIFEST_DIRS := $(CRD_DIR) $(RBAC_DIR) $(WEBHOOK_DIR)

# ==================================================================================
# Main Generation Targets
# ==================================================================================

.PHONY: generate
generate: generate-deepcopy generate-manifests ## Generate all code and manifests
	$(call log_info,"Code generation completed successfully")

.PHONY: manifests  
manifests: generate-crds generate-rbac generate-webhook ## Generate all Kubernetes manifests
	$(call log_info,"Kubernetes manifest generation completed")

.PHONY: generate-all
generate-all: generate manifests ## Generate everything (code + manifests)
	$(call log_info,"Complete generation finished")

# ==================================================================================
# Code Generation Targets
# ==================================================================================

.PHONY: generate-deepcopy
generate-deepcopy: $(CONTROLLER_GEN) ## Generate deepcopy methods for API types
	$(call log_build,"Generating deepcopy methods")
	@if [ ! -d "$(API_DIR)" ]; then \
		$(call log_warn,API directory $(API_DIR) not found, skipping deepcopy generation); \
		exit 0; \
	fi
	@$(CONTROLLER_GEN) object:headerFile="$(BOILERPLATE)",year="$(CURRENT_YEAR)" paths="./$(API_DIR)/..." || { \
		$(call log_warn,Failed to generate deepcopy methods); \
		exit 1; \
	}
	@$(call log_info,Deepcopy generation completed)

.PHONY: generate-client
generate-client: generate-deepcopy ## Generate Kubernetes client code (if needed)
	$(call log_info,"Client generation skipped (using controller-runtime)")

# ==================================================================================
# Kubernetes Manifest Generation
# ==================================================================================

.PHONY: generate-crds
generate-crds: $(CONTROLLER_GEN) | $(CRD_DIR) ## Generate Custom Resource Definitions
	$(call log_build,"Generating CRDs with enhanced validation")
	@$(CONTROLLER_GEN) crd:allowDangerousTypes=true \
		paths="./$(API_DIR)/..." \
		output:crd:artifacts:config="$(CRD_DIR)"
	@$(call log_info,"CRD generation completed")

.PHONY: generate-rbac  
generate-rbac: $(CONTROLLER_GEN) | $(RBAC_DIR) ## Generate RBAC manifests
	$(call log_build,"Generating RBAC manifests")
	@$(CONTROLLER_GEN) rbac:roleName=manager-role \
		paths="./internal/controller/..." \
		output:rbac:artifacts:config="$(RBAC_DIR)"
	@$(call log_info,"RBAC generation completed")

.PHONY: generate-webhook
generate-webhook: $(CONTROLLER_GEN) | $(WEBHOOK_DIR) ## Generate webhook manifests
	$(call log_build,"Generating webhook configurations")
	@if [ -d "./internal/webhook" ]; then \
		$(CONTROLLER_GEN) webhook \
			paths="./internal/webhook/..." \
			output:webhook:artifacts:config="$(WEBHOOK_DIR)"; \
	else \
		echo "No webhook directory found, skipping webhook generation"; \
	fi
	@$(call log_info,"Webhook generation completed")

.PHONY: generate-manifests
generate-manifests: generate-crds generate-rbac generate-webhook ## Generate all Kubernetes manifests
	$(call log_info,"All manifest generation completed")

# Legacy alias for backward compatibility
.PHONY: manifests-legacy
manifests-legacy: generate-manifests
	@$(call log_warn,"'manifests-legacy' is deprecated, use 'generate-manifests'")

# ==================================================================================
# Code Quality and Development Tools
# ==================================================================================

.PHONY: fmt
fmt: ## Run go fmt against code
	$(call log_build,"Running go fmt")
	@go fmt ./...
	@$(call log_info,"Code formatting completed")

.PHONY: vet
vet: ## Run go vet against code
	$(call log_build,"Running go vet")
	@go vet ./...
	@$(call log_info,"Code vetting completed")

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(call log_build,"Running golangci-lint")
	@$(GOLANGCI_LINT) run -E gosec -E goconst --timeout=15m ./...
	@$(call log_info,"Linting completed")

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(call log_build,"Running golangci-lint with auto-fix")
	@$(GOLANGCI_LINT) run -E gosec -E goconst --timeout=15m ./... --fix
	@$(call log_info,"Linting with fixes completed")

.PHONY: lint-config
lint-config: golangci-lint ## Verify golangci-lint linter configuration
	$(call log_build,"Verifying golangci-lint configuration")
	@$(GOLANGCI_LINT) config verify
	@$(call log_info,"Lint configuration verified")

# ==================================================================================
# Testing
# ==================================================================================

.PHONY: test
test: ginkgo envtest ## Run tests with Ginkgo and envtest
	$(call log_build,"Running tests with Ginkgo")
	@KUBEBUILDER_ASSETS="$$($(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" \
		$(GINKGO) -p --nodes 4 -randomize-all --randomize-suites --cover ./...
	@$(call log_info,"Testing completed")

# ==================================================================================
# Validation and Verification
# ==================================================================================

.PHONY: verify-generate
verify-generate: ## Verify that generated files are up to date
	@$(call log_build,"Verifying generated files are up to date")
	@if [ -n "$$(git status --porcelain 2>/dev/null)" ]; then \
		echo "$(WARN_COLOR)⚠️  Working directory has uncommitted changes before verification$(NO_COLOR)"; \
	fi
	@$(MAKE) clean-generated
	@$(MAKE) generate-all
	@if [ -n "$$(git status --porcelain 2>/dev/null | grep -E '\.(go|yaml)$$')" ]; then \
		echo "$(WARN_COLOR)⚠️  Generated files are not up to date. Please run 'make generate-all'$(NO_COLOR)"; \
		git status --porcelain | grep -E '\.(go|yaml)$$'; \
		exit 1; \
	fi
	@$(call log_info,"Generated files are up to date")

.PHONY: validate-crds
validate-crds: $(KUSTOMIZE) generate-crds ## Validate generated CRDs
	$(call log_build,"Validating CRDs")
	@for crd in $(CRD_DIR)/*.yaml; do \
		if [ -f "$$crd" ]; then \
			echo "Validating $$crd"; \
			$(KUSTOMIZE) cfg tree "$$crd" > /dev/null || exit 1; \
		fi; \
	done
	@$(call log_info,"CRD validation completed")

.PHONY: validate-manifests  
validate-manifests: validate-crds $(KUSTOMIZE) ## Validate all generated manifests
	$(call log_build,"Validating Kubernetes manifests")
	@if [ -d "$(CRD_OPTIONS)" ]; then \
		$(KUSTOMIZE) build "$(CRD_OPTIONS)" > /dev/null && \
		$(call log_info,"CRD kustomization validated"); \
	fi
	@if [ -d "$(RBAC_DIR)" ]; then \
		$(KUSTOMIZE) cfg tree "$(RBAC_DIR)" > /dev/null && \
		$(call log_info,"RBAC manifests validated"); \
	fi

# ==================================================================================
# Tool Installation and Management
# ==================================================================================

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	$(call log_build,"Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)")
	@$(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path || { \
		$(call log_warn,"Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)"); \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

# Code quality tool installation
.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

.PHONY: ginkgo
ginkgo: $(GINKGO) ## Download ginkgo locally if necessary.
$(GINKGO): $(LOCALBIN)
	$(call go-install-tool,$(GINKGO),github.com/onsi/ginkgo/v2/ginkgo,$(GINKGO_VERSION))

.PHONY: install-tools
install-tools: controller-gen kustomize envtest golangci-lint ginkgo ## Install all development tools
	$(call log_info,"All development tools installed successfully")

.PHONY: update-tools
update-tools: clean-tools install-tools ## Update all tools to latest versions
	$(call log_info,"Tools updated successfully")

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary  
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
	echo "$(BUILD_COLOR)🔨 Downloading $(2)@$(3)$(NO_COLOR)" ;\
	set -e ;\
	package=$(2)@$(3) ;\
	rm -f $(1) || true ;\
	GOBIN=$(LOCALBIN) go install $${package} ;\
	mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef

# ==================================================================================
# Cleanup Targets
# ==================================================================================

.PHONY: clean-generated
clean-generated: ## Clean all generated files
	$(call log_build,"Cleaning generated files")
	@rm -rf $(CRD_DIR)/*.yaml 2>/dev/null || true
	@if [ -n "$(DEEPCOPY_FILES)" ]; then \
		rm -f $(DEEPCOPY_FILES); \
	fi
	@find $(API_DIR) -name "$(DEEPCOPY_BASE).go" -delete 2>/dev/null || true
	@$(call log_info,"Generated files cleaned")

.PHONY: clean-tools
clean-tools: ## Clean downloaded tools
	$(call log_build,"Cleaning tools")
	@rm -rf $(LOCALBIN)
	@$(call log_info,"Tools cleaned")

.PHONY: clean-manifests
clean-manifests: ## Clean generated Kubernetes manifests
	$(call log_build,"Cleaning Kubernetes manifests")
	@rm -rf $(CRD_DIR)/*.yaml $(RBAC_DIR)/role.yaml $(WEBHOOK_DIR)/*.yaml 2>/dev/null || true
	@$(call log_info,"Manifests cleaned")

.PHONY: clean-all
clean-all: clean-generated clean-manifests ## Clean all generated content
	$(call log_info,"All generated content cleaned")

# ==================================================================================
# Directory Creation
# ==================================================================================

$(LOCALBIN):
	@mkdir -p $(LOCALBIN)

$(CRD_DIR):
	@mkdir -p $(CRD_DIR)

$(RBAC_DIR):
	@mkdir -p $(RBAC_DIR)

$(WEBHOOK_DIR):
	@mkdir -p $(WEBHOOK_DIR)

# ==================================================================================
# Help and Information
# ==================================================================================

.PHONY: help-kubebuilder
help-kubebuilder: ## Show help for Kubebuilder targets
	@echo "$(BUILD_COLOR)Kubebuilder Code Generation Targets:$(NO_COLOR)"
	@echo ""
	@echo "  $(GEN_COLOR)Code Generation:$(NO_COLOR)"
	@echo "    generate             Generate all code and manifests (main target)"
	@echo "    generate-all         Generate everything (code + manifests)"
	@echo "    generate-deepcopy    Generate deepcopy methods for API types"
	@echo "    generate-client      Generate Kubernetes client code"
	@echo "    generate-crds        Generate Custom Resource Definitions" 
	@echo "    generate-rbac        Generate RBAC manifests"
	@echo "    generate-webhook     Generate webhook configurations"
	@echo "    generate-manifests   Generate all Kubernetes manifests"
	@echo "    manifests            Generate all Kubernetes manifests (alias)"
	@echo ""
	@echo "  $(GEN_COLOR)Code Quality & Development:$(NO_COLOR)"
	@echo "    fmt                  Run go fmt against code"
	@echo "    vet                  Run go vet against code"
	@echo "    lint                 Run golangci-lint linter"
	@echo "    lint-fix             Run golangci-lint with auto-fix"
	@echo "    lint-config          Verify golangci-lint configuration"
	@echo "    test                 Run tests with Ginkgo and envtest"
	@echo ""
	@echo "  $(GEN_COLOR)Validation:$(NO_COLOR)"
	@echo "    verify-generate      Verify generated files are up to date"
	@echo "    validate-crds        Validate generated CRDs"
	@echo "    validate-manifests   Validate all generated manifests"
	@echo ""
	@echo "  $(GEN_COLOR)Tool Management:$(NO_COLOR)"
	@echo "    install-tools        Install all development tools"
	@echo "    update-tools         Update tools to latest versions"
	@echo "    clean-tools          Clean downloaded tools"
	@echo "    controller-gen       Install controller-gen tool"
	@echo "    kustomize            Install kustomize tool"
	@echo "    envtest              Install setup-envtest tool"
	@echo "    setup-envtest        Setup envtest binaries for testing"
	@echo "    golangci-lint        Install golangci-lint linting tool"
	@echo "    ginkgo               Install Ginkgo testing framework"
	@echo ""
	@echo "  $(GEN_COLOR)Cleanup:$(NO_COLOR)"
	@echo "    clean-generated      Clean generated code files"
	@echo "    clean-manifests      Clean generated Kubernetes manifests"
	@echo "    clean-all            Clean all generated content"
