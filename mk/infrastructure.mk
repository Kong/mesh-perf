.PHONY: start-cluster
start-cluster:
	$(TERRAFORM) -chdir=$(TOP)/infrastructure/$(ENV) init && $(TERRAFORM) -chdir=$(TOP)/infrastructure/$(ENV) apply -auto-approve

.PHONY: destroy-cluster
destroy-cluster:
	$(TERRAFORM) -chdir=$(TOP)/infrastructure/$(ENV) destroy -auto-approve