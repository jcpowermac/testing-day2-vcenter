# CSI Storage Provisioning Test Plan

## Context

Existing CSI tests (`test/e2e/csi_integration_test.go`) are readonly: credential parity, pod health, cloud config datacenter coverage. No tests exercise actual storage provisioning — PV/PVC creation, topology-aware binding, or deletion protection when PVs exist. The multi-vCenter Day 2 feature must not break storage, and removal guardrails must protect PV-backed failure domains.

## Scope

1. **PV provisioning in a new failure domain** — after adding a second vCenter/FD, prove a PVC can bind to a PV there with correct topology labels
2. **FD/vCenter deletion protection with PVs** — attempt FD and vCenter removal when PVs exist in that topology; document whether denial occurs
3. **CSI driver health after topology change** — ClusterCSIDriver operator conditions, pod stability, config propagation
4. **StorageClass topology awareness** — default SC exists, is CSI-backed, topology plumbing is connected

## Product Gap: No VAP for PV-Referenced Failure Domains

Existing VAPs protect FDs referenced by Machines (`N-SEQ-01`), CPMS (`N-SEQ-02`), and MachineSets (`N-SEQ-03`). **There is no `vsphere-failure-domain-in-use-by-pv` VAP.** This means removing a FD with PVs is likely allowed by the API server. Tests N-CSI-08 and N-CSI-09 probe this behavior and document the finding. If a future VAP is added, these tests will assert the denial.

## New Files

### `pkg/framework/storage.go` — storage operation helpers

| Function | Purpose |
|----------|---------|
| `CreateTestNamespace(ctx, client, prefix)` | Isolated namespace for test PVCs |
| `DeleteNamespace(ctx, client, name, timeout)` | Namespace cleanup with wait; tolerates `Terminating` with diagnostics |
| `GetStorageClass(ctx, client, name)` | Fetch SC by name |
| `GetDefaultStorageClass(ctx, client)` | Find SC with annotation `storageclass.kubernetes.io/is-default-class=true` |
| `CloneStorageClassWithTopology(ctx, client, source, name, topologyTerms)` | Clone default SC parameters (provisioner, reclaimPolicy, parameters), override allowedTopologies + name |
| `StorageClassIsWaitForFirstConsumer(sc)` | Check binding mode |
| `CreatePVC(ctx, client, ns, name, size, storageClassName)` | Create PVC; pass `""` for cluster default SC |
| `DeletePVC(ctx, client, ns, name)` | Delete PVC |
| `WaitForPVCBound(ctx, client, ns, name, timeout)` | Poll until Bound |
| `GetPV(ctx, client, name)` | Fetch PV |
| `DeletePV(ctx, client, name)` | Delete PV |
| `WaitForPVDeleted(ctx, client, name, timeout)` | Poll until gone; also polls for `VolumeAttachment` removal |
| `PVTopologyLabels(pv)` | Extract region/zone from PV nodeAffinity; returns `("","",false)` for PVs without nodeAffinity |
| `PVBelongsToFailureDomain(pv, region, zone)` | Match PV topology to FD |
| `ListPVsInFailureDomain(ctx, client, region, zone)` | All PVs in a given FD (skips PVs without nodeAffinity) |
| `CreateBusyboxPod(ctx, client, ns, name, pvcName)` | Minimal Pod to trigger WaitForFirstConsumer binding; uses `BusyboxImage` constant |
| `CreateBusyboxPodWithNodeSelector(ctx, client, ns, name, pvcName, nodeSelector)` | Pod with topology constraint |
| `DeletePod(ctx, client, ns, name)` | Delete Pod |
| `WaitForPodRunning(ctx, client, ns, name, timeout)` | Poll until Running |

### `pkg/framework/csi.go` — ClusterCSIDriver condition helpers (extends `conditions.go` pattern)

| Function | Purpose |
|----------|---------|
| `GetClusterCSIDriverCondition(ctx, client, name, conditionType)` | ClusterCSIDriver condition; uses same `wait.Poll` style as `conditions.go` |
| `WaitForClusterCSIDriverAvailable(ctx, client, name, timeout)` | Available=True, not Degraded; skip with message if ClusterCSIDriver CRD absent |

### `test/e2e/csi_storage_test.go` — 11 test specs

### Modified Files

- `pkg/framework/constants.go` — add CSI constants
- `test/e2e/helpers_test.go` — add `requireLabConfigWithFD()`, `createTestNamespaceWithCleanup()`, `requireDefaultStorageClass()`
- `pkg/framework/machine.go` — add `CloneMachineSetForFD` (see Appendix A)
- `Makefile` — add `test-csi` and `test-csi-readonly` targets

## Constants to Add (`pkg/framework/constants.go`)

```go
// CSI driver
CSIDriverNamespace        = "openshift-cluster-csi-drivers"
CSIDriverControllerLabel  = "app=vmware-vsphere-csi-driver-controller"
CSIDriverNodeLabel        = "app=vmware-vsphere-csi-driver-node"
CSICredentialSecretName   = "vmware-vsphere-cloud-credentials"
CSITopologyRegionKey      = "topology.csi.vmware.com/k8s-region"
CSITopologyZoneKey        = "topology.csi.vmware.com/k8s-zone"
ClusterCSIDriverName      = "csi.vsphere.vmware.com"
StorageOperatorName       = "storage"

// Test defaults
TestPVCSize               = "1Gi"
TestNamespacePrefix       = "e2e-csi-storage"
BusyboxImage              = "registry.k8s.io/e2e-test-images/busybox:1.36.1"
```

Note: `DefaultStorageClassName` removed as constant — tests use annotation-based discovery via `GetDefaultStorageClass()` (AR-CSI-C5).

## Test Specifications

### Category A: CSI Driver Health and Config (readonly)

**N-CSI-01: ClusterCSIDriver Available and not Degraded**
- Labels: `readonly`, `csi`, `p0`
- Fetches ClusterCSIDriver `csi.vsphere.vmware.com`, asserts Available=True, Degraded=False
- Supplements (not replaces) existing `operator_health_test.go` CO checks — this checks the CSI-specific CR, not the `storage` ClusterOperator
- Skip with message if ClusterCSIDriver CRD absent (pre-4.10 OCP)

**N-CSI-10: Default StorageClass exists and is CSI-backed**
- Labels: `readonly`, `csi`, `p1`
- Uses `GetDefaultStorageClass()` (annotation-based discovery, no hardcoded name)
- Verifies provisioner is `csi.vsphere.vmware.com`, volumeBindingMode is `WaitForFirstConsumer`
- Asserts `reclaimPolicy: Delete` (needed by N-CSI-07)

**N-CSI-11: StorageClass topology plumbing connected to Infrastructure FDs**
- Labels: `readonly`, `csi`, `p2`
- Gate: `requireGateEnabled()` when `len(vcenters) >= 2`
- If SC has `allowedTopologies`: each term maps to an Infrastructure FD
- If SC has no `allowedTopologies` and `len(vcenters) >= 2`: require at least one node in *lab FD* with matching CSI topology labels, or skip with "no node in lab FD topology"
- Single vCenter: verify at least one node has any CSI topology labels

### Category B: PV Provisioning in New Failure Domain (mutating, real-vcenter)

All mutating tests gate on: `requireDefaultStorageClass()` + `requireLabConfigWithFD()` + `requireGateEnabled()`

**N-CSI-04: PVC provisions and binds in existing failure domain** (smoke baseline)
- Labels: `real-vcenter`, `mutating`, `csi`, `p1`
- Creates test namespace, PVC (using default SC from annotation discovery) + busybox Pod, waits for Bound
- Asserts PV topology labels match *some* Infrastructure FD (not just "CSI works somewhere")
- Explicitly labeled as smoke — does not prove new-FD provisioning

**N-CSI-05: PV provisioned in new FD has correct topology labels** (headline test)
- Labels: `real-vcenter`, `mutating`, `csi`, `Serial`, `p1`
- Requires `labCfg.FailureDomain` set (skip otherwise)
- Shortcut: if a worker node already exists in the lab FD (check CSI topology labels), skip MachineSet creation and reuse existing node
- Full sequence when no node exists:
  1. Verify second vCenter + FD present in Infrastructure (applied via `make apply-lab`)
  2. Clone existing MachineSet via `CloneMachineSetForFD` (see Appendix A) — sets full providerSpec workspace from lab config
  3. Wait for 1 Machine Running in new FD
  4. Verify node has CSI topology labels matching FD region/zone
  5. Create PVC + Pod with nodeSelector constraining to new FD topology
  6. Wait PVC Bound (`LongTimeout` — 15min for first provisioning on new vCenter)
  7. Read PV, extract topology via `PVTopologyLabels(pv)`
  8. Assert PV region == FD region, PV zone == FD zone
  9. Cross-reference: `vsphere.FindFailureDomainByRegionZone(fds, pvRegion, pvZone)` is not nil and `fd.Server == labCfg.SecondVCenter.Server`

**N-CSI-06: PVC with explicit topology constraint binds in correct FD**
- Labels: `real-vcenter`, `mutating`, `csi`, `Serial`, `p2`
- Uses `CloneStorageClassWithTopology()` — clones default SC's provisioner, parameters, reclaimPolicy; overrides allowedTopologies to restrict to lab FD; unique name
- Creates PVC using custom SC, verifies PV topology matches lab FD
- `DeferCleanup`: Pod, PVC, StorageClass, namespace

**N-CSI-07: PV cleanup works after PVC deletion**
- Labels: `real-vcenter`, `mutating`, `csi`, `p2`
- Reads default SC and asserts `reclaimPolicy: Delete` before proceeding (skip otherwise)
- Creates PVC, binds via Pod, records PV name, deletes Pod+PVC
- `WaitForPVDeleted` with `LongTimeout` — polls VolumeAttachment removal, fails loudly with PV/attachment dump on timeout

### Category C: FD/vCenter Deletion Protection When PVs Exist (mutating, real-vcenter)

**N-CSI-08: Probe FD removal behavior when PVs exist in that FD**
- Labels: `real-vcenter`, `mutating`, `csi`, `Serial`, `p1`
- Provisions PV in lab FD, then attempts dry-run removal of that specific FD from Infrastructure spec
- **Denial reason parsing**: if denied, parse error message to distinguish:
  - Machine VAP (`vsphere-failure-domain-in-use-by-machine`) — log as "blocked by Machine VAP, not PV protection"
  - CPMS VAP — log similarly
  - MachineSet VAP — log similarly
  - Unknown/new VAP mentioning PV/volume — log as "PV VAP detected!"
- If allowed: log "PRODUCT GAP: FD removal allowed despite PVs present. No VAP/xValidation rule protects PV-backed failure domains."
- Uses dry-run only — never actually removes the FD
- When future `vsphere-failure-domain-in-use-by-pv` ships: test flips from "document allowed" to strict denial assertion (same test ID, behavior-driven)

**N-CSI-09: Confirm vCenter removal blocked by existing xValidation, PV presence irrelevant**
- Labels: `real-vcenter`, `mutating`, `csi`, `Serial`, `p1`
- Provisions PV in lab FD, then attempts dry-run removal of the vCenter hosting that FD
- Existing xValidation rule (N-INF-12 / N-SEQ-04) blocks vCenter removal when FDs still reference it
- Assert error references FD/server dependency, explicitly log "PV presence irrelevant — denial is FD reference, not PV protection"
- Documents that no incremental PV guard exists for vCenter removal

### Category D: CSI Health with Current Topology (readonly/real-vcenter)

**N-CSI-02: CSI driver pods healthy with current Infrastructure topology**
- Labels: `readonly`, `csi`, `p1`
- Snapshot check: CSI controller pods Running, restart count < 5
- Also checks CSI node DaemonSet pods (`CSIDriverNodeLabel`) for completeness
- **Does not claim before/after observation** — `make apply-lab` runs before `make test-csi`; this is a health snapshot, not a delta measurement
- If `len(vcenters) >= 2`: additionally verify pod count >= 1 (CSI scaled for multi-vCenter)

**N-CSI-03: CSI credential secret reflects all Infrastructure vCenters** (consolidated)
- Labels: `readonly`, `csi`, `p1`
- Gate: `requireGateEnabled()` when `len(vcenters) >= 2`; single-vCenter clusters skip the multi-vCenter assertion
- This replaces the overlap with existing `csi_integration_test.go` credential check — the existing test in `csi_integration_test.go` will be marked as covered by N-CSI-03 via comment, or removed to avoid duplicate
- Incremental assertion beyond existing: verify CSI config secret/configmap (probe for `vsphere-csi-config-secret` or similar in CSI namespace; skip if not found)

## Test Classification Summary

| ID | Readonly | Real vCenter | Mutating | Serial | Priority | Resources Created |
|----|----------|-------------|----------|--------|----------|-------------------|
| N-CSI-01 | yes | no | no | no | P0 | none |
| N-CSI-02 | yes | no | no | no | P1 | none |
| N-CSI-03 | yes | no | no | no | P1 | none |
| N-CSI-04 | no | yes | yes | no | P1 | namespace, PVC, Pod |
| N-CSI-05 | no | yes | yes | yes | P1 | MachineSet, namespace, PVC, Pod |
| N-CSI-06 | no | yes | yes | yes | P2 | StorageClass, namespace, PVC, Pod |
| N-CSI-07 | no | yes | yes | no | P2 | namespace, PVC, Pod |
| N-CSI-08 | no | yes | yes | yes | P1 | namespace, PVC, Pod |
| N-CSI-09 | no | yes | yes | yes | P1 | namespace, PVC, Pod |
| N-CSI-10 | yes | no | no | no | P1 | none |
| N-CSI-11 | yes | no | no | no | P2 | none |

## Cleanup Strategy

Every mutating test uses `DeferCleanup()`. Resources cleaned in reverse-dependency order:
1. Delete Pod (releases PVC mount)
2. Delete PVC (triggers PV deletion for reclaimPolicy:Delete)
3. `WaitForPVDeleted` with `LongTimeout` — polls VolumeAttachment removal, fails loudly with PV/attachment dump on timeout
4. Delete custom StorageClass (if created)
5. Scale MachineSet to 0, wait drained, delete (if created)
6. Delete test namespace last — tolerates `Terminating` state with diagnostics

Existing `AfterSuite` restores Infrastructure backup as final safety net.

## Makefile Targets

```makefile
test-csi:
	$(GINKGO) --label-filter="csi" ./test/e2e/

test-csi-readonly:
	$(GINKGO) --label-filter="csi && readonly" ./test/e2e/
```

Label is `csi` (not `storage`) to avoid collision with future non-vSphere storage tests (AR-CSI-N6).

## Appendix A: `CloneMachineSetForFD` providerSpec Field Mapping

New helper in `pkg/framework/machine.go`. Clones an existing MachineSet, then rewrites `VSphereMachineProviderSpec` fields from lab config:

| providerSpec field | Source |
|-------------------|--------|
| `workspace.server` | `labCfg.SecondVCenter.Server` |
| `workspace.datacenter` | `labCfg.FailureDomain.Topology.Datacenter` |
| `workspace.datastore` | `labCfg.FailureDomain.Topology.Datastore` |
| `workspace.resourcePool` | `labCfg.FailureDomain.Topology.ResourcePool` |
| `workspace.folder` | Derived: `/<datacenter>/vm/<infraID>` (read from source MachineSet, replace datacenter) |
| `network.devices[0].networkName` | `labCfg.FailureDomain.Topology.Networks[0]` |
| `template` | Derived: `/<datacenter>/vm/<infraID>/<infraID>-rhcos` (standard OCP template path in new DC) |

Additionally sets:
- Machine region/zone labels: `labCfg.FailureDomain.Region`, `labCfg.FailureDomain.Zone`
- MachineSet name: `<infraID>-csi-test-<fdName>`
- Replicas: 1
- `DeferCleanup` registered by caller

Preconditions checked:
- Source MachineSet must have valid `VSphereMachineProviderSpec` (type assertion)
- Lab config `FailureDomain.Topology` must have all required fields (validated by `labconfig.Validate()`)

## Key Design Notes

- **WaitForFirstConsumer**: default SC requires a consumer Pod for PVC binding — every provisioning test creates a busybox Pod using `BusyboxImage` constant (configurable for disconnected/mirrored clusters)
- **CSI topology keys**: `topology.csi.vmware.com/k8s-region` and `topology.csi.vmware.com/k8s-zone` map to Infrastructure FD region/zone which map to vCenter tag categories
- **StorageClass discovery**: annotation-based via `GetDefaultStorageClass()`, never hardcoded name — handles clusters where default SC is not named `thin-csi`
- **Timeouts**: `LongTimeout` (15min) for PV provisioning and PV deletion; `DefaultTimeout` (5min) for pod scheduling
- **Serial tests**: N-CSI-05/06/08/09 use `Serial` label — they create MachineSets and PVCs that could conflict with parallel lab use
- **Gate guard**: multi-vCenter topology tests (N-CSI-03/05/11) check `requireGateEnabled()` when asserting cross-vCenter behavior
- **Node reuse shortcut**: N-CSI-05 checks for existing worker in lab FD before creating a MachineSet — avoids 15min Machine provision when `make apply-lab` already scaled workers

## Verification

1. `make build` — compiles with new framework helpers
2. `make vet` — passes
3. `make test-csi-readonly` — N-CSI-01, N-CSI-02, N-CSI-03, N-CSI-10, N-CSI-11 pass against any vSphere cluster
4. `make apply-lab` then `make test-csi` — full suite including provisioning tests
5. Inspect test output for N-CSI-08/09 product gap logs
6. Update test catalog (`docs/tests.md` or equivalent) with all N-CSI-XX IDs

## Adversarial Review Response

### Critical findings — all resolved

| ID | Resolution |
|----|-----------|
| AR-CSI-C1 | N-CSI-02 rewritten as readonly snapshot check. Title changed to "CSI driver pods healthy with current Infrastructure topology". No before/after claim. |
| AR-CSI-C2 | Appendix A added with full providerSpec field mapping table. Preconditions documented. |
| AR-CSI-C3 | N-CSI-08 now parses denial reason to distinguish Machine/CPMS/MachineSet VAP noise from PV-specific denial. Logs specific VAP source when blocked. |
| AR-CSI-C4 | `WaitForPVDeleted` uses `LongTimeout`, polls VolumeAttachment removal, fails loudly with dump. Namespace delete is last and tolerates `Terminating`. |
| AR-CSI-C5 | `DefaultStorageClassName` constant removed. All tests use `GetDefaultStorageClass()` (annotation-based). Mutating tests gate on `requireDefaultStorageClass()`. |

### Major findings — all resolved

| ID | Resolution |
|----|-----------|
| AR-CSI-M1 | N-CSI-03 consolidated — incremental assertions only (CSI config secret probe). Existing `csi_integration_test.go` credential check marked as covered/removed. |
| AR-CSI-M2 | N-CSI-01 supplements CO check. Skip with message if ClusterCSIDriver CRD absent. |
| AR-CSI-M3 | N-CSI-04 now asserts PV topology matches *some* Infrastructure FD. Labeled as smoke. |
| AR-CSI-M4 | New `CloneStorageClassWithTopology()` helper clones default SC parameters (provisioner, reclaimPolicy, parameters), overrides allowedTopologies + name. |
| AR-CSI-M5 | Priority tiers kept as-is: N-CSI-05/08/09 remain P1 because they are the headline tests the user requested. N-CSI-06/07 are P2. |
| AR-CSI-M6 | N-CSI-05/06/08/09 use `Serial` label. Node reuse shortcut added. Lab capacity documented in Appendix A. |
| AR-CSI-M7 | N-CSI-09 title changed to "Confirm vCenter removal blocked by existing xValidation, PV presence irrelevant". Asserts error references FD dependency, logs PV irrelevance. |
| AR-CSI-M8 | Gate check added to N-CSI-03/05/11 when asserting multi-vCenter behavior. |
| AR-CSI-M9 | N-CSI-11 strengthened: when `len(vcenters) >= 2`, requires node in lab FD with matching labels. |
| AR-CSI-M10 | New `pkg/framework/csi.go` file extends `conditions.go` pattern with shared `wait.Poll` style. |

### Minor findings — all addressed

| ID | Resolution |
|----|-----------|
| AR-CSI-N1 | Verification step 6 added: update test catalog. |
| AR-CSI-N2 | `PVTopologyLabels` returns `false` for PVs without nodeAffinity. `ListPVsInFailureDomain` skips them. |
| AR-CSI-N3 | `BusyboxImage` constant added — configurable for disconnected clusters. |
| AR-CSI-N4 | N-CSI-07 asserts `reclaimPolicy: Delete` from SC before proceeding (skip otherwise). |
| AR-CSI-N5 | N-CSI-02 now checks CSI node DaemonSet pods (`CSIDriverNodeLabel`) in addition to controller. |
| AR-CSI-N6 | Label changed from `storage` to `csi`. Makefile targets renamed to `test-csi`/`test-csi-readonly`. |
| AR-CSI-N7 | Deferred — negative path (PVC for non-existent FD) is a stretch goal, not in initial scope. |
| AR-CSI-N8 | Deferred — N-CFG-07 is out of scope for this plan; tracked in coverage-gap-plan.md. |

### Questions answered

1. **N-CSI-02**: Snapshot-only health check. Title drops "after Day 2 add".
2. **CloneMachineSetForFD**: Full field mapping in Appendix A.
3. **MachineSet shortcut**: Yes, N-CSI-05 skips MachineSet create if worker already exists in lab FD.
4. **N-CSI-08 interpretation**: Parses error substring for each known VAP name; unknown denials logged with full error.
5. **Parallelism**: Mutating PVC tests use `Serial` label.
6. **StorageClass discovery**: Annotation-based discovery wins; constant removed.
7. **RBAC**: e2e tests run with `system:admin` kubeconfig — cluster-admin permissions. Documented assumption.
8. **Future VAP**: Same test ID flips behavior — no new spec needed.
9. **CSI node driver**: N-CSI-02 checks both controller and node DaemonSet.
10. **Skip budget**: Not specified — lab-dependent. Tests skip with clear messages; skip rate is observational.
