# DO NOT EDIT. Generated with:
#
#    devctl
#
#    https://github.com/giantswarm/devctl/blob/0ec3e49745962245bdd6d5c282d4272c40faec37/pkg/gen/input/makefile/internal/file/Makefile.gen.go.mk.template
#

APPLICATION    := $(shell go list -m | cut -d '/' -f 3)
BUILDTIMESTAMP := $(shell date -u '+%FT%TZ')
GITSHA1        := $(shell git rev-parse --verify HEAD)
MODULE         := $(shell go list -m)
# main() is usually in `main.go`, but sometimes in `cmd/main.go` (for example in newer kubebuilder projects)
MAIN_SOURCE    := $(shell if test -e cmd/main.go; then echo cmd/main.go; else echo main.go; fi)
OS             := $(shell go env GOOS)
SOURCES        := $(shell find . -name '*.go')
VERSION        := $(shell gitsemver get)
ifeq ($(OS), linux)
EXTLDFLAGS := -static
endif
LDFLAGS        ?= -w -linkmode 'auto' -extldflags '$(EXTLDFLAGS)' \
  -X '$(MODULE)/pkg/project.version=$(VERSION)' \
  -X '$(MODULE)/pkg/project.buildTimestamp=$(BUILDTIMESTAMP)' \
  -X '$(MODULE)/pkg/project.gitSHA=$(GITSHA1)'

.DEFAULT_GOAL := build

##@ Go

.PHONY: build build-darwin build-darwin-64 build-linux build-linux-arm64 build-windows-amd64
build: $(APPLICATION) ## Builds a local binary.
	@echo "====> $@"
build-darwin: $(APPLICATION)-darwin ## Builds a local binary for darwin/amd64.
	@echo "====> $@"
build-darwin-arm64: $(APPLICATION)-darwin-arm64 ## Builds a local binary for darwin/arm64.
	@echo "====> $@"
build-linux: $(APPLICATION)-linux ## Builds a local binary for linux/amd64.
	@echo "====> $@"
build-linux-arm64: $(APPLICATION)-linux-arm64 ## Builds a local binary for linux/arm64.
	@echo "====> $@"
build-windows-amd64: $(APPLICATION)-windows-amd64.exe ## Builds a local binary for windows/amd64.
	@echo "====> $@"

$(APPLICATION): $(APPLICATION)-v$(VERSION)-$(OS)-amd64
	@echo "====> $@"
	cp -a $< $@

$(APPLICATION)-darwin: $(APPLICATION)-v$(VERSION)-darwin-amd64
	@echo "====> $@"
	cp -a $< $@

$(APPLICATION)-darwin-arm64: $(APPLICATION)-v$(VERSION)-darwin-arm64
	@echo "====> $@"
	cp -a $< $@

$(APPLICATION)-linux: $(APPLICATION)-v$(VERSION)-linux-amd64
	@echo "====> $@"
	cp -a $< $@

$(APPLICATION)-linux-arm64: $(APPLICATION)-v$(VERSION)-linux-arm64
	@echo "====> $@"
	cp -a $< $@

$(APPLICATION)-windows-amd64.exe: $(APPLICATION)-v$(VERSION)-windows-amd64.exe
	@echo "====> $@"
	cp -a $< $@

$(APPLICATION)-v$(VERSION)-%-amd64: $(SOURCES)
	@echo "====> $@"
	CGO_ENABLED=0 GOOS=$* GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $@ "$(MAIN_SOURCE)"

$(APPLICATION)-v$(VERSION)-%-arm64: $(SOURCES)
	@echo "====> $@"
	CGO_ENABLED=0 GOOS=$* GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $@ "$(MAIN_SOURCE)"

$(APPLICATION)-v$(VERSION)-windows-amd64.exe: $(SOURCES)
	@echo "====> $@"
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $@ "$(MAIN_SOURCE)"

.PHONY: install
install: ## Install the application.
	@echo "====> $@"
	go build -ldflags "$(LDFLAGS)" -o "$(shell go env GOPATH)/bin/$(APPLICATION)" "$(MAIN_SOURCE)"

.PHONY: run
run: ## Runs go run main.go.
	@echo "====> $@"
	go run -ldflags "$(LDFLAGS)" -race "$(MAIN_SOURCE)"

.PHONY: clean
clean: ## Cleans the binary.
	@echo "====> $@"
	rm -f $(APPLICATION)*
	go clean

.PHONY: imports
imports: ## Runs goimports.
	@echo "====> $@"
	goimports -local $(MODULE) -w .

.PHONY: lint
lint: ## Runs golangci-lint.
	@echo "====> $@"
	golangci-lint run -E gosec -E goconst --timeout=15m ./...

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: nancy
nancy: ## Runs nancy (requires v1.0.37 or newer).
	@echo "====> $@"
	CGO_ENABLED=0 go list -json -m all | nancy sleuth --skip-update-check --quiet --exclude-vulnerability-file ./.nancy-ignore --additional-exclude-vulnerability-files ./.nancy-ignore.generated

# Race detector needs a C toolchain. The architect CI image has none and runs
# with CGO_ENABLED=0, so degrade to cgo-free there; everywhere a compiler exists
# (laptops, coding agents, GitHub Actions, any cgo-capable runner) keeps -race.
RACE := $(shell { [ "$${CGO_ENABLED:-1}" != "0" ] && { command -v gcc || command -v clang; } >/dev/null 2>&1; } && echo -race)

.PHONY: test
test: ## Runs go test with default values (race detector when a C toolchain is available).
	@echo "====> $@"
	go test -ldflags "$(LDFLAGS)" $(RACE) ./...

.PHONY: build-docker
build-docker: build-linux ## Builds docker image to registry.
	@echo "====> $@"
	cp -a $(APPLICATION)-linux $(APPLICATION)
	docker build -t ${APPLICATION}:${VERSION} .
