TERRAFORM_DIR=$(TOP)/infrastructure/$(ENV)
CREATE_CLUSTER_DEPS_TARGET ?=
DESTROY_CLUSTER_DEPS_TARGET ?=
TERRAFORM_ECR_DIR ?=
ifeq ($(ENV),eks)
TERRAFORM_VARS += -var="nodes_number=$${EKS_NUM_OF_NODES:=3}"
TERRAFORM_ECR_DIR=$(TOP)/infrastructure/$(ENV)/ecr
CREATE_CLUSTER_DEPS_TARGET += create-ecr
DESTROY_CLUSTER_DEPS_TARGET += destroy-ecr
endif

# If specified, then containers that have a pull number dependent on scale (kuma-dp, fake-service)
# will be downloaded from this registry
ALTERNATIVE_CONTAINER_REGISTRY:=$(ALTERNATIVE_CONTAINER_REGISTRY)

.PHONY: number-of-nodes
number-of-nodes:
	@$(E2E_ENV_VARS) go run tools/eksformula/main.go

.PHONY: start-cluster
start-cluster: create-cluster $(CREATE_CLUSTER_DEPS_TARGET)

.PHONY: create-cluster
create-cluster:
	$(TERRAFORM) -chdir=$(TERRAFORM_DIR) init && \
	$(TERRAFORM) -chdir=$(TERRAFORM_DIR) apply -auto-approve $(TERRAFORM_VARS)

.PHONY: create-ecr
create-ecr: 
	$(TERRAFORM) -chdir=$(TERRAFORM_ECR_DIR) init && \
	$(TERRAFORM) -chdir=$(TERRAFORM_ECR_DIR) apply -auto-approve

.PHONY: destroy-cluster
destroy-cluster: $(DESTROY_CLUSTER_DEPS_TARGET)
	$(TERRAFORM) -chdir=$(TERRAFORM_DIR) destroy -auto-approve $(TERRAFORM_VARS)

.PHONY: destroy-ecr
destroy-ecr:
	$(TERRAFORM) -chdir=$(TERRAFORM_ECR_DIR) destroy -auto-approve

.PHONY: ecr-get-registry
ecr-get-registry:
	@$(TERRAFORM) -chdir=$(TERRAFORM_ECR_DIR) output -raw ecr_registry

.PHONY: ecr-get-region
ecr-get-region:
	@$(TERRAFORM) -chdir=$(TERRAFORM_ECR_DIR) output -raw region

.PHONY: ecr-push
ecr-push: ecr-authenticate ecr-push-kuma-dp ecr-push-fake-service

.PHONY: ecr-authenticate
ecr-authenticate:
	aws ecr get-login-password --region $(AWS_REGION) | docker login $(ALTERNATIVE_CONTAINER_REGISTRY) --username AWS --password-stdin

.PHONY: ecr-push-kuma-dp
ecr-push-kuma-dp:
	@if [[ -z "$(PERF_TEST_MESH_VERSION)" ]]; then echo "PERF_TEST_MESH_VERSION must be defined"; exit 1; fi
	docker pull kong/kuma-dp:$(PERF_TEST_MESH_VERSION) --platform linux/arm64 && \
	docker tag kong/kuma-dp:$(PERF_TEST_MESH_VERSION) $(ALTERNATIVE_CONTAINER_REGISTRY)/kuma-dp:$(PERF_TEST_MESH_VERSION) && \
	docker push $(ALTERNATIVE_CONTAINER_REGISTRY)/kuma-dp:$(PERF_TEST_MESH_VERSION)

.PHONY: ecr-push-fake-service
ecr-push-fake-service:
	docker pull nicholasjackson/fake-service:v0.25.2 --platform linux/arm64 && \
	docker tag nicholasjackson/fake-service:v0.25.2 $(ALTERNATIVE_CONTAINER_REGISTRY)/fake-service:v0.25.2 && \
	docker push $(ALTERNATIVE_CONTAINER_REGISTRY)/fake-service:v0.25.2
