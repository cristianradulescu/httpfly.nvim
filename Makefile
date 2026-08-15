HTTPBIN_IMAGE := kennethreitz/httpbin
HTTPBIN_NAME  := httpfly-httpbin
HTTPBIN_PORT  ?= 8080

.PHONY: httpbin-up httpbin-down httpbin-logs

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
