HTTPBIN_IMAGE := kennethreitz/httpbin
HTTPBIN_NAME  := httpfly-httpbin
HTTPBIN_PORT  ?= 8080

# The plugin's own git tag (vX.Y.Z[-N-gSHA]) with the leading "v" stripped,
# or "dev" outside a git checkout.
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || true)
VERSION     := $(or $(VERSION),dev)
VERSION_PKG := github.com/cristianradulescu/httpfly.nvim/backend/internal/cli

LUA_DIRS := lua plugin ftdetect ftplugin

.PHONY: build test fmt vet lint lint-backend format format-fix check httpbin-up httpbin-down httpbin-logs

## Build the httpfly backend binary into ./bin (what the plugin runs).
build:
	@mkdir -p bin
	cd backend && go build -ldflags "-X $(VERSION_PKG).Version=$(VERSION)" -o ../bin/httpfly ./cmd/httpfly

## Run the backend's Go test suite.
test:
	cd backend && go test ./...

## Format the Go code.
fmt:
	cd backend && gofmt -l -w .

## Vet the Go code.
vet:
	cd backend && go vet ./...

## Fail if any Go file isn't gofmt-formatted.
lint-backend:
	@test -z "$$(gofmt -l backend)" || (gofmt -l backend && exit 1)

## Run golangci-lint (see backend/.golangci.yml); install: https://golangci-lint.run/welcome/install/
lint:
	cd backend && golangci-lint run

## Check Lua formatting (stylua, see .stylua.toml).
format:
	stylua --check $(LUA_DIRS)

## Format the Lua code.
format-fix:
	stylua $(LUA_DIRS)

## Run everything CI would run.
check: lint-backend vet test format

## Start a local httpbin container for trying the docs/examples/*.http files against.
httpbin-up:
	docker run --rm -d --name $(HTTPBIN_NAME) -p $(HTTPBIN_PORT):80 $(HTTPBIN_IMAGE)
	@echo "httpbin running at http://localhost:$(HTTPBIN_PORT)"

## Stop the local httpbin container.
httpbin-down:
	docker stop $(HTTPBIN_NAME)

## Tail the local httpbin container's logs.
httpbin-logs:
	docker logs -f $(HTTPBIN_NAME)
