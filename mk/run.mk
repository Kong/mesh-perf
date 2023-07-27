KUMACTLBIN = $(TOP)/build/kong-mesh-$(MESH_VERSION)/bin/kumactl

E2E_ENV_VARS += K8SCLUSTERS="mesh-perf"
E2E_ENV_VARS += KUMA_K8S_TYPE=k3d
E2E_ENV_VARS += E2E_CONFIG_FILE="$(TOP)/test/cfg.yaml"
E2E_ENV_VARS += KUMACTLBIN="$(KUMACTLBIN)"
E2E_ENV_VARS += MESH_VERSION=$(MESH_VERSION)

.PHONY:
fetch-mesh:
	mkdir -p build
	[ -f $(KUMACTLBIN) ] || (cd build && curl -L https://docs.konghq.com/mesh/installer.sh | VERSION=$(MESH_VERSION) sh -)

.PHONY: run
run: export PERF_TEST_NUM_SERVICES ?= 5
run: export PERF_TEST_STABILIZATION_SLEEP ?= 10s
run: fetch-mesh
	$(E2E_ENV_VARS) $(GINKGO) --json-report=raw-report.json ./test/...
	jq '{Parameters: env | with_entries(select(.key | startswith("PERF_TEST"))), Suites: .}' raw-report.json > report.json
