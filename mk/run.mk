KUMACTLBIN = $(TOP)/build/kong-mesh-$(MESH_VERSION)/bin/kumactl

E2E_ENV_VARS += K8SCLUSTERS="mesh-perf"
E2E_ENV_VARS += KUMA_K8S_TYPE=k3d
E2E_ENV_VARS += E2E_CONFIG_FILE="$(TOP)/test/cfg.yaml"
E2E_ENV_VARS += KUMACTLBIN="$(KUMACTLBIN)"
E2E_ENV_VARS += MESH_VERSION=$(MESH_VERSION)
E2E_ENV_VARS += TEST_NUM_SERVICES=$(TEST_NUM_SERVICES)
E2E_ENV_VARS += TEST_INSTANCES_PER_SERVICE=$(TEST_INSTANCES_PER_SERVICE)

.PHONY:
fetch-mesh:
	mkdir -p build
	[ -f $(KUMACTLBIN) ] || (cd build && curl -L https://docs.konghq.com/mesh/installer.sh | VERSION=$(MESH_VERSION) sh -)

.PHONY: run
run: fetch-mesh
	$(E2E_ENV_VARS) $(GINKGO) ./test/...
