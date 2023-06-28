CI_TOOLS_DIR ?= ${HOME}/.mesh-perf-dev
ifdef XDG_DATA_HOME
	CI_TOOLS_DIR := ${XDG_DATA_HOME}/mesh-perf-dev
endif
CI_TOOLS_BIN_DIR=$(CI_TOOLS_DIR)/bin

GINKGO=$(CI_TOOLS_BIN_DIR)/ginkgo

.PHONY: dev/tools/ginkgo
dev/tools/ginkgo:
	GOBIN=${CI_TOOLS_BIN_DIR} go install github.com/onsi/ginkgo/v2/ginkgo@$$(go list -f '{{.Version}}' -m github.com/onsi/ginkgo/v2)

.PHONY: dev/tools
dev/tools: dev/tools/ginkgo
