DASHBOARDS_DIR=tools/observability/grafana/provisioning/dashboards
JSONNET_BUNDLER_CACHE_DIR=build/jsonnet

dashboards = $(foreach dir,$(shell find $(DASHBOARDS_DIR) -maxdepth 1 -mindepth 1 -type d | sort),$(notdir $(dir)))

$(JSONNET_BUNDLER_CACHE_DIR):
	$(JSONNET_BUNDLER) update --jsonnetpkg-home=$(JSONNET_BUNDLER_CACHE_DIR)

generate-grafana-%:
	$(JSONNET) -J $(JSONNET_BUNDLER_CACHE_DIR) $(DASHBOARDS_DIR)/$*/main.libsonnet -o $(DASHBOARDS_DIR)/$*.json

.PHONY: generate-grafana
generate-grafana: $(JSONNET_BUNDLER_CACHE_DIR) $(addprefix generate-grafana-,$(dashboards))

.PHONY: generate
generate: generate-grafana

clean-%: 
	rm $(DASHBOARDS_DIR)/$*.json

clean-$(JSONNET_BUNDLER_CACHE_DIR):
	rm -rf $(JSONNET_BUNDLER_CACHE_DIR)

.PHONY: clean
clean: clean-$(JSONNET_BUNDLER_CACHE_DIR) $(addprefix clean-,$(dashboards))
