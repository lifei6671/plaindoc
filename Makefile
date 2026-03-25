SHELL := /usr/bin/env bash

ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
SERVER_DIR := $(ROOT_DIR)/apps/server
WEB_DIR := $(ROOT_DIR)/apps/web
RELEASE_DIR := $(ROOT_DIR)/release

# 允许通过 make 变量覆盖，例如：
# make server-dev ENV_FILE=apps/server/.env.local
ENV_FILE ?= apps/server/.env
SERVER_ENV := APP_ENV=development
APP_ADDR ?= :8080
WEB_ORIGIN ?= http://localhost:3001

SSR_WORKER_ENTRY_REL := apps/web/dist-ssr/worker-entry.js
SSR_WORKER_ENTRY_ABS := $(ROOT_DIR)/$(SSR_WORKER_ENTRY_REL)
SSR_WORKER_EXEC ?= node

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BUILD_VERSION ?= $(VERSION)
BUILD_COMMIT ?= $(shell git rev-parse --verify HEAD 2>/dev/null || echo unknown)
BUILD_TIME_UTC ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
BUILD_GO_VERSION ?= $(shell cd "$(SERVER_DIR)" && go env GOVERSION 2>/dev/null || echo unknown)
SERVER_LDFLAGS := -X github.com/lifei6671/plaindoc/apps/server/internal/buildinfo.Version=$(BUILD_VERSION) \
	-X github.com/lifei6671/plaindoc/apps/server/internal/buildinfo.CommitSHA=$(BUILD_COMMIT) \
	-X github.com/lifei6671/plaindoc/apps/server/internal/buildinfo.BuildTimeUTC=$(BUILD_TIME_UTC) \
	-X github.com/lifei6671/plaindoc/apps/server/internal/buildinfo.GoVersion=$(BUILD_GO_VERSION)
SERVER_BINARY := plaindoc-server-linux-amd64
SERVER_BINARY_PATH := $(RELEASE_DIR)/$(SERVER_BINARY)

.PHONY: help \
	check-go-tools check-web-tools check-node-exec check-env-file check-web-build check-web-ssr \
	install web-dev server-dev server-dev-ssr dev \
	web-build web-build-ssr server-build build \
	test-server test \
	package clean-release print-ssr-entry

help:
	@echo "PlainDoc Make Targets"
	@echo ""
	@echo "Development:"
	@echo "  make install          # install npm deps"
	@echo "  make web-dev          # run web dev server"
	@echo "  make server-dev       # run backend (default: SSR disabled)"
	@echo "  make server-dev-ssr   # run backend with SSR enabled (checks worker entry)"
	@echo "  make dev              # hint for starting full local dev"
	@echo ""
	@echo "Build:"
	@echo "  make web-build        # build web dist + dist-ssr"
	@echo "  make web-build-ssr    # build only dist-ssr"
	@echo "  make server-build     # build backend linux-amd64 binary"
	@echo "  make build            # web-build + server-build"
	@echo ""
	@echo "Test:"
	@echo "  make test-server      # go test ./... (apps/server)"
	@echo "  make test             # alias of test-server"
	@echo ""
	@echo "Package:"
	@echo "  make package          # create release artifacts under ./release"
	@echo "  make clean-release    # remove ./release"
	@echo ""
	@echo "Utilities:"
	@echo "  make print-ssr-entry  # print expected SSR worker entry path"
	@echo ""
	@echo "Variables:"
	@echo "  ENV_FILE=$(ENV_FILE)"
	@echo "  VERSION=$(VERSION)"
	@echo "  BUILD_VERSION=$(BUILD_VERSION)"
	@echo "  BUILD_COMMIT=$(BUILD_COMMIT)"
	@echo "  BUILD_TIME_UTC=$(BUILD_TIME_UTC)"
	@echo "  BUILD_GO_VERSION=$(BUILD_GO_VERSION)"

check-go-tools:
	@command -v go >/dev/null 2>&1 || { echo "go is required but not found"; exit 1; }

check-web-tools:
	@command -v npm >/dev/null 2>&1 || { echo "npm is required but not found"; exit 1; }

check-node-exec:
	@command -v $(SSR_WORKER_EXEC) >/dev/null 2>&1 || { echo "$(SSR_WORKER_EXEC) is required but not found"; exit 1; }

check-env-file:
	@if [ ! -f "$(ROOT_DIR)/$(ENV_FILE)" ]; then \
		echo "env file $(ENV_FILE) not found."; \
		echo "hint: cp apps/server/.env.example apps/server/.env"; \
		exit 1; \
	fi

check-web-build:
	@if [ ! -f "$(ROOT_DIR)/apps/web/dist/index.html" ]; then \
		echo "web dist is missing: apps/web/dist/index.html"; \
		echo "run: make web-build"; \
		exit 1; \
	fi

check-web-ssr:
	@if [ ! -f "$(SSR_WORKER_ENTRY_ABS)" ]; then \
		echo "SSR worker entry is missing: $(SSR_WORKER_ENTRY_REL)"; \
		echo "run: make web-build-ssr  (or make web-build)"; \
		exit 1; \
	fi

install: check-web-tools
	npm ci

web-dev: check-web-tools
	npm run web:dev

server-dev: check-go-tools
	@set -a; \
	if [ -f "$(ROOT_DIR)/$(ENV_FILE)" ]; then echo "load env from $(ENV_FILE)"; fi; \
	[ -f "$(ROOT_DIR)/$(ENV_FILE)" ] && . "$(ROOT_DIR)/$(ENV_FILE)"; \
	set +a; \
	cd "$(SERVER_DIR)"; \
	$(SERVER_ENV) APP_ADDR="$(APP_ADDR)" WEB_ORIGIN="$(WEB_ORIGIN)" SSR_WORKER_ENABLED=false go run ./cmd/server

server-dev-ssr: check-go-tools check-node-exec check-web-ssr
	@set -a; \
	if [ -f "$(ROOT_DIR)/$(ENV_FILE)" ]; then echo "load env from $(ENV_FILE)"; fi; \
	[ -f "$(ROOT_DIR)/$(ENV_FILE)" ] && . "$(ROOT_DIR)/$(ENV_FILE)"; \
	set +a; \
	cd "$(SERVER_DIR)"; \
	$(SERVER_ENV) APP_ADDR="$(APP_ADDR)" WEB_ORIGIN="$(WEB_ORIGIN)" \
	SSR_WORKER_ENABLED=true SSR_WORKER_EXEC="$(SSR_WORKER_EXEC)" SSR_WORKER_ENTRY="$(SSR_WORKER_ENTRY_ABS)" \
	go run ./cmd/server

dev:
	@echo "Start dev in two terminals:"
	@echo "  Terminal 1: make server-dev"
	@echo "  Terminal 2: make web-dev"
	@echo ""
	@echo "If you need reader SSR in local backend:"
	@echo "  make web-build-ssr && make server-dev-ssr"

web-build: check-web-tools
	VITE_BUILD_VERSION="$(BUILD_VERSION)" npm run web:build

web-build-ssr: check-web-tools
	VITE_BUILD_VERSION="$(BUILD_VERSION)" npm run web:build-ssr

server-build: check-go-tools
	@mkdir -p "$(RELEASE_DIR)"
	cd "$(SERVER_DIR)" && go mod download
	cd "$(SERVER_DIR)" && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(SERVER_LDFLAGS)" -o "$(SERVER_BINARY_PATH)" ./cmd/server
	@echo "built: $(SERVER_BINARY_PATH)"

build: web-build server-build

test-server:
	cd "$(SERVER_DIR)" && go test ./... -count=1

test: test-server

package: build check-web-build check-web-ssr
	@mkdir -p "$(RELEASE_DIR)"
	@printf "version=%s\ncommit_sha=%s\nbuild_time_utc=%s\ngo_version=%s\n" \
		"$(BUILD_VERSION)" \
		"$(BUILD_COMMIT)" \
		"$(BUILD_TIME_UTC)" \
		"$(BUILD_GO_VERSION)" \
		> "$(RELEASE_DIR)/build-metadata-$(VERSION).txt"
	tar -C "$(WEB_DIR)" -czf "$(RELEASE_DIR)/plaindoc-web-$(VERSION).tar.gz" dist dist-ssr
	@rm -rf "$(RELEASE_DIR)/plaindoc-bundle"
	@mkdir -p "$(RELEASE_DIR)/plaindoc-bundle/apps/web"
	cp "$(SERVER_BINARY_PATH)" "$(RELEASE_DIR)/plaindoc-bundle/$(SERVER_BINARY)"
	cp "$(RELEASE_DIR)/build-metadata-$(VERSION).txt" "$(RELEASE_DIR)/plaindoc-bundle/build-metadata.txt"
	cp -R "$(WEB_DIR)/dist" "$(RELEASE_DIR)/plaindoc-bundle/apps/web/"
	cp -R "$(WEB_DIR)/dist-ssr" "$(RELEASE_DIR)/plaindoc-bundle/apps/web/"
	tar -czf "$(RELEASE_DIR)/plaindoc-server-linux-amd64-$(VERSION).tar.gz" \
		-C "$(RELEASE_DIR)/plaindoc-bundle" \
		"$(SERVER_BINARY)" \
		build-metadata.txt \
		apps/web/dist \
		apps/web/dist-ssr
	sha256sum \
		"$(SERVER_BINARY_PATH)" \
		"$(RELEASE_DIR)/plaindoc-server-linux-amd64-$(VERSION).tar.gz" \
		"$(RELEASE_DIR)/plaindoc-web-$(VERSION).tar.gz" \
		"$(RELEASE_DIR)/build-metadata-$(VERSION).txt" \
		> "$(RELEASE_DIR)/checksums-$(VERSION).txt"
	@echo "release artifacts generated under $(RELEASE_DIR)"

clean-release:
	rm -rf "$(RELEASE_DIR)"

print-ssr-entry:
	@echo "$(SSR_WORKER_ENTRY_ABS)"
