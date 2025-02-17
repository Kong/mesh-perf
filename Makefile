TOP = $(shell pwd)

include mk/dev.mk
include mk/check.mk
include mk/upgrade.mk
include mk/infrastructure.mk
include mk/run.mk
include mk/generate.mk
include mk/grafana.mk
