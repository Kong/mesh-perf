GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

ifeq (,$(shell which mise))
$(error "mise - https://github.com/jdx/mise - not found. Please install it.")
endif
MISE := $(shell which mise)

GINKGO=$(shell $(MISE) which ginkgo)
GO=$(shell $(MISE) which go)
GOLANGCI_LINT=$(shell $(MISE) which golangci-lint)
JSONNET=$(shell $(MISE) which jsonnet)
JSONNET_BUNDLER=$(shell $(MISE) which jb)
TERRAFORM=$(shell $(MISE) which terraform)
