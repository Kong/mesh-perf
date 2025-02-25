TF_CMD = $(TERRAFORM) -chdir=$(DIR)

DEBUG := $(or $(DEBUG),$(or $(RUNNER_DEBUG),false))
# If DEBUG is true, force AWS_REGION=us-west-2 (to create a manager prometheus workspace,
# which is unavailable in us-west-1). Otherwise, use the user-provided AWS_REGION or default to us-west-1.
AWS_REGION := $(if $(filter true,$(DEBUG)),us-west-2,$(or $(AWS_REGION),us-west-1))

CONTAINER_REGISTRY = $(shell $(TERRAFORM) -chdir=$(ecr_DIR) output -json | jq --raw-output '.registry.value // ""')

ENV_VARS += -var="ci=$(or $(CI),false)"
ENV_VARS += -var="debug=$(DEBUG)"
vpc_ENV_VARS += -var="region=$(AWS_REGION)"
vpc_ENV_VARS += -var='availability_zones=["$(AWS_REGION)b", "$(AWS_REGION)c"]'
eks_ENV_VARS += -var="nodes_number=$(shell $(E2E_ENV_VARS) go run tools/eksformula/main.go)"

# Define top-level directories relative to the TOP directory.
DIR_INFRASTRUCTURE := $(shell realpath --relative-to=$(TOP) $(TOP)/infrastructure)
DIR_AWS := $(shell realpath --relative-to=$(TOP) $(DIR_INFRASTRUCTURE)/aws)

# Define directories for each AWS component for portability.
vpc_DIR := $(shell realpath --relative-to=$(TOP) $(DIR_AWS)/vpc)
ecr_DIR := $(shell realpath --relative-to=$(TOP) $(DIR_AWS)/ecr)
eks_DIR := $(shell realpath --relative-to=$(TOP) $(DIR_AWS)/eks)
monitoring_DIR := $(shell realpath --relative-to=$(TOP) $(DIR_AWS)/monitoring)

# List AWS components to easily add or remove items from the build process.
COMPONENTS := vpc ecr eks monitoring

# Macro to generate 'apply' and 'destroy' targets for each AWS component.
# $(1) is the component name (vpc, ecr, eks, monitoring).
# The 'apply' target conditionally runs an initialization step (if INIT is not "false")
# and then calls the Terraform apply command.
# The 'destroy' target calls the Terraform destroy command.
# For each component, we depend on e.g. terraform/apply/infrastructure/aws/vpc
# which will match the pattern terraform/apply/%.
define MAKE_AWS_TARGETS
.PHONY: aws/apply/$(1) aws/destroy/$(1)
aws/apply/$(1): $(if $(filter-out false,$(INIT)),terraform/init/$($(1)_DIR)) terraform/apply/$($(1)_DIR)
aws/destroy/$(1): terraform/destroy/$($(1)_DIR)
endef

# Generate targets for each AWS component.
$(foreach comp, $(COMPONENTS), $(eval $(call MAKE_AWS_TARGETS,$(comp))))

# Terraform initialization: sets DIR to the target stem and runs init with optional flags.
.PHONY: terraform/init/%
terraform/init/%: DIR=$*
terraform/init/%:
	$(TF_CMD) init$(if $(UPGRADE), -upgrade,)$(if $(RECONFIGURE), -reconfigure,)

# Generic rule for both apply and destroy.
# It extracts the command (apply or destroy) from the target name, sets the working directory,
# injects any extra variables via $(VARS), and appends -auto-approve if AUTO_APPROVE isn’t "false".
.PHONY: terraform/apply/% terraform/destroy/%
## For certain directories, assign target-specific variable values for Terraform.
terraform/apply/$(vpc_DIR) terraform/destroy/$(vpc_DIR): VARS = $(vpc_ENV_VARS)
terraform/apply/$(eks_DIR) terraform/destroy/$(eks_DIR): VARS = $(ENV_VARS) $(eks_ENV_VARS)
terraform/apply/$(monitoring_DIR) terraform/destroy/$(monitoring_DIR): VARS = $(ENV_VARS)
terraform/apply/% terraform/destroy/%:
	$(TERRAFORM) -chdir=$* $(word 2,$(subst /, ,$@)) $(if $(VARS),$(VARS))$(if $(filter false,$(AUTO_APPROVE)),, -auto-approve)

.PHONY: aws/destroy
aws/destroy: aws/destroy/ecr aws/destroy/eks aws/destroy/vpc

.PHONY: aws/create
aws/create: aws/apply/vpc aws/apply/eks aws/apply/ecr ecr/push

#ecr/%: export CONTAINER_REGISTRY=$(CONTAINER_REGISTRY)

.PHONY: ecr/authenticate
ecr/authenticate:
	aws ecr get-login-password --region $(AWS_REGION) | docker login $(CONTAINER_REGISTRY) --username AWS --password-stdin

.PHONY: check-perf-test-mesh-version
check-perf-test-mesh-version:
	@if [[ -z "$(PERF_TEST_MESH_VERSION)" ]]; then echo "PERF_TEST_MESH_VERSION must be defined"; exit 1; fi

.PHONY: ecr/push/%
ecr/push/%: check-perf-test-mesh-version
	docker pull kong/$*:$(PERF_TEST_MESH_VERSION) --platform linux/arm64 && \
	docker tag kong/$*:$(PERF_TEST_MESH_VERSION)-arm64 $(CONTAINER_REGISTRY)/$*:$(PERF_TEST_MESH_VERSION) && \
	docker push $(CONTAINER_REGISTRY)/$*:$(PERF_TEST_MESH_VERSION)

.PHONY: ecr/push/fake-service
ecr/push/fake-service:
	docker pull nicholasjackson/fake-service:v0.26.0 --platform linux/arm64 && \
	docker tag nicholasjackson/fake-service:v0.26.0 $(CONTAINER_REGISTRY)/fake-service:v0.26.0 && \
	docker push $(CONTAINER_REGISTRY)/fake-service:v0.26.0

.PHONY: ecr/push
ecr/push: ecr/authenticate ecr/push/kuma-cp ecr/push/kuma-dp ecr/push/kuma-init ecr/push/kumactl ecr/push/fake-service
