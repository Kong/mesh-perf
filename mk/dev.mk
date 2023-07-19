TOOLS_DIR = $(TOP)/tools
TOOLS_DEPS_DIRS=$(TOP)/mk/dependencies
TOOLS_DEPS_LOCK_FILE=mk/dependencies/deps.lock
TOOLS_MAKEFILE=$(TOP)/mk/dev.mk
CI_TOOLS_DIR ?= ${HOME}/.mesh-perf-dev
ifdef XDG_DATA_HOME
	CI_TOOLS_DIR := ${XDG_DATA_HOME}/mesh-perf-dev
endif
CI_TOOLS_BIN_DIR=$(CI_TOOLS_DIR)/bin

GINKGO=$(CI_TOOLS_BIN_DIR)/ginkgo

GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

TERRAFORM=$(CI_TOOLS_BIN_DIR)/terraform
JSONNET=$(CI_TOOLS_BIN_DIR)/jsonnet
JSONNET_BUNDLER=$(CI_TOOLS_BIN_DIR)/jb

.PHONY: dev/tools/ginkgo
dev/tools/ginkgo:
	GOBIN=${CI_TOOLS_BIN_DIR} go install github.com/onsi/ginkgo/v2/ginkgo@$$(go list -f '{{.Version}}' -m github.com/onsi/ginkgo/v2)

.PHONY: dev/tools
dev/tools: dev/tools/ginkgo
	$(TOOLS_DIR)/dev/install-dev-tools.sh $(CI_TOOLS_BIN_DIR) $(CI_TOOLS_DIR) "$(TOOLS_DEPS_DIRS)" $(TOOLS_DEPS_LOCK_FILE) $(GOOS) $(GOARCH) $(TOOLS_MAKEFILE)
