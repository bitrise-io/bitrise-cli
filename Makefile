BINARY  := bitrise-cli
MODULE  := github.com/bitrise-io/bitrise-cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -s -w -X $(MODULE)/cmd.version=$(VERSION) -X $(MODULE)/cmd.commit=$(COMMIT)

# RELEASE=true reproduces the shipped command surface locally: non-GA (stub)
# namespaces are hidden from --help and completion. Off by default so dev
# builds keep the full surface. The CI release build sets this on its own.
ifeq ($(RELEASE),true)
LDFLAGS += -X $(MODULE)/cmd.releaseBuild=true
endif

GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2

.PHONY: build fmt vet lint lint-fix test tidy docs docs-check clean

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

clean:
	rm -f ./$(BINARY)
