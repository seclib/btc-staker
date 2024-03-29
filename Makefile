DOCKER = $(shell which docker)
BUILDDIR ?= $(CURDIR)/build
TOOLS_DIR := tools

BTCD_PKG := github.com/btcsuite/btcd
BTCDW_PKG := github.com/btcsuite/btcwallet
BABYLON_PKG := github.com/babylonchain/babylon/cmd/babylond

GO_BIN := ${GOPATH}/bin
BTCD_BIN := $(GO_BIN)/btcd

ldflags := $(LDFLAGS)
build_tags := $(BUILD_TAGS)
build_args := $(BUILD_ARGS)

PACKAGES_E2E=$(shell go list ./... | grep '/itest')

ifeq ($(LINK_STATICALLY),true)
	ldflags += -linkmode=external -extldflags "-Wl,-z,muldefs -static" -v
endif

ifeq ($(VERBOSE),true)
	build_args += -v
endif

BUILD_TARGETS := build install
BUILD_FLAGS := --tags "$(build_tags)" --ldflags '$(ldflags)'

all: build install

build: BUILD_ARGS := $(build_args) -o $(BUILDDIR)

$(BUILD_TARGETS): go.sum $(BUILDDIR)/
	CGO_CFLAGS="-O -D__BLST_PORTABLE__" go $@ -mod=readonly $(BUILD_FLAGS) $(BUILD_ARGS) ./...

$(BUILDDIR)/:
	mkdir -p $(BUILDDIR)/

build-docker:
	$(DOCKER) build --tag babylonchain/btc-staker -f Dockerfile \
		$(shell git rev-parse --show-toplevel)

.PHONY: build build-docker

test:
	go test ./...

test-e2e:
	cd $(TOOLS_DIR); go install -trimpath $(BTCD_PKG); go install -trimpath $(BTCDW_PKG); go install -trimpath $(BABYLON_PKG);
	go test -mod=readonly -timeout=25m -v $(PACKAGES_E2E) -count=1 --tags=e2e

proto-gen:
	@$(call print, "Compiling protos.")
	cd ./proto; ./gen_protos_docker.sh

.PHONY: proto-gen
