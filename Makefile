.PHONY: vet build test-dry-run test-readonly test-p0 test-mutating test-real apply-lab restore-lab verify-lab day2-lab

GINKGO ?= $(shell go env GOPATH)/bin/ginkgo
CONFIG ?= config/lab.yaml

vet:
	go vet ./...

build:
	go build ./...

day2-vcenter:
	go run ./cmd/day2-vcenter $(ARGS)

apply-lab:
	test -f $(CONFIG) || (echo "missing $(CONFIG) — copy config/lab.yaml.example and edit"; exit 1)
	go run ./cmd/day2-vcenter apply -config $(CONFIG)

restore-lab:
	test -f $(CONFIG) || (echo "missing $(CONFIG)"; exit 1)
	go run ./cmd/day2-vcenter restore -config $(CONFIG)

verify-lab:
	test -f $(CONFIG) || (echo "missing $(CONFIG)"; exit 1)
	go run ./cmd/day2-vcenter verify -config $(CONFIG)

test-dry-run:
	$(GINKGO) --dry-run ./test/e2e/

test-readonly:
	$(GINKGO) --label-filter="readonly" ./test/e2e/

test-p0:
	$(GINKGO) --label-filter="p0 && readonly" ./test/e2e/

test-mutating:
	$(GINKGO) --label-filter="mutating" ./test/e2e/

test-real:
	test -f $(CONFIG) || (echo "missing $(CONFIG) — copy config/lab.yaml.example and edit"; exit 1)
	E2E_LAB_CONFIG=$(CONFIG) $(GINKGO) --label-filter="real-vcenter" ./test/e2e/

# Full real-vCenter workflow: apply, verify tests, restore
day2-lab: apply-lab test-real restore-lab
