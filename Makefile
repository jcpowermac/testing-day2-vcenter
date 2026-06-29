.PHONY: vet build test-dry-run test-readonly test-p0 test-mutating

GINKGO ?= $(shell go env GOPATH)/bin/ginkgo

vet:
	go vet ./...

build:
	go build ./...

test-dry-run:
	$(GINKGO) --dry-run ./test/e2e/

test-readonly:
	$(GINKGO) --label-filter="readonly" ./test/e2e/

test-p0:
	$(GINKGO) --label-filter="p0 && readonly" ./test/e2e/

test-mutating:
	$(GINKGO) --label-filter="mutating" ./test/e2e/
