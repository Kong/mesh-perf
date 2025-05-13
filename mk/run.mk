KUMACTLBIN = $(TOP)/build/kong-mesh-$(PERF_TEST_MESH_VERSION)/bin/kumactl

E2E_ENV_VARS += K8SCLUSTERS="mesh-perf"
E2E_ENV_VARS += KUMA_K8S_TYPE=k3d
E2E_ENV_VARS += TEST_ROOT="$(TOP)"
E2E_ENV_VARS += KUMACTLBIN="$(KUMACTLBIN)"
E2E_ENV_VARS += PERF_TEST_MESH_VERSION=$(PERF_TEST_MESH_VERSION)
E2E_ENV_VARS += PERF_LIMIT_MEGA_MEMORY=$${PERF_LIMIT_MEGA_MEMORY:=400}
E2E_ENV_VARS += PERF_LIMIT_MILLI_CPU=$${PERF_LIMIT_MILLI_CPU:=1000}
E2E_ENV_VARS += PERF_TEST_NUM_SERVICES=$${PERF_TEST_NUM_SERVICES:=70}
E2E_ENV_VARS += PERF_TEST_INSTANCES_PER_SERVICE=$${PERF_TEST_INSTANCES_PER_SERVICE:=2}
E2E_ENV_VARS += PERF_TEST_STABILIZATION_SLEEP=$${PERF_TEST_STABILIZATION_SLEEP:=30s}
E2E_ENV_VARS += CONTAINER_REGISTRY=$(CONTAINER_REGISTRY)

.PHONY: fetch-mesh
fetch-mesh:
	@if [ -z "$(PERF_TEST_MESH_VERSION)" ]; then echo "PERF_TEST_MESH_VERSION must be defined"; exit 1; fi
	mkdir -p build
	[ -f $(KUMACTLBIN) ] || (cd build && curl -L https://docs.konghq.com/mesh/installer.sh | VERSION=$(PERF_TEST_MESH_VERSION) sh -)

test-runs:
	mkdir -p test-runs

.PHONY: run
run: fetch-mesh | test-runs
	$(E2E_ENV_VARS) $(GINKGO) --timeout=4h --label-filter="!limits" --json-report=raw-report.json -v ./test/... 2>&1;

.PHONY: run/limits
run/limits: fetch-mesh
	$(E2E_ENV_VARS) $(GINKGO) -v --timeout=4h --label-filter="limits" --json-report=raw-report.json ./test/...
