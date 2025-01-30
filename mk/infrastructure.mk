CI ?= false
TERRAFORM_DIR=$(TOP)/infrastructure/$(ENV)
TERRAFORM_ECR_DIR=$(TERRAFORM_DIR)/ecr

ifeq ($(ENV),eks)
	EKS_NUM_OF_NODES ?= 3
	TERRAFORM_VARS_EKS += -var="ci=$(CI)"
	TERRAFORM_VARS_EKS += -var="nodes_number=$(EKS_NUM_OF_NODES)"
	ALTERNATIVE_CONTAINER_REGISTRY ?= $(shell $(MAKE) --silent ecr-get-registry)
	CREATE_CLUSTER_DEPS_TARGET += start-ecr
	DESTROY_CLUSTER_DEPS_TARGET += destroy-ecr
endif

.PHONY: number-of-nodes
number-of-nodes:
	@$(E2E_ENV_VARS) go run tools/eksformula/main.go

.PHONY: start-cluster
start-cluster: create-cluster $(CREATE_CLUSTER_DEPS_TARGET)

.PHONY: create-cluster
create-cluster:
	$(TERRAFORM) -chdir=$(TERRAFORM_DIR) init && \
	$(TERRAFORM) -chdir=$(TERRAFORM_DIR) apply -auto-approve $(TERRAFORM_VARS_EKS)

.PHONY: create-ecr
create-ecr:
	$(TERRAFORM) -chdir=$(TERRAFORM_ECR_DIR) init && \
	$(TERRAFORM) -chdir=$(TERRAFORM_ECR_DIR) apply -auto-approve

.PHONY: start-ecr
start-ecr: create-ecr
	export AWS_REGION=$(shell $(MAKE) --silent ecr-get-region)
	export REGISTRY=$(shell $(MAKE) --silent ecr-get-registry)
	$(MAKE) ecr-push

.PHONY: destroy-cluster
destroy-cluster: $(DESTROY_CLUSTER_DEPS_TARGET)
	$(TERRAFORM) -chdir=$(TERRAFORM_DIR) destroy -auto-approve $(TERRAFORM_VARS_EKS)

.PHONY: destroy-ecr
destroy-ecr:
	$(TERRAFORM) -chdir=$(TERRAFORM_ECR_DIR) destroy -auto-approve

.PHONY: ecr-get-registry
ecr-get-registry:
	@$(TERRAFORM) -chdir=$(TERRAFORM_ECR_DIR) output -json | jq --raw-output '.ecr_registry.value // ""'

.PHONY: ecr-get-region
ecr-get-region:
	@$(TERRAFORM) -chdir=$(TERRAFORM_ECR_DIR) output -raw region

.PHONY: ecr-push
ecr-push: ecr-authenticate ecr-push-kuma-dp ecr-push-fake-service

.PHONY: ecr-authenticate
ecr-authenticate:
	aws ecr get-login-password --region $(AWS_REGION) | docker login $(REGISTRY) --username AWS --password-stdin

.PHONY: ecr-push-kuma-dp
ecr-push-kuma-dp:
	@if [[ -z "$(PERF_TEST_MESH_VERSION)" ]]; then echo "PERF_TEST_MESH_VERSION must be defined"; exit 1; fi
	docker pull kong/kuma-dp:$(PERF_TEST_MESH_VERSION) --platform linux/arm64 && \
	docker tag kong/kuma-dp:$(PERF_TEST_MESH_VERSION) $(REGISTRY)/kuma-dp:$(PERF_TEST_MESH_VERSION) && \
	docker push $(REGISTRY)/kuma-dp:$(PERF_TEST_MESH_VERSION)

.PHONY: ecr-push-fake-service
ecr-push-fake-service:
	docker pull nicholasjackson/fake-service:v0.26.0 --platform linux/arm64 && \
	docker tag nicholasjackson/fake-service:v0.26.0 $(REGISTRY)/fake-service:v0.26.0 && \
	docker push $(REGISTRY)/fake-service:v0.26.0
