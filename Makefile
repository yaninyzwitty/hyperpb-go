# See https://tech.davis-hansson.com/p/make/
SHELL := bash
.DELETE_ON_ERROR:
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := all
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules
MAKEFLAGS += --no-print-directory

BIN ?= $(abspath .tmp/bin)
CACHE := $(abspath .tmp/cache)

COPYRIGHT_YEARS := 2020-2025
LICENSE_IGNORE := -E -e "/testdata/"

# Set to use a different compiler. For example, `GO=go1.18rc1 make test`.
GO ?= go
GO_CMD := GOTOOLCHAIN=local $(GO)
GO_TAGS ?= ""

ASM_FILTER ?= ^github.com/bufbuild/fastpb
BENCHMARK ?= .
ifeq ($(PKG),)
	PKGS := ./...
else
	PKGS := $(PKG)
endif
PKG ?= .

TOOLS_MOD_DIR := ./internal/tools
PATH_SEP := ":"

.PHONY: help
help: ## Describe useful make targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "%-30s %s\n", $$1, $$2}'

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
	$(GO_CMD) test -tags=$(GO_TAGS) $(PKGS)

.PHONY: bench
bench: build ## Run benchmarks
	$(GO_CMD) run ./internal/prettybench -tags=$(GO_TAGS) -bench '$(BENCHMARK)' $(PKGS)

.PHONY: profile
profile: build ## Profile benchmarks and open them in pprof
	$(GO_CMD) test -bench '$(BENCHMARK)' -benchmem -run '^B' \
		-tags=$(GO_TAGS) \
		-benchtime 3s \
		-o fastpb.test \
		-cpuprofile fastpb.prof \
		$(PKG)
	$(GO_CMD) tool pprof -http localhost:8000 fastpb.test fastpb.prof

.PHONY: asm
asm: build ## Generate assembly output for manual inspection
	$(GO_CMD) test -tags=$(GO_TAGS) -c -o fastpb.test $(PKG)
	$(GO_CMD) tool objdump -gnu -s '$(ASM_FILTER)' fastpb.test | \
		$(GO_CMD) run ./internal/prettyasm \
			-info fileline \
			-prefix 'github.com/bufbuild/fastpb' \
			-nops \
			> fastpb.s 

.PHONY: build
build: generate ## Build all packages
	$(GO_CMD) build -tags=$(GO_TAGS) ./...

.PHONY: lint
lint: $(BIN)/golangci-lint ## Lint Go
	$(GO_CMD) vet -unsafeptr=false ./...
	$(BIN)/golangci-lint run

.PHONY: lintfix
lintfix: $(BIN)/golangci-lint ## Automatically fix some lint errors
	$(BIN)/golangci-lint run --fix

.PHONY: generate
generate: $(BIN)/license-header ## Regenerate code and licenses
	PATH="$(BIN)$(PATH_SEP)$(PATH)" $(GO_CMD) generate ./...
	@# We want to operate on a list of modified and new files, excluding
	@# deleted and ignored files. git-ls-files can't do this alone. comm -23 takes
	@# two files and prints the union, dropping lines common to both (-3) and
	@# those only in the second file (-2). We make one git-ls-files call for
	@# the modified, cached, and new (--others) files, and a second for the
	@# deleted files.
	comm -23 \
		<(git ls-files --cached --modified --others --no-empty-directory --exclude-standard | sort -u | grep -v $(LICENSE_IGNORE) ) \
		<(git ls-files --deleted | sort -u) | \
		xargs $(BIN)/license-header \
			--license-type apache \
			--copyright-holder "Buf Technologies, Inc." \
			--year-range "$(COPYRIGHT_YEARS)"

.PHONY: upgrade
upgrade: ## Upgrade dependencies
	go get -u -t ./... && go mod tidy -v

.PHONY: checkgenerate
checkgenerate:
	@# Used in CI to verify that `make generate` doesn't produce a diff.
	@echo git status --porcelain
	@if [[ -n "$$(git status --porcelain | tee /dev/stderr)" ]]; then \
	  git diff; \
	  false; \
	fi

$(BIN)/license-header: internal/tools/go.mod internal/tools/go.sum
	@mkdir -p $(@D)
	cd $(TOOLS_MOD_DIR) && \
		GOWORK=off $(GO_CMD) build -o $@ github.com/bufbuild/buf/private/pkg/licenseheader/cmd/license-header

$(BIN)/golangci-lint: internal/tools/go.mod internal/tools/go.sum
	@mkdir -p $(@D)
	cd $(TOOLS_MOD_DIR) && \
		GOWORK=off $(GO_CMD) build -o $@ github.com/golangci/golangci-lint/cmd/golangci-lint
