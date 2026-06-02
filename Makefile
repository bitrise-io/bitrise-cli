BINARY  := bitrise-cli
MODULE  := github.com/bitrise-io/bitrise-cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -s -w -X $(MODULE)/cmd.version=$(VERSION) -X $(MODULE)/cmd.commit=$(COMMIT)

GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
GORELEASER    := go run github.com/goreleaser/goreleaser/v2@v2.16.0

.PHONY: build fmt vet lint lint-fix test tidy docs docs-check release-check release-snapshot release-ci clean

build:
	go build -ldflags "$(LDFLAGS)" -o ./$(BINARY) .

fmt:
	gofmt -l .

vet:
	go vet ./...

lint:
	$(GOLANGCI_LINT) run ./...

lint-fix:
	$(GOLANGCI_LINT) run --fix ./...

test:
	go test -race -count=1 -timeout=5m ./...

tidy:
	go mod tidy && git diff --exit-code go.mod go.sum

docs:
	go run ./tools/gendocs

docs-check: docs
	@status=$$(git status --porcelain docs/cli README.md); \
	if [ -n "$$status" ]; then \
		echo "Generated CLI docs are out of date — run 'make docs' and commit the result:"; \
		echo "$$status"; \
		exit 1; \
	fi

# Validate .goreleaser.yaml.
release-check:
	$(GORELEASER) check

# Build every release platform into dist/ without tagging or publishing.
release-snapshot:
	$(GORELEASER) release --snapshot --clean

# Real release: builds all platforms and publishes a draft GitHub release.
# Run by the `release` workflow in bitrise.yml on vX.Y.Z tag pushes; needs
# GITHUB_TOKEN with contents:write. Not intended for laptops.
release-ci:
	$(GORELEASER) release --clean

clean:
	rm -f ./$(BINARY)
	rm -rf ./dist
