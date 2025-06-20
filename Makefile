# Copyright 2025 Buf Technologies, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http:#www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# See https://tech.davis-hansson.com/p/make/
SHELL := bash
.DELETE_ON_ERROR:
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := all
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules
MAKEFLAGS += --no-print-directory

BIN := .tmp/bin
export PATH := $(abspath $(BIN)):$(PATH)
export GOBIN := $(abspath $(BIN))

COPYRIGHT_YEARS := 2025
LICENSE_IGNORE := testdata/

BUF_VERSION := v1.50.0
LINT_VERSION := v2.1.6 # Keep in sync w/ .github/workflows/ci.yaml.

GO ?= go
GO := GOTOOLCHAIN=local $(GO)
TAGS ?= ""

ASM_FILTER ?= ^github.com/bufbuild/fastpb
BENCHMARK ?= .
ifeq ($(PKG),)
	PKGS := ./...
else
	PKGS := $(PKG)
endif
PKG ?= .
TESTFLAGS ?=
BENCHFLAGS ?= -benchmem

.PHONY: help
help: ## Describe useful make targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "%-30s %s\n", $$1, $$2}'

.PHONY: all
all: ## Build, test, and lint (default)
	$(MAKE) test
	$(MAKE) lint

.PHONY: clean
clean: ## Delete intermediate build artifacts
	@# -X only removes untracked files, -d recurses into directories, -f actually removes files/dirs
	git clean -Xdf

.PHONY: test
test: build ## Run unit tests
	$(GO) test -tags=$(TAGS) $(PKGS) $(TESTFLAGS)

.PHONY: bench
bench: build ## Run benchmarks
	$(GO) run ./internal/tools/bench -bench '$(BENCHMARK)' $(BENCHFLAGS) \
		-tags=$(TAGS) \
		$(PKGS)

.PHONY: profile
profile: build ## Profile benchmarks and open them in pprof
	$(GO) test -bench '$(BENCHMARK)' $(BENCHFLAGS) -run '^B' \
		-tags=$(TAGS) \
		-benchtime 3s \
		-o fastpb.test \
		-cpuprofile fastpb.prof \
		$(PKG)
	$(GO) tool pprof -http localhost:8000 fastpb.test fastpb.prof

.PHONY: asm
asm: build ## Generate assembly output for manual inspection
	$(GO) test -tags=$(TAGS) -c -o fastpb.test $(PKG) $(TESTFLAGS)
	$(GO) run ./internal/tools/objdump \
		-s '$(ASM_FILTER)' \
		-info fileline \
		-prefix 'github.com/bufbuild/fastpb' \
		-nops \
		-o fastpb.s \
		fastpb.test
	
.PHONY: build
build: generate ## Build all packages
	$(GO) build -tags=$(TAGS) $(PKGS)

.PHONY: lint
lint: $(BIN)/golangci-lint ## Lint
	$(GO) vet -unsafeptr=false ./...
	$(BIN)/golangci-lint run \
		--modules-download-mode=readonly \
		--timeout=3m0s

.PHONY: lintfix
lintfix: $(BIN)/golangci-lint ## Automatically fix some lint errors
	$(BIN)/golangci-lint run \
		--modules-download-mode=readonly \
		--timeout=3m0s \
		--fix

.PHONY: generate
generate: $(BIN)/buf $(BIN)/license-header ## Regenerate code and licenses
	$(GO) generate ./...
	@#$(BIN)/buf generate --clean
	$(BIN)/license-header \
		--license-type apache \
		--copyright-holder "Buf Technologies, Inc." \
		--year-range "$(COPYRIGHT_YEARS)" \
		--ignore $(LICENSE_IGNORE)

.PHONY: upgrade
upgrade: ## Upgrade dependencies
	go mod edit -toolchain=$(GO_MOD_GOTOOLCHAIN)
	go get -u -t ./...
	go mod tidy -v

.PHONY: checkgenerate
checkgenerate:
	@# Used in CI to verify that `make generate` doesn't produce a diff.
	git --no-pager diff --exit-code >&2

$(BIN)/buf: Makefile
	@mkdir -p $(@D)
	$(GO) install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)

$(BIN)/license-header: Makefile
	@mkdir -p $(@D)
	$(GO) install github.com/bufbuild/buf/private/pkg/licenseheader/cmd/license-header@$(BUF_VERSION)

$(BIN)/golangci-lint: Makefile
	@mkdir -p $(@D)
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(LINT_VERSION)