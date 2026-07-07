.PHONY: vet build test-dry-run test-readonly test-p0 test-mutating test-storage test-storage-readonly test-csi-operator test-csi-topology test-csi-orphan test-real test-e2e apply-lab restore-lab verify-lab

GINKGO ?= $(shell go env GOPATH)/bin/ginkgo
GINKGO_FLAGS ?= -v
REPORT_DIR ?= reports
CONFIG ?= config/lab.yaml

GINKGO_REPORT = --output-dir=$(REPORT_DIR) --junit-report

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
	$(GINKGO) $(GINKGO_FLAGS) --dry-run ./test/e2e/

test-readonly:
	@mkdir -p $(REPORT_DIR)
	$(GINKGO) $(GINKGO_FLAGS) $(GINKGO_REPORT)=readonly.xml --label-filter="readonly" ./test/e2e/

test-p0:
	@mkdir -p $(REPORT_DIR)
	$(GINKGO) $(GINKGO_FLAGS) $(GINKGO_REPORT)=p0.xml --label-filter="p0 && readonly" ./test/e2e/

test-mutating:
	@mkdir -p $(REPORT_DIR)
	E2E_LAB_CONFIG=$(abspath $(CONFIG)) $(GINKGO) $(GINKGO_FLAGS) $(GINKGO_REPORT)=mutating.xml --label-filter="mutating" ./test/e2e/

test-storage:
	@mkdir -p $(REPORT_DIR)
	E2E_LAB_CONFIG=$(abspath $(CONFIG)) $(GINKGO) $(GINKGO_FLAGS) $(GINKGO_REPORT)=storage.xml --label-filter="storage" ./test/e2e/

test-storage-readonly:
	@mkdir -p $(REPORT_DIR)
	$(GINKGO) $(GINKGO_FLAGS) $(GINKGO_REPORT)=storage-readonly.xml --label-filter="storage && readonly" ./test/e2e/

test-csi-operator:
	@mkdir -p $(REPORT_DIR)
	test -f $(CONFIG) || (echo "missing $(CONFIG) — copy config/lab.yaml.example and edit"; exit 1)
	E2E_LAB_CONFIG=$(abspath $(CONFIG)) $(GINKGO) $(GINKGO_FLAGS) $(GINKGO_REPORT)=csi-operator.xml --label-filter="csi-operator" ./test/e2e/

# Readonly + baseline mutating check of ClusterCSIDriver topologyCategories precedence.
# Does not require E2E_LAB_CONFIG (TOPO-01–05 are readonly, TOPO-06 only needs cluster access).
test-csi-topology:
	@mkdir -p $(REPORT_DIR)
	$(GINKGO) $(GINKGO_FLAGS) $(GINKGO_REPORT)=csi-topology.xml --label-filter="csi-topology" ./test/e2e/

# Synthetic orphan tag tests: attach the cluster tag to a non-FD datastore and
# verify the operator detects and cleans it up. Requires E2E_LAB_CONFIG with
# orphanTest.datastore set (or a discoverable non-FD datastore), and requires
# `make apply-lab` to have run first so the cluster tag/category exist on
# secondVCenter.
test-csi-orphan:
	@mkdir -p $(REPORT_DIR)
	test -f $(CONFIG) || (echo "missing $(CONFIG) — copy config/lab.yaml.example and edit"; exit 1)
	E2E_LAB_CONFIG=$(abspath $(CONFIG)) $(GINKGO) $(GINKGO_FLAGS) $(GINKGO_REPORT)=csi-orphan.xml --label-filter="csi-orphan" ./test/e2e/

test-real:
	@mkdir -p $(REPORT_DIR)
	test -f $(CONFIG) || (echo "missing $(CONFIG) — copy config/lab.yaml.example and edit"; exit 1)
	E2E_LAB_CONFIG=$(abspath $(CONFIG)) $(GINKGO) $(GINKGO_FLAGS) $(GINKGO_REPORT)=real-vcenter.xml --label-filter="real-vcenter" ./test/e2e/

# Full end-to-end: baseline → apply → all tests → restore
# Restore always runs after apply, even if tests fail.
test-e2e:
	test -f $(CONFIG) || (echo "missing $(CONFIG) — copy config/lab.yaml.example and edit"; exit 1)
	@mkdir -p $(REPORT_DIR)
	@echo "=== Phase 1: baseline readonly (single-vCenter) ==="
	$(GINKGO) $(GINKGO_FLAGS) $(GINKGO_REPORT)=phase1-readonly.xml --label-filter="readonly" ./test/e2e/
	@echo "=== Phase 2: apply second vCenter ==="
	go run ./cmd/day2-vcenter apply -config $(CONFIG)
	@echo "=== Phase 2b: verify cluster readiness ==="
	go run ./cmd/day2-vcenter verify -config $(CONFIG)
	@rc=0; \
	echo "=== Phase 3: readonly (multi-vCenter) ==="; \
	$(GINKGO) $(GINKGO_FLAGS) $(GINKGO_REPORT)=phase3-readonly.xml --label-filter="readonly" ./test/e2e/ || rc=$$?; \
	echo "=== Phase 4: mutating ==="; \
	E2E_LAB_CONFIG=$(abspath $(CONFIG)) $(GINKGO) $(GINKGO_FLAGS) $(GINKGO_REPORT)=phase4-mutating.xml --label-filter="mutating" ./test/e2e/ || rc=$$?; \
	echo "=== Phase 5: storage ==="; \
	E2E_LAB_CONFIG=$(abspath $(CONFIG)) $(GINKGO) $(GINKGO_FLAGS) $(GINKGO_REPORT)=phase5-storage.xml --label-filter="storage" ./test/e2e/ || rc=$$?; \
	echo "=== Phase 6: restore ==="; \
	go run ./cmd/day2-vcenter restore -config $(CONFIG); \
	exit $$rc
