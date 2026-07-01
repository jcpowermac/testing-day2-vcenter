# Testing

## Remote workflow
1. Edit and commit locally, `git push`
2. `ssh jcallen@10.38.201.171 'cd ~/Development/testing-day2-vcenter && git pull'`
3. `ssh jcallen@10.38.201.171 'KUBECONFIG=$HOME/before-installer-testing/vsphere-ipi/auth/kubeconfig make -C ~/Development/testing-day2-vcenter test-readonly'`

Always use `make -C <path> <target>`. Never rsync — use git push/pull.

## Key make targets
- `make test-readonly` — safe readonly tests (no cluster mutation)
- `make test-storage` — CSI storage tests (label filter: storage)
- `make test-storage-readonly` — storage tests that don't provision PVCs
- `make test-p0` — p0 priority readonly tests
- `make test-mutating` — mutating tests (backup/restore)
- `make test-real` — requires `config/lab.yaml` with real second vCenter
- `make test-e2e` — full end-to-end: baseline → apply → verify → all tests → restore
- `make apply-lab` / `restore-lab` — add/remove second vCenter (waits for cluster readiness)
- `make verify-lab` — verify second vCenter was added correctly

## Test reports & Ginkgo flags
JUnit XML reports written to `reports/` (gitignored), one per target (e.g. `readonly.xml`, `mutating.xml`).
`test-e2e` writes per-phase reports (`phase1-readonly.xml`, `phase3-readonly.xml`, etc.).
Default `GINKGO_FLAGS=-v`. Override to control behavior:
- `GINKGO_FLAGS="-vv"` — verbose + GinkgoWriter output for passing tests
- `GINKGO_FLAGS="-v --fail-fast"` — stop on first failure
- `GINKGO_FLAGS="-v --focus='N-TOPO-01'"` — run one test
- `GINKGO_FLAGS="-v --skip='N-SEQ-01|N-SEQ-02'"` — skip specific tests

## Test files
- `helpers_test.go` — BeforeSuite, shared utilities, spec builders, namespace helpers
- `featuregate_test.go` — feature gate presence/state
- `infrastructure_validation_test.go` — xValidation CRD CEL rules (N-INF-*)
- `vap_test.go` — VAP existence, Machine/CPMS/MachineSet denial (N-SEQ-01/02/03)
- `configmap_content_test.go` — cloud config format/parity
- `configmap_ownership_test.go` — ConfigMap ownership and recreation
- `operator_health_test.go` — ClusterOperator health + pod crash loop checks
- `credentials_test.go` — credential secret propagation across 4 consumers
- `machine_integration_test.go` — Machine phase, labels, providerSpec vs FD topology
- `cpms_integration_test.go` — CPMS FD names match Infrastructure FDs
- `machineset_integration_test.go` — MachineSet providerSpec/labels vs FD topology
- `csi_integration_test.go` — CSI credential secret, pods, cloud config parity
- `topology_lifecycle_test.go` — mutating add/remove + 0-replica MS VAP probe
- `real_vcenter_test.go` — real second vCenter end-to-end verification
- `csi_storage_test.go` — CSI storage provisioning, topology, FD removal probes (N-CSI-*). AfterAll force-deletes orphaned Machines if drain times out.
- `problem_detector_test.go` — vsphere-problem-detector (stub)

## Guards
- `requireMultiVCenter()` — skips tests needing 2+ vCenters (run `make apply-lab` first)
- `requireLabConfig()` / `requireLabConfigWithFD()` — skips tests needing `E2E_LAB_CONFIG`
- `requireGateEnabled()` — skips tests needing `VSphereMultiVCenterDay2` gate

## VAP test approach
VAP denial tests (N-SEQ-01/02/03, N-SEQ-05, N-TOPO-01) use real patches, not dry-run.
A denied patch doesn't mutate the cluster. If the VAP is broken and the patch goes through,
that's a product bug worth discovering. These tests are labeled `mutating`.
xValidation (CEL) tests (N-INF-*) use dry-run — CEL evaluates identically under dry-run.

## Cluster readiness
`Apply()` and `Restore()` call `waitForClusterReady()` which checks (each with 15min timeout):
1. Operators stable (Available=True, Degraded=False, Progressing=False): `cloud-controller-manager`, `config-operator`, `machine-api`, `storage`
2. All non-deleting Machines in "Running" phase (Failed Machines with no NodeRef are skipped — dead CPMS replacements)
3. All Nodes in Ready condition
4. All Running Machines have region/zone labels (CCM syncs vCenter tags asynchronously after Infrastructure change)

## Credential secrets
- Apply/restore only update root credential secrets (`kube-system/vsphere-creds`, `openshift-config/cloud-credentials`)
- The CCO reconciles `openshift-machine-api/vsphere-cloud-credentials` from root secrets — never write to it directly (causes conflict errors)
- Backup still captures the CCO-managed secret for completeness; restore skips it

## test-e2e restore guarantee
The Makefile `test-e2e` target always runs restore (Phase 6) after apply, even if test phases 3-5 fail. Test failures no longer leave the cluster dirty.

## Test results (2026-07-01, full e2e run)
Phase 1 (single-vCenter baseline): 41/41 passed.
Phase 3 (multi-vCenter readonly): 50/52 passed. 2 failures:
- Machine missing region/zone labels (readiness race — now fixed with WaitForAllMachinesLabeled)
- N-INF-12: CRD allows orphaned FD→vCenter reference (product bug, SPLAT-2827)
Phase 4 (mutating): 19/21 passed. 2 failures:
- N-SEQ-04: same xValidation gap as N-INF-12 (product bug, SPLAT-2827)
- N-CSI-09: SCC annotation race on new namespace (now fixed with namespace readiness poll)
Phase 5 (storage): 10/11 passed. 1 failure:
- N-CSI-09: xValidation gap let vCenter removal dry-run succeed; cleanup timed out (product bug + now fixed with force-delete)
Phase 6 (restore): failed due to orphaned Machine blocking VAP (now fixed with force-delete cleanup)

### Product bugs found
- SPLAT-2827: No xValidation rule prevents removing a vCenter still referenced by a failure domain's `.server` field
- CSI driver crash-loops with "topology-categories required" when multi-vCenter config lacks topology categories
- CSI storage policy not propagated to second vCenter (documented in `mem:cluster_state`)