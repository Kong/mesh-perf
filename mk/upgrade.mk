KUMA_DIR = ../kuma

UNAME := $(shell uname)
SED_INLINE := sed -i

ifeq ($(UNAME), Darwin)
	SED_INLINE := sed -i ''
endif

.PHONY: upgrade-dashboards
upgrade-dashboards:
	cp $(KUMA_DIR)/app/kumactl/data/install/k8s/metrics/grafana/kuma-cp.json tools/observability/grafana/provisioning/dashboards/kuma-cp.json
	$(SED_INLINE) 's/$${DS_PROMETHEUS}/Prometheus/' tools/observability/grafana/provisioning/dashboards/kuma-cp.json
