BIN_DIR := bin
APP := cx-mpicheck
exec_prefix ?= /usr/local
bindir ?=
GOOS ?= $(shell go env GOOS)
GOPATH ?= $(shell go env GOPATH)
EXE :=
ifeq ($(GOOS),windows)
EXE := .exe
endif
DEFAULT_BINDIR := $(if $(filter windows,$(GOOS)),$(GOPATH)/bin,$(exec_prefix)/bin)
INSTALL_BINDIR := $(if $(bindir),$(bindir),$(DEFAULT_BINDIR))
VERSION := $(shell awk -F '\"' '/^var version =/ {print $$2}' cmd/cx-mpicheck/version.go 2>/dev/null)

.PHONY: all build clean verify install test

default: build

test:
	@go test ./...

build: test
	@mkdir -p $(BIN_DIR)
	@go mod download
	@./scripts/update-version.sh >/dev/null 2>&1 || true
	go build -o $(BIN_DIR)/$(APP)$(EXE) ./cmd/cx-mpicheck

clean:
	@rm -rf $(BIN_DIR)

all: clean test
	@mkdir -p $(BIN_DIR)
	@go mod download
	@./scripts/update-version.sh >/dev/null 2>&1 || true
	GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/$(APP)_$(VERSION)_linux-x64 ./cmd/cx-mpicheck
	GOOS=linux GOARCH=arm64 go build -o $(BIN_DIR)/$(APP)_$(VERSION)_linux-arm64 ./cmd/cx-mpicheck
	GOOS=darwin GOARCH=amd64 go build -o $(BIN_DIR)/$(APP)_$(VERSION)_darwin-x64 ./cmd/cx-mpicheck
	GOOS=darwin GOARCH=arm64 go build -o $(BIN_DIR)/$(APP)_$(VERSION)_darwin-arm64 ./cmd/cx-mpicheck
	GOOS=windows GOARCH=amd64 go build -o $(BIN_DIR)/$(APP)_$(VERSION)_win-x64.exe ./cmd/cx-mpicheck
	GOOS=windows GOARCH=arm64 go build -o $(BIN_DIR)/$(APP)_$(VERSION)_win-arm64.exe ./cmd/cx-mpicheck
	$(MAKE) verify

verify:
	@mkdir -p $(BIN_DIR)
	@rm -f $(BIN_DIR)/SHA256SUMS
	@for f in $(BIN_DIR)/*; do \
		[ -f $$f ] || continue; \
		if command -v sha256sum >/dev/null 2>&1; then \
			sha256sum $$f >> $(BIN_DIR)/SHA256SUMS; \
		else \
			shasum -a 256 $$f >> $(BIN_DIR)/SHA256SUMS; \
		fi; \
	done

install: build
	@mkdir -p $(INSTALL_BINDIR)
	@cp $(BIN_DIR)/$(APP)$(EXE) $(INSTALL_BINDIR)/$(APP)$(EXE)
