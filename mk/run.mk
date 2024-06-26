KUMACTLBIN = $(TOP)/build/kong-mesh-$(PERF_TEST_MESH_VERSION)/bin/kumactl

E2E_ENV_VARS += K8SCLUSTERS="mesh-perf"
E2E_ENV_VARS += KUMA_K8S_TYPE=k3d
E2E_ENV_VARS += TEST_ROOT="$(TOP)"
E2E_ENV_VARS += E2E_CONFIG_FILE="$(TOP)/test/cfg.yaml"
E2E_ENV_VARS += KUMACTLBIN="$(KUMACTLBIN)"
E2E_ENV_VARS += PERF_TEST_MESH_VERSION=$(PERF_TEST_MESH_VERSION)
E2E_ENV_VARS += PERF_TEST_NUM_SERVICES=$${PERF_TEST_NUM_SERVICES:=5}
E2E_ENV_VARS += PERF_TEST_INSTANCES_PER_SERVICE=$${PERF_TEST_INSTANCES_PER_SERVICE:=1}
E2E_ENV_VARS += PERF_TEST_STABILIZATION_SLEEP=$${PERF_TEST_STABILIZATION_SLEEP:=10s}

.PHONY: fetch-mesh
fetch-mesh:
	@if [[ -z "$(PERF_TEST_MESH_VERSION)" ]]; then echo "PERF_TEST_MESH_VERSION must be defined"; exit 1; fi
	mkdir -p build
	[ -f $(KUMACTLBIN) ] || (cd build && curl -L https://docs.konghq.com/mesh/installer.sh | VERSION=$(PERF_TEST_MESH_VERSION) sh -)

.PHONY: run
run: fetch-mesh
	$(E2E_ENV_VARS) $(GINKGO) -v --timeout=4h --json-report=raw-report.json ./test/...
