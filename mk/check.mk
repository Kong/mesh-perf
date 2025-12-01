.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: fmt
fmt:
	$(GO) fmt ./...

.PHONY: check
check: clean tidy fmt generate
	# fail if Git working tree is dirty or there are untracked files
	git diff --quiet || \
	git ls-files --other --directory --exclude-standard --no-empty-directory | wc -l | read UNTRACKED_FILES; if [ "$$UNTRACKED_FILES" != "0" ]; then false; fi || \
	test $$(git diff --name-only | wc -l) -eq 0 || \
	( \
		echo "The following changes (result of code generators and code checks) have been detected:" && \
		git --no-pager diff && \
		echo "The following files are untracked:" && \
		git ls-files --other --directory --exclude-standard --no-empty-directory && \
		false \
	)

# -------------------------------------------------------------------
# Terraform Linting
# -------------------------------------------------------------------

.PHONY: lint/terraform/%
lint/terraform/%:
	$(TFLINT) --chdir=$* --init
	$(TFLINT) --chdir=$*

.PHONY: lint/terraform
lint/terraform: $(foreach component,$(notdir $(wildcard $(TOP)/infrastructure/*)),lint/terraform/infrastructure/$(component))
