BINARY  := bitrise-cli
MODULE  := github.com/bitrise-io/bitrise-cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -s -w -X $(MODULE)/cmd.version=$(VERSION) -X $(MODULE)/cmd.commit=$(COMMIT)

GOLANGCI_LINT_VERSION := v2.1.6

.PHONY: build fmt vet lint test tidy clean

build:
	go build -ldflags "$(LDFLAGS)" -o ./$(BINARY) .

fmt:
	gofmt -l .

vet:
	go vet ./...

lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	golangci-lint run ./...

test:
	go test -race -count=1 -timeout=5m ./...

tidy:
	go mod tidy && git diff --exit-code go.mod go.sum

clean:
	rm -f ./$(BINARY)
