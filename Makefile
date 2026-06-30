.PHONY: vet build test-dry-run test-readonly test-p0 test-mutating test-storage test-storage-readonly test-real test-e2e apply-lab restore-lab verify-lab

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
	E2E_LAB_CONFIG=$(abspath $(CONFIG)) $(GINKGO) --label-filter="mutating" ./test/e2e/

test-storage:
	E2E_LAB_CONFIG=$(abspath $(CONFIG)) $(GINKGO) --label-filter="storage" ./test/e2e/

test-storage-readonly:
	$(GINKGO) --label-filter="storage && readonly" ./test/e2e/

test-real:
	test -f $(CONFIG) || (echo "missing $(CONFIG) — copy config/lab.yaml.example and edit"; exit 1)
	E2E_LAB_CONFIG=$(abspath $(CONFIG)) $(GINKGO) --label-filter="real-vcenter" ./test/e2e/

# Full end-to-end: baseline → apply → all tests → restore
test-e2e:
	test -f $(CONFIG) || (echo "missing $(CONFIG) — copy config/lab.yaml.example and edit"; exit 1)
	@echo "=== Phase 1: baseline readonly (single-vCenter) ==="
	$(GINKGO) --label-filter="readonly" ./test/e2e/
	@echo "=== Phase 2: apply second vCenter ==="
	go run ./cmd/day2-vcenter apply -config $(CONFIG)
	@echo "=== Phase 2b: verify cluster readiness ==="
	go run ./cmd/day2-vcenter verify -config $(CONFIG)
	@echo "=== Phase 3: readonly (multi-vCenter) ==="
	$(GINKGO) --label-filter="readonly" ./test/e2e/
	@echo "=== Phase 4: mutating ==="
	E2E_LAB_CONFIG=$(abspath $(CONFIG)) $(GINKGO) --label-filter="mutating" ./test/e2e/
	@echo "=== Phase 5: storage ==="
	E2E_LAB_CONFIG=$(abspath $(CONFIG)) $(GINKGO) --label-filter="storage" ./test/e2e/
	@echo "=== Phase 6: restore ==="
	go run ./cmd/day2-vcenter restore -config $(CONFIG)
