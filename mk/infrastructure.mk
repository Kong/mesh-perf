TERRAFORM_DIR=$(TOP)/infrastructure/$(ENV)

ifeq ($(ENV),eks)
TERRAFORM_VARS += -var="nodes_number=$${EKS_NUM_OF_NODES:=3}"
endif

# If specified, then containers that have a pull number dependent on scale (kuma-dp, fake-service)
# will be downloaded from this registry
ALTERNATIVE_CONTAINER_REGISTRY:=$(ALTERNATIVE_CONTAINER_REGISTRY)

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

.PHONY: ecr-get-registry
ecr-get-registry:
	@$(TERRAFORM) -chdir=$(TERRAFORM_DIR) output ecr_registry

.PHONY: ecr-get-region
ecr-get-region:
	@$(TERRAFORM) -chdir=$(TERRAFORM_DIR) output region

.PHONY: ecr-push
ecr-push: ecr-authenticate ecr-push-kuma-dp ecr-push-fake-service

.PHONY: ecr-authenticate
ecr-authenticate:
	aws ecr get-login-password --region $(AWS_REGION) | docker login $(ALTERNATIVE_CONTAINER_REGISTRY) --username AWS --password-stdin

.PHONY: ecr-push-kuma-dp
ecr-push-kuma-dp:
	docker pull kong/kuma-dp:$(PERF_TEST_MESH_VERSION) && \
	docker tag kong/kuma-dp:$(PERF_TEST_MESH_VERSION) $(ALTERNATIVE_CONTAINER_REGISTRY)/kuma-dp:$(PERF_TEST_MESH_VERSION) && \
	docker push $(ALTERNATIVE_CONTAINER_REGISTRY)/kuma-dp:$(PERF_TEST_MESH_VERSION)

.PHONY: ecr-push-fake-service
ecr-push-fake-service:
	docker pull nicholasjackson/fake-service:v0.25.2 && \
	docker tag nicholasjackson/fake-service:v0.25.2 $(ALTERNATIVE_CONTAINER_REGISTRY)/fake-service:v0.25.2 && \
	docker push $(ALTERNATIVE_CONTAINER_REGISTRY)/fake-service:v0.25.2
