VERSION ?= 1.5.0

GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
ACTIONLINT    := go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12

.PHONY: test lint lint-ci generate check schema

test:
	go test -race ./...

lint:
	$(GOLANGCI_LINT) run ./...

lint-ci:
	$(ACTIONLINT)

GENERATED := ocsf_gen.go catalog/catalog_gen.go

# Regenerate ocsf_gen.go (from the embedded compiled schema and gen.yaml)
# and catalog/catalog_gen.go (from the schema).
generate:
	go generate ./...

# Fails if a generated file is stale or untracked (CI).
check: generate
	git diff --exit-code -- $(GENERATED)
	@test -z "$$(git status --porcelain -- $(GENERATED))" || { git status --short -- $(GENERATED); exit 1; }

# Compile an OCSF release into validate/schema (network and python3
# required; not run in CI). Then update the //go:embed and //go:generate
# paths if VERSION changed, and run make generate.
schema:
	scripts/compile-schema.sh $(VERSION)
