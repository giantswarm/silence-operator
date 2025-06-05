# Makefile.custom.mk - Custom targets for silence-operator
# This file contains custom make targets for project-specific operations

# ==================================================================================
# Helm Chart Integration
# ==================================================================================

# Helm chart directories
HELM_CHART_DIR := helm/silence-operator
HELM_TEMPLATES_DIR := $(HELM_CHART_DIR)/templates
CRD_BASES_DIR := config/crd/bases
CRD_TEMPLATE_FILE := $(HELM_TEMPLATES_DIR)/crds.yml

##@ Helm Chart Management

.PHONY: update-helm-crds
update-helm-crds: generate-crds ## Update Helm chart CRD templates from generated CRDs
	$(call log_build,"Updating Helm chart CRDs from generated manifests")
	@if [ ! -d "$(HELM_TEMPLATES_DIR)" ]; then \
		mkdir -p "$(HELM_TEMPLATES_DIR)"; \
	fi
	@echo "{{- if .Values.crds.install }}" > $(CRD_TEMPLATE_FILE)
	@for crd_file in $(CRD_BASES_DIR)/*.yaml; do \
		if [ -f "$$crd_file" ]; then \
			echo "Processing $$crd_file..."; \
			echo "---" >> $(CRD_TEMPLATE_FILE); \
			awk ' \
			/^---/ { next } \
			/^metadata:/ { \
				print $$0; \
				print "  labels:"; \
				print "    app.kubernetes.io/name: {{ template \"silence-operator.name\" . }}"; \
				print "    app.kubernetes.io/instance: {{ .Release.Name }}"; \
				next; \
			} \
			/^  annotations:/ { \
				print $$0; \
				print "    helm.sh/resource-policy: keep"; \
				next; \
			} \
			{ print $$0 } \
			' "$$crd_file" >> $(CRD_TEMPLATE_FILE); \
		fi; \
	done
	@echo "{{- end }}" >> $(CRD_TEMPLATE_FILE)
	$(call log_info,"Updated Helm chart CRD template: $(CRD_TEMPLATE_FILE)")

.PHONY: sync-helm-crds
sync-helm-crds: update-helm-crds ## Sync generated CRDs to Helm chart (alias for update-helm-crds)

# ==================================================================================
# Validation and Verification for Helm
# ==================================================================================

.PHONY: verify-helm-crds
verify-helm-crds: update-helm-crds ## Verify Helm CRDs are in sync with generated CRDs
	$(call log_build,"Verifying Helm CRDs are in sync")
	@if [ -n "$$(git status --porcelain $(CRD_TEMPLATE_FILE) 2>/dev/null)" ]; then \
		echo "$(WARN_COLOR)Helm CRD template is not up to date. Please run 'make update-helm-crds'$(NO_COLOR)"; \
		git diff $(CRD_TEMPLATE_FILE); \
		exit 1; \
	fi
	$(call log_info,"Helm CRDs are in sync with generated CRDs")

# ==================================================================================
# Development Workflow Integration
# ==================================================================================

.PHONY: generate-all-with-helm
generate-all-with-helm: generate-all update-helm-crds ## Generate all code and update Helm charts
	$(call log_info,"Generated all code and updated Helm charts")

# Add Helm CRD validation to the standard verification
.PHONY: verify-all-with-helm
verify-all-with-helm: verify-generate verify-helm-crds ## Verify all generated files including Helm CRDs
	$(call log_info,"All generated files verified successfully")
