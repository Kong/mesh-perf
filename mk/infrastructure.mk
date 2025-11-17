AWS_REGION ?= us-west-1

# ENV controls which component/environment Terraform will operate on. It defaults
# to "local" if not explicitly set, and is used when generating the -chdir flag for
# Terraform commands (e.g., -chdir=$(local_DIR) if ENV=local).
ENV ?= local

# Enabling DEBUG will configure certain Terraform resources more explicitly, increase logging,
# and, for example, install a metrics server in EKS clusters.
DEBUG := $(or $(DEBUG),$(or $(RUNNER_DEBUG),false))

# Environment variables needed for eksformula calculation
E2E_TF_VARS += PERF_TEST_NUM_SERVICES=$${PERF_TEST_NUM_SERVICES:=70}
E2E_TF_VARS += PERF_TEST_INSTANCES_PER_SERVICE=$${PERF_TEST_INSTANCES_PER_SERVICE:=2}

# Additional Terraform variables for the EKS component
eks_TF_VARS += -var="ci=$(or $(CI),false)"
eks_TF_VARS += -var="debug=$(DEBUG)"
eks_TF_VARS += -var="region=$(AWS_REGION)"
eks_TF_VARS += -var='availability_zones=["$(AWS_REGION)b","$(AWS_REGION)c"]'
ifneq ($(ENV),local)
eks_TF_VARS += -var="nodes_number=$(shell $(E2E_TF_VARS) go run tools/eksformula/main.go)"
endif

# MAKE_INFRA_TARGETS macro
# 1. Stores the relative path to "$(TOP)/infrastructure/$(1)" in "$(1)_DIR".
# 2. If "$(1)_TF_VARS" is defined, assigns that value to VARS for the
#    "terraform/apply/..." and "terraform/destroy/..." targets.
# 3. Defines "infra/create/$(1)" to:
#    - Optionally invoke "terraform/init/..." if INIT != false.
#    - Invoke "terraform/apply/..." with CHDIR set to "$($(1)_DIR)" (used by TF_CMD).
#    - Also depend on "ecr/push" if $(1) is "eks".
# 4. Defines "infra/destroy/$(1)" to invoke "terraform/destroy/..." for the component.
define MAKE_INFRA_TARGETS
$(1)_DIR := infrastructure/$(1)

# Apply/destroy targets use $(1)_TF_VARS if defined.
terraform/apply/$$($(1)_DIR) terraform/destroy/$$($(1)_DIR): VARS = $($(1)_TF_VARS)

.PHONY: infra/create/$(1) infra/destroy/$(1)

# Both create/destroy targets share CHDIR based on $(1)_DIR.
infra/create/$(1) infra/destroy/$(1): CHDIR = $$($(1)_DIR)

infra/create/$(1): \
  $(if $(filter-out false,$(INIT)),terraform/init/$$($(1)_DIR)) \
  terraform/apply/$$($(1)_DIR) \
  $(if $(filter eks,$(1)),ecr/push,)

infra/destroy/$(1): \
  $(if $(filter-out false,$(INIT)),terraform/init/$$($(1)_DIR)) \
  terraform/destroy/$$($(1)_DIR)
endef

# Automatically discover each subdirectory in "$(TOP)/infrastructure/" (e.g., eks, local, etc.),
# interpret it as a "component," and then generate the associated create/destroy targets via
# the MAKE_INFRA_TARGETS macro. This approach avoids manually listing components and updates
# automatically as new directories appear.
$(foreach component,$(notdir $(wildcard $(TOP)/infrastructure/*)),$(eval $(call MAKE_INFRA_TARGETS,$(component))))

# Top-level targets forwarding to infra/create/$(ENV) and infra/destroy/$(ENV).
.PHONY: infra/create infra/destroy
infra/create infra/destroy:
	@$(MAKE) $@/$(ENV)

# -------------------------------------------------------------------
# Terraform
# -------------------------------------------------------------------

# TF_CMD is a generic Terraform command that sets its working directory via CHDIR, if provided,
# or defaults to "$($(ENV)_DIR)". For example, if ENV=eks, the directory is "$(eks_DIR)".
# If ENV is not defined, it defaults to "local", so the directory becomes "$(local_DIR)".
# Individual targets can override CHDIR when needed.
TF_CMD = $(TERRAFORM) -chdir=$(or $(CHDIR),$($(ENV)_DIR))

# Initialize Terraform in the specified directory.
# - DIR is dynamically set to $*, which represents the matched part of the target.
.PHONY: terraform/init/%
terraform/init/%: CHDIR = $*
terraform/init/%:
	$(TF_CMD) init$(if $(UPGRADE), -upgrade,)$(if $(RECONFIGURE), -reconfigure,)

# Generic rule to apply or destroy Terraform configurations.
# - Uses $* to dynamically extract the directory path from the target.
# - Extracts "apply" or "destroy" from the target name via $(word 2,$(subst /,,$@)).
# - Passes component-specific variables via $(VARS).
# - Auto-approval is enabled unless AUTO_APPROVE is explicitly set to false.
.PHONY: terraform/apply/% terraform/destroy/%
terraform/apply/% terraform/destroy/%:
	@$(TERRAFORM) -chdir=$* \
		$(word 2,$(subst /, ,$@)) \
		$(if $(VARS),$(VARS)) \
		$(if $(filter false,$(AUTO_APPROVE)),,-auto-approve)

# -------------------------------------------------------------------
# ECR (Elastic Container Registry)
# -------------------------------------------------------------------

# Fetch the ECR container registry URL dynamically at runtime.
CONTAINER_REGISTRY = $(shell $(TF_CMD) output -json | jq --raw-output '.registry.value // ""')

.PHONY: ecr/authenticate
ecr/authenticate:
	aws ecr get-login-password --region $(AWS_REGION) | docker login $(CONTAINER_REGISTRY) --username AWS --password-stdin

.PHONY: check-perf-test-mesh-version
check-perf-test-mesh-version:
	@if [ -z "$(PERF_TEST_MESH_VERSION)" ]; then \
	  echo "PERF_TEST_MESH_VERSION must be defined"; \
	  exit 1; \
	fi

.PHONY: ecr/push/%
ecr/push/%: check-perf-test-mesh-version
	docker pull kong/$*:$(PERF_TEST_MESH_VERSION) --platform linux/arm64 && \
	docker tag kong/$*:$(PERF_TEST_MESH_VERSION) $(CONTAINER_REGISTRY)/$*:$(PERF_TEST_MESH_VERSION) && \
	docker push $(CONTAINER_REGISTRY)/$*:$(PERF_TEST_MESH_VERSION)

.PHONY: ecr/push/fake-service
ecr/push/fake-service:
	docker pull nicholasjackson/fake-service:v0.26.0 --platform linux/arm64 && \
	docker tag nicholasjackson/fake-service:v0.26.0 $(CONTAINER_REGISTRY)/fake-service:v0.26.0 && \
	docker push $(CONTAINER_REGISTRY)/fake-service:v0.26.0

.PHONY: ecr/push
ecr/push: ecr/authenticate ecr/push/kuma-dp ecr/push/fake-service
