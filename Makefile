.PHONY: build test vet clean install run help

BINARY_NAME=BOSS-cli
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS=-ldflags "-X github.com/Rhypoo-Ma/BOSS-cli/cmd.version=$(VERSION) \
	-X github.com/Rhypoo-Ma/BOSS-cli/cmd.commit=$(COMMIT) \
	-X github.com/Rhypoo-Ma/BOSS-cli/cmd.buildDate=$(BUILD_DATE)"

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the CLI binary
	go build $(LDFLAGS) -o $(BINARY_NAME) .

test: ## Run Go tests
	go test ./...

vet: ## Run go vet
	go vet ./...

clean: ## Remove build artifacts
	rm -f $(BINARY_NAME)

install: ## Install to $GOPATH/bin
	go install $(LDFLAGS) .

run: build ## Build and run the CLI (pass args with ARGS=...)
	./$(BINARY_NAME) $(ARGS)
