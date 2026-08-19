HTTPBIN_IMAGE := kennethreitz/httpbin
HTTPBIN_NAME  := httpfly-httpbin
HTTPBIN_PORT  ?= 8080

VERSION     := $(shell cat VERSION)
VERSION_PKG := github.com/cristianradulescu/httpfly/internal/cli

.PHONY: build test fmt vet lint check httpbin-up httpbin-down httpbin-logs

## Build the httpfly binary into ./bin.
build:
	go build -ldflags "-X $(VERSION_PKG).Version=$(VERSION)" -o bin/httpfly ./cmd/httpfly

## Run the test suite.
test:
	go test ./...

## Format the code.
fmt:
	gofmt -l -w .

## Vet the code.
vet:
	go vet ./...

## Run golangci-lint (see .golangci.yml); install: https://golangci-lint.run/welcome/install/
lint:
	golangci-lint run

## Run everything CI would run.
check: fmt vet test

## Start a local httpbin container for trying the doc/examples/*.http files against.
httpbin-up:
	docker run --rm -d --name $(HTTPBIN_NAME) -p $(HTTPBIN_PORT):80 $(HTTPBIN_IMAGE)
	@echo "httpbin running at http://localhost:$(HTTPBIN_PORT)"

## Stop the local httpbin container.
httpbin-down:
	docker stop $(HTTPBIN_NAME)

## Tail the local httpbin container's logs.
httpbin-logs:
	docker logs -f $(HTTPBIN_NAME)
