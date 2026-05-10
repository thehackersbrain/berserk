# berserk — build & developer workflow
# Run `make help` for the target list.

BINARY      := berserk
PREFIX      ?= /usr/local
BINDIR      ?= $(PREFIX)/bin
# CONFIGDIR holds config.yaml and the tool catalog yaml files. berserk reads
# every *.yaml/*.yml here (except config.yaml) as part of its tool registry.
CONFIGDIR   ?= /usr/share/berserk
# DESTDIR is the staged-install root, used by packagers (.deb, AUR, RPM).
# Leave empty for a normal install; set on the command line for staging:
#   make install DESTDIR=/tmp/pkgroot PREFIX=/usr
DESTDIR     ?=

LDFLAGS     := -s -w
GOFLAGS     := -trimpath -ldflags '$(LDFLAGS)'

DIST        := dist
PLATFORMS   := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

GO          ?= go
GOBIN       ?= $(shell $(GO) env GOPATH)/bin

.DEFAULT_GOAL := build

## build: compile the binary for the host platform
.PHONY: build
build:
	$(GO) build $(GOFLAGS) -o $(BINARY) .

## run: build then run against ./configs, forwarding ARGS (e.g. make run ARGS="search nmap")
.PHONY: run
run: build
	./$(BINARY) --config configs $(ARGS)

## install: install $(BINARY) to $(BINDIR) and every yaml in ./configs to $(CONFIGDIR)
.PHONY: install
install: build
	sudo mkdir -p $(DESTDIR)$(BINDIR)
	sudo cp $(BINARY) $(DESTDIR)$(BINDIR)/$(BINARY)
	sudo rm -rf $(DESTDIR)$(CONFIGDIR)

## uninstall: remove files placed by `make install`
.PHONY: uninstall
uninstall:
	sudo rm -f $(DESTDIR)$(BINDIR)/$(BINARY)
	sudo rm -rf $(DESTDIR)$(CONFIGDIR)

## test: run tests with race detector
.PHONY: test
test:
	$(GO) test -race ./...

## test-short: skip slow tests (useful for tight feedback loops)
.PHONY: test-short
test-short:
	$(GO) test -short ./...

## vet: go vet
.PHONY: vet
vet:
	$(GO) vet ./...

## fmt: gofmt all sources
.PHONY: fmt
fmt:
	$(GO) fmt ./...

## tidy: sync go.mod / go.sum
.PHONY: tidy
tidy:
	$(GO) mod tidy

## check: vet + tests (CI-equivalent gate)
.PHONY: check
check: vet test

## release: cross-compile every platform in $(PLATFORMS) into $(DIST)/
.PHONY: release
release: clean-dist
	@mkdir -p $(DIST)
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		out=$(DIST)/$(BINARY)-$$os-$$arch; \
		echo "  build $$os/$$arch -> $$out"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
		  $(GO) build $(GOFLAGS) -o $$out . || exit 1; \
	done
	@cp configs/*.yaml $(DIST)/
	@echo "release artifacts in $(DIST)/"

## clean: remove the host binary
.PHONY: clean
clean:
	rm -f $(BINARY)

## clean-dist: remove cross-compile artifacts
.PHONY: clean-dist
clean-dist:
	rm -rf $(DIST)

## clean-all: clean everything
.PHONY: clean-all
clean-all: clean clean-dist

## help: list targets
.PHONY: help
help:
	@awk 'BEGIN{FS=":.*##"; printf "Targets:\n"} /^## [a-zA-Z_-]+:/{sub(/^## /,""); split($$0,a,":"); printf "  %-14s %s\n", a[1], a[2]}' $(MAKEFILE_LIST)
