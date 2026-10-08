# DO NOT EDIT. Generated with:
#
#    devctl
#
#    https://github.com/giantswarm/devctl/blob/35e98f40f577d35ab7b8f049ca322e0332b27d03/pkg/gen/input/makefile/internal/file/Makefile.gen.app.mk.template
#

##@ App

YQ=docker run --rm -u $$(id -u) -v $${PWD}:/workdir mikefarah/yq:4.29.2
HELM_DOCS=docker run --rm -u $$(id -u) -v $${PWD}:/helm-docs jnorwood/helm-docs:v1.14.2 --sort-values-order=file

# APPLICATION may be set by a later included makefile (Makefile.gen.go.mk derives it from the
# Go module, and Makefile.*.mk are included in name order), so nothing here may read it while
# make parses this file: DEPS is recursively expanded (=) and the guard is a recipe.
DEPS = $(shell find helm/$(APPLICATION)/charts -maxdepth 2 -name "Chart.yaml" -printf "%h\n" 2>/dev/null)

.PHONY: lint-chart check-env update-chart helm-docs update-deps

lint-chart: IMAGE := giantswarm/helm-chart-testing:v3.0.0-rc.1
lint-chart: check-env ## Runs ct against the default chart.
	@echo "====> $@"
	rm -rf /tmp/$(APPLICATION)-test
	mkdir -p /tmp/$(APPLICATION)-test/helm
	cp -a ./helm/$(APPLICATION) /tmp/$(APPLICATION)-test/helm/
	architect helm template --dir /tmp/$(APPLICATION)-test/helm/$(APPLICATION)
	docker run -it --rm -v /tmp/$(APPLICATION)-test:/wd --workdir=/wd --name ct $(IMAGE) ct lint --validate-maintainers=false --charts="helm/$(APPLICATION)"
	rm -rf /tmp/$(APPLICATION)-test

update-chart: check-env ## Sync chart with upstream repo.
	@echo "====> $@"
	vendir sync
	$(MAKE) update-deps

update-deps: check-env ## Update main Chart.yaml with new local dep versions, then the Helm dependencies.
	@echo "====> $@"
	@for dep in $(DEPS); do \
		dep_name=$$(basename $$dep) && \
		new_version=`$(YQ) .version helm/$(APPLICATION)/charts/$$dep_name/Chart.yaml` && \
		$(YQ) -i e "with(.dependencies[]; select(.name == \"$$dep_name\") | .version = \"$$new_version\")" helm/$(APPLICATION)/Chart.yaml; \
	done
	cd helm/$(APPLICATION) && helm dependency update

helm-docs: check-env ## Update $(APPLICATION) README.
	$(HELM_DOCS) -c helm/$(APPLICATION) -g helm/$(APPLICATION)

check-env:
	@test -n "$(APPLICATION)" || { echo "APPLICATION is not defined: set it in Makefile.custom.mk or run make APPLICATION=<chart>" >&2; exit 1; }
