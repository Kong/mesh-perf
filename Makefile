TOP = $(shell pwd)
MESH_VERSION ?= 2.3.0

include mk/dev.mk
include mk/check.mk
include mk/run.mk
include mk/infrastructure.mk