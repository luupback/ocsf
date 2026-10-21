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

# Regenerate ocsf_gen.go from the embedded compiled schema and gen.yaml.
generate:
	go generate ./...

# Fails if ocsf_gen.go is stale (CI).
check: generate
	git diff --exit-code -- ocsf_gen.go

# Compile an OCSF release into validate/schema (network and python3
# required; not run in CI). Then update the //go:embed and //go:generate
# paths if VERSION changed, and run make generate.
schema:
	scripts/compile-schema.sh $(VERSION)
