TERRAFORM_DIR=$(TOP)/infrastructure/$(ENV)

ifeq ($(ENV),eks)
TERRAFORM_VARS += -var="nodes_number=$${EKS_NUM_OF_NODES:=3}"
endif

.PHONY: number-of-nodes
number-of-nodes:
	@$(E2E_ENV_VARS) go run tools/eksformula/main.go

.PHONY: start-cluster
start-cluster:
	$(TERRAFORM) -chdir=$(TERRAFORM_DIR) init && \
	$(TERRAFORM) -chdir=$(TERRAFORM_DIR) apply -auto-approve $(TERRAFORM_VARS)

.PHONY: destroy-cluster
destroy-cluster:
	$(TERRAFORM) -chdir=$(TERRAFORM_DIR) destroy -auto-approve $(TERRAFORM_VARS)
