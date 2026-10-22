# DO NOT EDIT. Generated with:
#
#    devctl
#
#    https://github.com/giantswarm/devctl/blob/055144a5a009e2cc2ccee52d5be12ad1a00cf448/pkg/gen/input/makefile/internal/file/Makefile.gen.app.mk.template
#

##@ App

# yq, helm-docs and ct run from PATH when installed (yq only as mikefarah's yq v4), otherwise
# from their container images; ct's container gets a TTY only when stdin is one.
YQ = $(if $(shell yq --version 2>/dev/null | grep -q mikefarah && echo yq),yq,docker run --rm -u $$(id -u) -v $${PWD}:/workdir mikefarah/yq:4.29.2)
HELM_DOCS = $(if $(shell command -v helm-docs 2>/dev/null),helm-docs,docker run --rm -u $$(id -u) -v $${PWD}:/helm-docs jnorwood/helm-docs:v1.14.2) --sort-values-order=file
CT = $(if $(shell command -v ct 2>/dev/null),ct,docker run $$(test -t 0 && echo -t) --rm -v /tmp/$(APPLICATION)-test:/wd --workdir=/wd --name ct $(IMAGE) ct)

# APPLICATION may be set by a later included makefile (Makefile.gen.go.mk derives it from the
# Go module, and Makefile.*.mk are included in name order), so nothing here may read it while
# make parses this file: DEPS is recursively expanded (=) and the guard is a recipe.
DEPS = $(shell find helm/$(APPLICATION)/charts -maxdepth 2 -name "Chart.yaml" -printf "%h\n" 2>/dev/null)

# A chart-only repository sets APPLICATION nowhere: it defaults to the one chart under helm/,
# and stays empty when there are several or none, so check-env stops. `?=` keeps a value set
# earlier (Makefile.custom.mk, the environment, the command line), and a later assignment
# (Makefile.gen.go.mk's `:=`, Makefile.zzz.custom.mk) replaces the default.
HELM_CHARTS = $(patsubst helm/%/Chart.yaml,%,$(wildcard helm/*/Chart.yaml))
APPLICATION ?= $(if $(word 2,$(HELM_CHARTS)),,$(HELM_CHARTS))

.PHONY: lint-chart check-env update-chart helm-docs update-deps

lint-chart: IMAGE := gsoci.azurecr.io/giantswarm/helm-chart-testing:v3.15.0
lint-chart: check-env ## Runs ct against the default chart.
	@echo "====> $@"
	rm -rf /tmp/$(APPLICATION)-test
	mkdir -p /tmp/$(APPLICATION)-test/helm
	cp -a ./helm/$(APPLICATION) /tmp/$(APPLICATION)-test/helm/
	architect helm template --dir /tmp/$(APPLICATION)-test/helm/$(APPLICATION)
	cd /tmp/$(APPLICATION)-test && $(CT) lint --validate-maintainers=false --charts="helm/$(APPLICATION)"
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
	@test -n "$(APPLICATION)" || { echo "APPLICATION is not defined and helm/ does not hold exactly one chart (found: $(or $(HELM_CHARTS),none)): set it in Makefile.custom.mk or run make APPLICATION=<chart>" >&2; exit 1; }

##@ Chart tests

# renovate: datasource=github-releases depName=helm-unittest/helm-unittest
HELM_UNITTEST_VERSION ?= v1.2.1
HELM_UNITTEST_ATTEMPTS ?= 3
HELM_UNITTEST_RETRY_DELAY ?= 5

.PHONY: helm-test install-helm-unittest

# helm-test renders the chart and asserts on the rendered manifests. It never talks to a
# cluster, pulls no image and needs no built chart, so it runs in under a second and it is
# the first thing to run after a template change. Cluster-level testing is app-test-suite,
# through the generated `execute-chart-tests` job and the repository's `.ats/main.yaml`;
# the two do not overlap and neither replaces the other.
#
# One `helm unittest` run per chart that has tests, rather than one run over the top-level
# chart. A suite asserts on the defaults of the chart it ships with, and a parent chart's
# `values.yaml` overrides those defaults for its subcharts -- so a subchart suite run from
# the parent is asserting against values its author never wrote. `--with-subchart=false`
# keeps each run inside the chart the suite belongs to.
#
# Both directory names are in use across the org: upstream charts vendored from Artifact
# Hub tend to use `unittests/`, our own charts use `tests/`. The `Chart.yaml` check rejects
# a nested directory that only shares the name, such as `tests/admission-webhooks/`.
# Nothing here reads APPLICATION, so this target carries no `check-env` prerequisite. A
# chart-only repository sets APPLICATION nowhere, and requiring it made `make helm-test`
# fail in exactly the repositories this target exists for.
# A chart under a placeholder directory such as `helm/{APP-NAME}` belongs to a template
# repository: its names render only once a repository created from it fills them in, so
# its suites cannot pass here and run on the rendered copy instead. It is skipped, as a
# chart reference with a placeholder is no chart for the catalog either.
helm-test: install-helm-unittest ## Run the chart unit tests (helm-unittest).
	@echo "====> $@"
	@found=""; \
	for d in $$(find helm -type d \( -name tests -o -name unittests \) 2>/dev/null | sort); do \
		c=$$(dirname "$$d"); \
		[ -f "$$c/Chart.yaml" ] || continue; \
		case "$$c" in *[{}]*) echo "skipping $$c: a template's placeholder chart"; continue;; esac; \
		ls "$$d"/*_test.yaml >/dev/null 2>&1 || continue; \
		found=1; \
		helm unittest -f "$$(basename "$$d")/*_test.yaml" --with-subchart=false "$$c" || exit 1; \
	done; \
	[ -n "$$found" ] || echo "no chart unit tests under helm/"

# Installs the pinned version, and REPLACES a different one. A guard that only checks that
# the plugin exists lets a developer keep an old version forever, and then a local run and
# a CI run are not the same run -- which is the whole point of pinning it here.
# helm v4 verifies a plugin's source by default and refuses this one, so `--verify=false`
# is passed whenever `helm plugin install` knows the flag; helm v3 has none and installs as before.
# The plugin's install hook downloads its binary with curl and no retry, so a transient
# network error (a connection reset) fails it; the install runs up to
# HELM_UNITTEST_ATTEMPTS times, HELM_UNITTEST_RETRY_DELAY seconds apart. A failed attempt
# can leave a half-installed plugin behind that the next install refuses, so it is
# uninstalled before each retry.
install-helm-unittest:
	@have=$$(helm plugin list 2>/dev/null | awk '$$1 == "unittest" {print $$2}'); \
	want=$$(echo "$(HELM_UNITTEST_VERSION)" | sed 's/^v//'); \
	if [ "$$have" != "$$want" ]; then \
		[ -z "$$have" ] || helm plugin uninstall unittest >/dev/null 2>&1 || true; \
		verify=""; \
		helm plugin install --help 2>&1 | grep -q -e '--verify' && verify="--verify=false"; \
		attempt=1; \
		until helm plugin install $$verify https://github.com/helm-unittest/helm-unittest --version $(HELM_UNITTEST_VERSION); do \
			[ "$$attempt" -lt "$(HELM_UNITTEST_ATTEMPTS)" ] || exit 1; \
			echo "helm-unittest install failed (attempt $$attempt of $(HELM_UNITTEST_ATTEMPTS)), retrying in $(HELM_UNITTEST_RETRY_DELAY)s" >&2; \
			helm plugin uninstall unittest >/dev/null 2>&1 || true; \
			sleep $(HELM_UNITTEST_RETRY_DELAY); \
			attempt=$$((attempt + 1)); \
		done; \
	fi

# Chart tests join `make test`, the interface CI calls (make-target-ci-interface ADR,
# giantswarm#36734). Prerequisite-only rule, carrying no recipe: it ADDS to the Go `test`
# recipe in Makefile.gen.go.mk rather than overriding it, and it DEFINES `test` in an app
# repository that has no Go suite behind it. Keep it last: the first target of the first
# included makefile is make's default goal.
test: helm-test
