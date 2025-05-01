# DO NOT EDIT. Generated with:
#
#    devctl
#
#    https://github.com/giantswarm/devctl/blob/4c09e629d5ccd51a8d9247011315c2e35d4613d3/pkg/gen/input/makefile/internal/file/Makefile.gen.k8sapi.mk.template
#

# Directories.
API_DIR := $(shell [ -d api ] &&  echo api || echo pkg/apis)# default to api, fall back to pkg/apis
CRD_DIR := config/crd
SCRIPTS_DIR := hack
GOBIN_DIR := $(abspath hack/bin)
CONTROLLER_GEN := $(abspath $(GOBIN_DIR)/controller-gen)

# Colors
BUILD_COLOR = ""
GEN_COLOR = ""
NO_COLOR = ""
ifneq (, $(shell command -v tput))
ifeq ($(shell test `tput colors` -ge 8 && echo "yes"), yes)
BUILD_COLOR=$(shell echo -e "\033[0;34m")
GEN_COLOR=$(shell echo -e "\033[0;32m")
NO_COLOR=$(shell echo -e "\033[0m")
endif
endif

# Inputs
DEEPCOPY_BASE = zz_generated.deepcopy
BOILERPLATE = $(SCRIPTS_DIR)/boilerplate.go.txt
YEAR = $(shell date +'%Y')

DEEPCOPY_FILES := $(shell find $(API_DIR) -name $(DEEPCOPY_BASE).go)

all: generate

$(CONTROLLER_GEN):
	@echo "$(BUILD_COLOR)Building controller-gen$(NO_COLOR)"
	GOBIN=$(GOBIN_DIR) go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
.PHONY: generate
generate:
	@$(MAKE) generate-deepcopy
	@$(MAKE) generate-manifests

.PHONY: verify
verify:
	@$(MAKE) clean-generated
	@$(MAKE) generate
	git diff --exit-code

.PHONY: generate-deepcopy
generate-deepcopy: $(CONTROLLER_GEN)
	@echo "$(GEN_COLOR)Generating deepcopy$(NO_COLOR)"
	$(CONTROLLER_GEN) \
	object:headerFile=$(BOILERPLATE),year=$(YEAR) \
	paths=./$(API_DIR)/...

.PHONY: generate-manifests
generate-manifests: $(CONTROLLER_GEN)
	@echo "$(GEN_COLOR)Generating CRDs$(NO_COLOR)"
	$(CONTROLLER_GEN) \
	crd \
	paths=./$(API_DIR)/... \
	output:dir="./$(CRD_DIR)"

.PHONY: clean-generated
clean-generated:
	@echo "$(GEN_COLOR)Cleaning generated files$(NO_COLOR)"
	rm -rf $(CRD_DIR) $(DEEPCOPY_FILES)

.PHONY: clean-tools
clean-tools:
	@echo "$(GEN_COLOR)Cleaning tools$(NO_COLOR)"
	rm -rf $(GOBIN_DIR)
