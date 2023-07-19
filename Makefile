TOP = $(shell pwd)

# Possible to use Kong Mesh version in the following format 0.0.0-preview.v964544ae9
MESH_VERSION ?= 0.0.0-preview.vaa5a34907

include mk/dev.mk
include mk/check.mk
include mk/run.mk
include mk/upgrade.mk
include mk/infrastructure.mk
include mk/generate.mk
include mk/grafana.mk
