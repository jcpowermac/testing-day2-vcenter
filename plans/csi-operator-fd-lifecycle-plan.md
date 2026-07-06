# CSI Driver Operator Failure Domain Lifecycle Integration Test Plan

## Context

PR [openshift/vmware-vsphere-csi-driver-operator#348](https://github.com/openshift/vmware-vsphere-csi-driver-operator/pull/348) (branch `multi-vcenter-failure-domain-lifecycle`, commits `acb68c32`, `42ac6c66`) adds failure domain lifecycle support to the CSI driver operator, gated behind `VSphereMultiVCenterDay2`. The changes are:

1. **Orphan tag detection** — `findOrphanedTags()` queries vCenter for datastores tagged with the cluster tag but no longer in the failure domain list
2. **PV-safe tag detach** — `detachOrphanTags()` skips detach when CNS volumes exist on the datastore (`datastoreHasCnsVolumes()`)
3. **SPBM profile cleanup** — `deleteStoragePolicy()` removes orphaned SPBM profiles from vCenters that lose all failure domains
4. **Backoff reset on success** — `syncStoragePolicy()` resets to `defaultBackoff` / `successCheckInterval` (10 minutes) after successful sync (was stuck at 30min cap)
5. **Per-vCenter backoff tracking** — `vCenterBackoffState` map keyed by hostname
6. **OrphanCleanupPending condition** — dedicated operator condition (`VMwareVSphereDriverStorageClassControllerOrphanCleanupPending`) set when PV-blocked orphans exist
7. **Force cleanup annotation** — `csi.vsphere.vmware.com/force-orphan-cleanup: "true"` on ClusterCSIDriver bypasses PV safety
8. **Prometheus metrics** — `TagOperationsTotal`, `OrphanTagsDetectedTotal`
9. **E2E test scaffolding** — `test/e2e/failure_domain_lifecycle_test.go` with `TestFailureDomainRemovalTagCleanup`, `TestPVSafetyBlocksCleanup`, `TestStorageClassSurvivesTopologyTransition`

These tests require a **real multi-vCenter cluster** with at least two vCenters and failure domains backed by actual datastores. All tests mutate Infrastructure and vCenter state — they are not dry-run or mock-based.

## Relationship to Existing Plans

This plan covers **CSI driver operator behavior** during failure domain and vCenter lifecycle events. It complements:

- **ginkgo-test-plan.md** — covers xValidation/VAP admission rules and read-only config checks (different layer)
- **coverage-gap-plan.md** — covers Machine/CPMS/MachineSet integration and credential propagation
- **csi-storage-plan.md** — covers PV provisioning in new FDs and deletion protection probes (N-CSI-04–09); this plan tests the **operator's reconciliation behavior**, not volume provisioning
- **guard-multi-vcenter-fix-vap.md** — single-vCenter guard and VAP dry-run fixes

This plan is the only one that exercises the CSI driver operator's tag management, SPBM profile lifecycle, and condition reporting against real vCenter infrastructure.

---

## Prerequisites

All tests require:

1. OpenShift cluster on vSphere with `VSphereMultiVCenterDay2` feature gate enabled (TechPreview profile)
2. Lab config (`config/lab.yaml`) with `secondVCenter` and `failureDomain` sections populated
3. Two physically distinct **real** vCenters with datastores, datacenters, and compute clusters as described in lab config (vcsim simulators do NOT support CNS, SPBM, or tag lifecycle APIs required by orphan cleanup)
4. Credential secrets for both vCenters present in `kube-system/vsphere-creds`
5. `KUBECONFIG` set to cluster-admin credentials
6. The `testing-day2-vcenter` repo's `pkg/lab` and `pkg/framework` packages available
7. **Direct network connectivity** from the test runner to both vCenter endpoints (port 443) — tag and SPBM verification require govmomi connections; this is a hard prerequisite, not a per-test skip (failing connectivity fails BeforeSuite)
8. **At least one schedulable worker node in the second failure domain** — PV-SAFE tests require Pod scheduling with topology constraints. Either provision via `CloneMachineSetForFD()` (from `csi-storage-plan.md`) in BeforeSuite or document as a lab setup step.
9. **Machine VAP compatibility** — the `vsphere-failure-domain-in-use-by-machine` VAP denies Infrastructure patches that remove FDs still referenced by Machine region/zone labels. FD removal tests must either (a) ensure no Machines carry the target FD's topology labels before patching, or (b) skip with a diagnostic when the VAP denies the patch. BeforeSuite should verify at least one FD can be removed without VAP denial.

### Lab Readiness Checklist

| Check | Command | Expected |
|-------|---------|----------|
| Feature gate enabled | `oc get featuregate cluster -o jsonpath='{.spec.featureSet}'` | `TechPreviewNoUpgrade` |
| Two vCenters in Infrastructure | `oc get infrastructure cluster -o jsonpath='{.spec.platformSpec.vsphere.vCenters[*].server}'` | Two hostnames |
| Second FD exists | `oc get infrastructure cluster -o jsonpath='{.spec.platformSpec.vsphere.failureDomains}'` | 2+ FDs |
| Storage CO healthy | `oc get co storage -o jsonpath='{.status.conditions}'` | Available=True, Degraded=False |
| Node in second FD | `oc get nodes -l topology.kubernetes.io/zone=<second-fd-zone>` | 1+ nodes |
| vCenter connectivity | `curl -sk https://<vcenter1>:443 && curl -sk https://<vcenter2>:443` | Both reachable |
| No Machine VAP conflict | `oc get machines -A -o jsonpath='{range .items[*]}{.metadata.labels.machine\.openshift\.io/region}/{.metadata.labels.machine\.openshift\.io/zone}{"\n"}{end}'` | All labels match an Infrastructure FD |

---

## Test Infrastructure

### Lab Setup Phases

Tests are organized around three cluster states, each built on the previous:

```
State 0: Single vCenter (fresh cluster or after make restore-lab)
    |
    v  make apply-lab
State 1: Two vCenters, second FD added (via lab.Apply)
    |
    v  FD removal tests (in-test)
State 2: Two vCenters, second FD removed (operator reconciles orphan cleanup)
    |
    v  Restore or proceed to vCenter removal
State 3: Second vCenter fully removed (all FDs on second vCenter removed, vCenter entry removed)
    |
    v  make restore-lab
State 0: (restored)
```

### Shared Test Setup (Ginkgo `BeforeSuite`)

```
1. Load lab config from E2E_LAB_CONFIG / config/lab.yaml
2. Initialize framework clients
3. Verify vSphere platform, feature gate enabled
4. Verify default StorageClass exists (via GetDefaultStorageClass(), not hardcoded thin-csi)
5. Snapshot Infrastructure CR (safety net for AfterSuite restore)
6. Verify State 1 via lab.Verify (do NOT run lab.Apply — apply is done externally via `make apply-lab` to avoid conflicting restore mechanisms)
7. If State 1 verification fails, fail BeforeSuite with diagnostic: "Run `make apply-lab` before this suite"
8. Verify direct govmomi connectivity to both vCenters (hard prerequisite — fail, don't skip)
9. Verify at least one FD can be removed without Machine VAP denial (check Machine labels vs FD topology)
10. Record initial ClusterOperator conditions for storage, machine-api, CCM, config-operator
```

### Shared Teardown (`AfterSuite`)

```
1. Restore Infrastructure to BeforeSuite snapshot (safety net for individual test cleanup failures)
2. Wait for all operators healthy
3. Verify Infrastructure matches original snapshot
```

Note: `lab.Restore()` is run externally via `make restore-lab`, not in AfterSuite. This avoids conflicting restore mechanisms (infraBackup vs `.lab-state`). AfterSuite only restores the Infrastructure CR itself.

---

## Test Specifications

### Category 1: Failure Domain Addition — Operator Response

These verify the CSI driver operator correctly responds when a second vCenter and failure domain are added.

**FD-ADD-01: Operator tags new failure domain's datastore after FD addition** [Serial, mutating, p0]
- Precondition: State 1 (lab.Apply completed)
- Action: Read the tag category and tag for the cluster from vCenter (use govmomi tag manager on second vCenter connection)
- Assert:
  - Storage policy tag category `openshift-<infraID>` exists on second vCenter (this is the cluster-scoped category, NOT the topology categories `openshift-region`/`openshift-zone` which are separate)
  - Tag `<infraID>` (the cluster's `InfrastructureName`, no prefix) is attached to the failure domain's datastore
  - Default StorageClass (discovered via `GetDefaultStorageClass()`, not hardcoded `thin-csi`) still exists with `StoragePolicyName` set
- Notes: This is a read-only verification of State 1 operator reconciliation. The actual apply was done in `BeforeSuite`. Tag/category names derived from operator source: category=`fmt.Sprintf("openshift-%s", infraName)`, tag=`infraName` (see `vmware.go` lines 33-34, 94).

**FD-ADD-02: SPBM storage profile exists on second vCenter after FD addition** [Serial, mutating, p0]
- Precondition: State 1
- Action: Connect to second vCenter PBM client, query for storage profile matching `openshift-storage-policy-<infraID>` (derived from `fmt.Sprintf("openshift-storage-policy-%s", infraName)`, `vmware.go` line 34)
- Assert:
  - Profile exists on second vCenter
  - Profile contains tag-based rule referencing the cluster tag (`<infraID>`)
  - Profile on primary vCenter also still exists (not deleted)

**FD-ADD-03: Operator conditions healthy after FD addition** [Serial, mutating, p0]
- Precondition: State 1
- Assert:
  - ClusterOperator `storage` is Available=True, Degraded=False
  - `VMwareVSphereDriverStorageClassControllerOrphanCleanupPending` condition on ClusterOperator `storage` is False (or absent)
  - No operator pod restarts > 2 in `openshift-cluster-csi-drivers` namespace
  - Add `requireClusterCSIDriver()` guard — skip if ClusterCSIDriver CRD is absent

**FD-ADD-04: CSI driver cloud config includes second vCenter** [Serial, mutating, p0]
- Precondition: State 1
- Action: Read CSI driver config (ConfigMap or Secret in `openshift-cluster-csi-drivers`)
- Assert:
  - Config has `[VirtualCenter "<secondVCenter.Server>"]` section (if INI format) or equivalent YAML key
  - Datacenters listed match lab config
  - Primary vCenter section is unchanged

---

### Category 2: Failure Domain Removal — Orphan Tag Cleanup

These are the headline tests for this PR. They exercise the core lifecycle: remove an FD, watch the operator detach the orphaned tag, verify the storage class and SPBM profile survive.

**FD-REM-01: Orphan tag detached after failure domain removal** [Serial, mutating, p0]
- Precondition: State 1 (2 vCenters, FD on each); no Machines with region/zone labels referencing the second FD (Machine VAP would deny patch)
- Setup: Verify tag `<infraID>` is attached to second FD's datastore (govmomi tag check on category `openshift-<infraID>`)
- Action:
  1. Remove the second failure domain from `Infrastructure.spec.platformSpec.vsphere.failureDomains` via merge patch
  2. If patch is denied by Machine VAP, **skip test with diagnostic** (not wait 10 minutes for impossible tag detachment)
  3. Poll for tag detachment on second vCenter (check every 10s, timeout 12 minutes — `successCheckInterval` is 10 minutes)
- Assert:
  - Tag `<infraID>` is **no longer attached** to the removed FD's datastore (govmomi tag check on second vCenter)
  - Tags on remaining (primary) FD's datastores are untouched
  - `OrphanCleanupPending` condition is False on **ClusterOperator `storage`** (condition type: `VMwareVSphereDriverStorageClassControllerOrphanCleanupPending`)
  - Default StorageClass (via `GetDefaultStorageClass()`) still exists with `StoragePolicyName` set
- Cleanup: Restore original Infrastructure (re-add the FD)
- Wait: Operator re-tags the restored FD's datastore

**FD-REM-02: StorageClass and SPBM profile survive FD removal** [Serial, mutating, p0]
- Precondition: State 1; no Machine VAP conflict on target FD
- Action: Remove the second FD (same as FD-REM-01, with VAP-denied skip)
- Assert:
  - Default StorageClass (via `GetDefaultStorageClass()`) exists with same `StoragePolicyName`
  - SPBM profile still exists on primary vCenter
  - If second vCenter still has other FDs: SPBM profile exists there too
  - If second vCenter has zero FDs after removal: SPBM profile **deleted** from second vCenter — BUT only when ALL of: `day2Enabled`, `len(failureDomains)==0` for that vCenter, `globalFDCount > 0`, no orphan detection failures, and no PV-blocked orphans (`len(unresolved)==0`). If any guard fails, profile is retained.
  - Post-removal PVC bind smoke test: create/bind/delete a small PVC on the remaining FD to prove StorageClass is functional (not just present)
- Cleanup: Restore FD

**FD-REM-03: Operator reconciles within backoff window after FD removal** [Serial, mutating, p1]
- Precondition: State 1; no Machine VAP conflict
- Action:
  1. Remove second FD
  2. Start timer
  3. Poll for tag detachment on second vCenter (check every 10s)
- Assert:
  - Tag detached within `successCheckInterval` (10 minutes, per `storageclasscontroller.go` line 48) + operator sync jitter
  - Specifically: tag detached within **12 minutes** (generous bound for 10-min success interval + API latency + informer delay)
- Purpose: Validates the backoff reset fix — if backoff were stuck at 30 min cap, this would timeout. The `defaultBackoff.Duration` is 1 minute (initial *failure* backoff), NOT the success resync interval. After a successful sync, `nextCheck = lastCheck + successCheckInterval` (10 min).
- Note: If a previous test left `pendingOrphans > 0`, the backoff skip is bypassed (`storageclasscontroller.go` line 224) and the operator re-syncs every cycle regardless. Run this test with clean orphan state.
- Cleanup: Restore FD

**FD-REM-04: Multiple FD removal in single patch — all orphan tags cleaned** [Serial, mutating, p2] **DEFERRED**
- Precondition: Cluster has 3+ FDs (if only 2 available, add a third FD on primary vCenter pointing at a different datastore before this test)
- Action: Remove 2 FDs in a single Infrastructure patch
- Assert:
  - Both removed FDs' datastore tags are detached
  - Remaining FD's tags untouched
  - Operator healthy
- Cleanup: Restore all FDs
- **Deferred:** Adding a third FD requires a distinct datastore, region/zone values, VAP-safe patch construction, and Machine topology isolation. Lab config provides one second FD. Implement in a follow-on plan with exact FD spec fields and datastore prerequisites documented.

---

### Category 3: PV Safety — Tag Detach Blocked by CNS Volumes

**PV-SAFE-01: Orphan tag detach blocked when PVs exist on removed FD's datastore** [Serial, mutating, p0]
- Precondition: State 1; at least one schedulable node in the second FD (use `CloneMachineSetForFD()` or require as lab prerequisite); no Machine VAP conflict
- Setup:
  1. Create test namespace (wait for SCC annotation per `CreateTestNamespace` convention)
  2. Create PVC (using default StorageClass via `GetDefaultStorageClass()`) — bind it via a busybox Pod scheduled to a node in the second FD (use nodeSelector with topology labels `topology.kubernetes.io/zone=<second-fd-zone>`)
  3. Verify PVC is Bound and PV exists
  4. Confirm the PV's backing CNS volume is on the second FD's datastore (via PV topology labels matching the FD's region/zone, or direct CNS query)
- Action:
  1. Remove the second FD from Infrastructure (skip with diagnostic if Machine VAP denies)
  2. Wait for operator sync (up to 12 minutes — `successCheckInterval` is 10 min)
- Assert:
  - Tag `<infraID>` is **still attached** to second FD's datastore (orphan not cleaned)
  - `OrphanCleanupPending` condition is True on **ClusterOperator `storage`** (condition type: `VMwareVSphereDriverStorageClassControllerOrphanCleanupPending`) with message containing "orphaned datastore tag(s) blocked by existing PVs"
  - Operator is **not Degraded** — PV blocking is informational, not a failure
  - PVC remains Bound and accessible
- Note: `datastoreHasCnsVolumes()` returns `true` conservatively on any error (CNS client unavailable, login failure, query error). If the test sees "blocked" without real PVs, check the condition message to distinguish "real PVs" from "CNS query failed."
- Cleanup (in order):
  1. Delete Pod
  2. Delete PVC
  3. Wait for PV deletion (reclaimPolicy: Delete)
  4. Restore original Infrastructure FDs
  5. Wait for operator to re-tag and clear OrphanCleanupPending
  6. Delete test namespace

**PV-SAFE-02: Orphan cleanup proceeds after PVs are deleted** [Serial, mutating, p0]
- Precondition: Continuation of PV-SAFE-01 scenario (or standalone with same setup); node in second FD; no Machine VAP conflict
- Setup:
  1. Create PVC+Pod on second FD (same as PV-SAFE-01, using default StorageClass)
  2. Remove second FD → tag blocked (skip if VAP denies)
  3. Verify `OrphanCleanupPending` is True on ClusterOperator `storage`
- Action:
  1. Delete Pod
  2. Delete PVC
  3. Wait for PV deletion (via `reclaimPolicy: Delete`)
  4. Wait for next operator sync cycle (up to 12 minutes — when `pendingOrphans > 0`, backoff skip is bypassed so the operator re-syncs every cycle, but sync cadence is still governed by the controller's resync period)
- Assert:
  - Tag `<infraID>` now detached from second FD's datastore (orphan cleaned)
  - `OrphanCleanupPending` condition cleared to False on ClusterOperator `storage`
- Cleanup: Restore FDs

**PV-SAFE-03: Force cleanup annotation overrides PV safety** [Serial, mutating, p1]
- Precondition: State 1 with PVC bound on second FD; node in second FD; no Machine VAP conflict
- Setup: Same as PV-SAFE-01 (PVC on second FD, FD removed, cleanup blocked)
- Action:
  1. Annotate **ClusterCSIDriver** `csi.vsphere.vmware.com` with `csi.vsphere.vmware.com/force-orphan-cleanup: "true"` (annotation is read from ClusterCSIDriver, NOT Infrastructure — see `storageclasscontroller.go` lines 219-221)
  2. Wait for operator sync (force annotation also bypasses backoff skip, so sync should be faster)
- Assert:
  - Tag `<infraID>` detached from second FD's datastore **despite** PVs existing (operator logs: "Force orphan cleanup enabled via ClusterCSIDriver annotation, skipping PV safety check")
  - `OrphanCleanupPending` condition cleared on ClusterOperator `storage`
  - PVC still Bound (PV not affected — only the vCenter tag was removed)
- Product risk: Force detach removes the tag while PVs remain Bound. The plan does NOT verify post-detach I/O, snapshot, expansion, or re-attach behavior. Document this as a known gap — the PV remains functional for existing workloads but topology-aware operations (migration, expansion to tagged datastore) may behave unexpectedly.
- Cleanup:
  1. Remove force annotation from ClusterCSIDriver
  2. Delete PVC/Pod
  3. Restore FDs

---

### Category 4: vCenter Removal — Full Lifecycle

**VC-REM-01: Complete vCenter removal lifecycle** [Serial, mutating, p0]
- Precondition: State 1 (second vCenter added with FD); no Machines with second FD topology labels; no PVs on second vCenter datastores
- Note: The operator e2e test (`TestVCenterRemovalCleanup`) is explicitly skipped upstream as "internal controller state not observable from e2e." This QE test covers the gap.
- Action (ordered sequence):
  1. Remove all failure domains referencing the second vCenter from Infrastructure (skip with diagnostic if Machine VAP denies)
  2. Wait for operator to detach orphan tags on second vCenter (up to 12 minutes)
  3. Remove the second vCenter from `Infrastructure.spec.platformSpec.vsphere.vCenters`
  4. Wait for operator sync
- Assert after step 2:
  - Tags `<infraID>` detached from all second vCenter datastores
  - SPBM profile `openshift-storage-policy-<infraID>` deleted from second vCenter (zero FDs → `deleteStoragePolicy()` fires, subject to guards in A4)
  - `OrphanCleanupPending` is False on ClusterOperator `storage`
- Assert after step 4:
  - Default StorageClass (via `GetDefaultStorageClass()`) still exists; post-removal PVC bind smoke test passes
  - CSI driver config no longer references second vCenter
  - Storage ClusterOperator healthy
  - Primary vCenter's tags and SPBM profile are intact
  - `StaleVCenterRemoved` event emitted (operator `Sync()` lines 118-146 detects removed vCenters and cleans backoff/orphan state)
- Cleanup: Full `lab.Restore()` back to State 0, then re-apply to State 1

**VC-REM-02: CSI driver config updated after vCenter removal** [Serial, mutating, p0]
- Precondition: Same sequence as VC-REM-01
- Action: After both FD and vCenter removal, read CSI driver config
- Assert:
  - Config has only one `[VirtualCenter]` section (primary)
  - No reference to removed vCenter's hostname
  - Datacenters reflect only primary vCenter's topology
- Cleanup: Restore

**VC-REM-03: Credential secrets cleaned after vCenter removal** [Serial, mutating, p2] **OBSERVATIONAL ONLY**
- Precondition: Same sequence as VC-REM-01
- Action: After vCenter removal, read credential secrets
- Assert: **Log observed behavior, do not hard-assert** — credential cleanup is CCO's responsibility (see `feedback_cco_secrets` memory), not the CSI operator. Skip with informational message if credential keys are still present.
  - Log whether `kube-system/vsphere-creds` still has `<secondVCenter.Server>.username` / `.password` keys
  - Log whether CCO has reconciled `openshift-machine-api/vsphere-cloud-credentials`
  - Never write to CCO-managed secrets
- Note: Demoted from P1 to P2. The test documents behavior but cannot fail meaningfully when the responsible controller is ambiguous.
- Cleanup: Restore

---

### Category 5: Operator Resilience and Edge Cases

**EDGE-01: Operator recovers from temporary vCenter connectivity loss during cleanup** [Serial, mutating, p2] **DEFERRED — MANUAL ONLY**
- Precondition: State 1 with second FD
- Action:
  1. Remove second FD from Infrastructure
  2. Before operator syncs, block connectivity to second vCenter (firewall rule or DNS override) — **manual step, or skip with note**
  3. Wait for operator sync — expect orphan detection to fail gracefully
  4. Restore connectivity
  5. Wait for next sync
- Assert:
  - Operator did not crash or degrade on connectivity failure
  - After connectivity restored, orphan tag is cleaned
- **Deferred:** Requires firewall/DNS control not available in automated lab. Default to `Skip("EDGE-01 requires manual network partition — run manually with lab infrastructure control")`. Excluded from automated spec count.

**EDGE-02: Backoff resets after successful sync following failures** [Serial, mutating, p1]
- Precondition: State 1
- Action:
  1. Observe current sync timing (operator logs or metric scrape)
  2. Cause a transient failure (e.g., temporarily invalid credential, or remove FD with connectivity issue)
  3. After failure, restore valid state
  4. Measure time to next successful sync
- Assert:
  - After recovery, sync interval returns to ~1 minute (not stuck at 30 min)
  - `TagOperationsTotal` metric increments on next successful tag attach
- Note: May require log/metric scraping. Document expected log patterns.

**EDGE-03: Topology transition — 2 FDs to 1 FD and back** [Serial, mutating, p1]
- Precondition: State 1
- Action:
  1. Remove second FD → 1 FD remains
  2. Wait for operator reconcile
  3. Verify topology state (CSI topology feature config)
  4. Re-add second FD → 2 FDs
  5. Wait for operator reconcile
- Assert:
  - After step 2: topology still enabled (1 FD still has topology)
  - After step 4: tag re-attached to second FD's datastore, SPBM profile re-created on second vCenter
  - StorageClass persists through both transitions

---

### Category 6: Metrics and Observability

**OBS-01: OrphanTagsDetectedTotal metric incremented on FD removal** [Serial, mutating, p1]
- Precondition: State 1
- Action:
  1. Scrape `vsphere_csi_orphan_tags_detected_total` from operator metrics endpoint (port 8443 or via `oc exec` into operator pod)
  2. Remove second FD
  3. Wait for operator sync
  4. Re-scrape metric
- Assert:
  - Metric value increased by at least 1
- Cleanup: Restore FD

**OBS-02: TagOperationsTotal metric tracks detach operations** [Serial, mutating, p1]
- Precondition: State 1
- Action: Same as OBS-01
- Assert:
  - `vsphere_csi_tag_operations_total{operation="detach",result="success"}` increased
- Cleanup: Restore FD

**OBS-03: TagOperationsTotal metric tracks PV-blocked skips** [Serial, mutating, p1]
- Precondition: State 1 with PVC on second FD
- Action: Remove second FD while PVC exists
- Assert:
  - `vsphere_csi_tag_operations_total{operation="skip",result="pv_blocked"}` increased
  - `vsphere_csi_orphan_tags_detected_total` increased
- Cleanup: Delete PVC, restore FD

---

## Framework Helpers Needed

### vCenter Tag Verification (`pkg/vsphere/tags.go`)

| Function | Purpose |
|----------|---------|
| `IsDatastoreTagged(ctx, vcenterConn, datastoreMOR, tagName)` | Check if a specific tag is attached to a datastore |
| `GetClusterTagName(infraID)` | Derive storage policy tag name (just `infraID` — no prefix, no zone suffix) |
| `GetClusterTagCategoryName(infraID)` | Derive storage policy tag category name (`"openshift-" + infraID`) |
| `ConnectToVCenter(ctx, server, username, password)` | Create govmomi connection for tag verification |

**Important:** The storage policy tag category is `openshift-<infraID>` and the tag is `<infraID>`. These are distinct from the topology tag categories `openshift-region` and `openshift-zone` (defined in `utils/topology.go`). Do not confuse them.

### SPBM Profile Verification (`pkg/vsphere/spbm.go`)

| Function | Purpose |
|----------|---------|
| `StorageProfileExists(ctx, vcenterConn, profileName)` | Check if SPBM profile exists on a vCenter |
| `GetStorageProfileName(infraID)` | Derive profile name from cluster infra ID |

### CSI Driver Config Reading (`pkg/framework/csi.go`)

| Function | Purpose |
|----------|---------|
| `GetCSIDriverConfig(ctx, kubeClient)` | Read CSI driver cloud config from operator namespace |
| `CSIConfigHasVCenter(config, vcenterHost)` | Check if config references a vCenter |

### Metric Scraping (`pkg/framework/metrics.go`)

| Function | Purpose |
|----------|---------|
| `ScrapeOperatorMetric(ctx, kubeClient, metricName)` | Read a Prometheus metric from CSI operator pod |
| `ScrapeOperatorCounterByLabels(ctx, kubeClient, metricName, labels)` | Read counter metric with label filter |

---

## Test Classification Summary

| ID | Mutating | Serial | Priority | Resources Created | vCenter API Calls |
|----|----------|--------|----------|-------------------|-------------------|
| FD-ADD-01 | yes (via BeforeSuite) | yes | P0 | none | tag read |
| FD-ADD-02 | yes (via BeforeSuite) | yes | P0 | none | PBM query |
| FD-ADD-03 | yes (via BeforeSuite) | yes | P0 | none | none |
| FD-ADD-04 | yes (via BeforeSuite) | yes | P0 | none | config read |
| FD-REM-01 | yes | yes | P0 | none | tag read |
| FD-REM-02 | yes | yes | P0 | none | PBM query |
| FD-REM-03 | yes | yes | P1 | none | tag read |
| FD-REM-04 | yes | yes | P2 (DEFERRED) | FD (temp) | tag read |
| PV-SAFE-01 | yes | yes | P0 | PVC, Pod, NS | tag read |
| PV-SAFE-02 | yes | yes | P0 | PVC, Pod, NS | tag read |
| PV-SAFE-03 | yes | yes | P1 | PVC, Pod, NS | tag read |
| VC-REM-01 | yes | yes | P0 | none | tag read, PBM |
| VC-REM-02 | yes | yes | P0 | none | config read |
| VC-REM-03 | yes | yes | P2 (OBSERVATIONAL) | none | secret read |
| EDGE-01 | yes | yes | P2 (DEFERRED) | none | tag read |
| EDGE-02 | yes | yes | P1 | none | metric scrape |
| EDGE-03 | yes | yes | P1 | none | tag read, PBM |
| OBS-01 | yes | yes | P1 | none | metric scrape |
| OBS-02 | yes | yes | P1 | none | metric scrape |
| OBS-03 | yes | yes | P1 | PVC, Pod, NS | metric scrape |

**Total: 20 test specs** (10 P0, 6 P1, 4 P2/deferred/stretch)

Active automated specs: 17 (excludes FD-REM-04, EDGE-01, VC-REM-03 which are deferred/observational)

---

## Execution Order

Tests must run in this order due to Infrastructure state dependencies:

```
Phase A: FD Addition verification (reads State 1 from BeforeSuite)
  FD-ADD-01 → FD-ADD-02 → FD-ADD-03 → FD-ADD-04

Phase B: FD Removal (each test restores after mutation)
  FD-REM-01 → FD-REM-02 → FD-REM-03
  (FD-REM-04 DEFERRED — requires 3+ FDs not available in standard lab)

Phase C: PV Safety (each test creates PVC, removes FD, verifies, cleans up)
  PV-SAFE-01 → PV-SAFE-02 → PV-SAFE-03

Phase D: Full vCenter removal (longer sequence, restore at end)
  VC-REM-01 → VC-REM-02
  (VC-REM-03 observational only — logs CCO behavior, does not hard-assert)

Phase E: Edge cases and observability (independent, each restores)
  EDGE-02 → EDGE-03 → OBS-01 → OBS-02 → OBS-03

Phase F: Manual/deferred (Skip by default)
  EDGE-01 (requires manual network partition)
  FD-REM-04 (requires 3+ FDs)
```

Use `Ordered` Ginkgo containers for phases B, C, D. Use Ginkgo's `Serial` decorator (not a label string `"Serial"`) on all containers — only `Ordered` enforces execution order.

---

## Cleanup Strategy

Every test that mutates Infrastructure uses `DeferCleanup`:

1. **Infrastructure patch restore** — re-apply original FDs/VCenters via merge patch
2. **PVC/Pod cleanup** — delete in reverse dependency order (Pod → PVC → wait for PV deletion)
3. **Annotation cleanup** — remove force-cleanup annotation from ClusterCSIDriver
4. **Namespace cleanup** — delete test namespace
5. **Operator health gate** — after every restore, `waitForOperatorHealthy()` blocks until `storage` CO is Available

`AfterSuite` runs `lab.Restore()` as a safety net — even if individual test cleanups fail, the cluster is restored to State 0.

---

## vCenter Connection Requirements

Tests that verify tag state or SPBM profiles need direct govmomi connections to both vCenters. These connections:

- Use credentials from lab config (same as `pkg/lab/apply.go` uses)
- Are read-only (tag queries, PBM queries) — never modify vCenter state directly
- Are created per-test and logged out in `DeferCleanup`
- Require network connectivity from the test runner to both vCenter endpoints (port 443)

Direct vCenter connectivity is a **hard prerequisite**. If connectivity fails, BeforeSuite fails the entire suite with a diagnostic message. Tag and SPBM verification are the core assertions — falling back to condition-only checks would gut the test's value.

---

## Ginkgo Labels

```
Labels (Ginkgo Label() decorator):
  "csi-operator"   — all tests in this plan
  "fd-lifecycle"   — failure domain add/remove tests
  "pv-safety"      — PV-blocked cleanup tests
  "vcenter-removal"— full vCenter removal lifecycle
  "observability"  — metrics and condition verification
  "p0"             — P0 priority
  "p1"             — P1 priority
  "p2"             — P2 priority
  "mutating"       — all tests (all mutate Infrastructure)

Decorators (NOT labels — Ginkgo label filters don't enforce these):
  Serial           — Ginkgo Serial decorator, prevents parallel execution
  Ordered          — Ginkgo Ordered decorator on phase containers (B, C, D), enforces execution order
```

Run examples:
- `ginkgo --label-filter="csi-operator && p0"` — P0 lifecycle tests only
- `ginkgo --label-filter="fd-lifecycle"` — FD add/remove only
- `ginkgo --label-filter="pv-safety"` — PV safety tests only
- `ginkgo --label-filter="csi-operator"` — full suite

---

## CI Integration

| Tier | Filter | When | Cluster |
|------|--------|------|---------|
| Nightly | `csi-operator && p0` | Nightly on dedicated multi-vCenter lab | Requires 2 vCenters + lab config |
| Weekly | `csi-operator` | Weekly full run | Same lab |
| Pre-release | all | Before payload sign-off | Dedicated lab |

These tests **cannot** run in PR CI (require real multi-vCenter infrastructure). They are nightly/periodic only.

### Makefile Target

Add `test-csi-operator` target (mirrors `test-storage` pattern):

```makefile
test-csi-operator:
	$(GINKGO) $(GINKGO_FLAGS) --label-filter="csi-operator" \
		--junit-report=reports/csi-operator.xml ./test/e2e/
```

Add to `test-e2e` as a phase after `test-storage` (Phase 5.5 or Phase 6, before restore).

### Framework File Ownership

Both this plan and `csi-storage-plan.md` introduce `pkg/framework/csi.go`. To avoid merge conflicts:

1. Implement shared framework (`pkg/framework/csi.go`, `storage.go`, `metrics.go`) in one PR first
2. `pkg/vsphere/tags.go` and `spbm.go` are net-new govmomi helpers — no precedent in this codebase beyond `cloud_config.go` and `session.go`
3. Coordinate with `csi-storage-plan.md` on `CloneMachineSetForFD` reuse

---

## Mapping to PR Code Changes

| PR Code Change | Tests That Exercise It |
|----------------|----------------------|
| `findOrphanedTags()` | FD-REM-01, FD-REM-04, PV-SAFE-01, VC-REM-01 |
| `detachOrphanTags()` | FD-REM-01, FD-REM-04, PV-SAFE-02, VC-REM-01 |
| `datastoreHasCnsVolumes()` | PV-SAFE-01, PV-SAFE-02, PV-SAFE-03 |
| `deleteStoragePolicy()` | FD-REM-02, VC-REM-01 |
| Backoff reset (`defaultBackoff` on success) | FD-REM-03, EDGE-02 |
| `vCenterBackoffState` per-hostname | FD-REM-03 (implicit) |
| `OrphanCleanupPending` condition | PV-SAFE-01, PV-SAFE-02, PV-SAFE-03, FD-REM-01, OBS-03 |
| Force cleanup annotation | PV-SAFE-03 |
| `TagOperationsTotal` metric | OBS-02, OBS-03 |
| `OrphanTagsDetectedTotal` metric | OBS-01, OBS-03 |
| `updateConditions()` orphan path | PV-SAFE-01, PV-SAFE-02, PV-SAFE-03 |
| CSI config generation (multi-VC) | FD-ADD-04, VC-REM-02 |
| Stale vCenter state cleanup (`Sync()` lines 118-146) | VC-REM-01 (event check) |

---

## Known Gaps / Out of Scope

| Gap | Reason | Tracked By |
|-----|--------|------------|
| Multi-vCenter connection manager (`VSphereConnectionManager`) | Not in this PR — Phase 2 of the operator plan | `vcenter-failure-domain-lifecycle-plan.md` Phase 2 |
| Per-vCenter storage profile replication | Not in this PR — Phase 2.3 | Same |
| Node topology awareness for orphan cleanup | Not in this PR — Phase 3B.2 | Same |
| Failure domain consistency checker | Not in this PR — Phase 4 | Same |
| Cross-vCenter VM lookup for NodeChecker | Not in this PR — Phase 4.4 | Same |
| Credential cleanup after vCenter removal | CCO responsibility, not CSI operator | Document behavior in VC-REM-03 |
| CSI driver per-vCenter credentials | Upstream CSI driver dependency | Phase 2.6 exit criterion |
| Topology mode transition (2 FD → 0 FD) | Code exists but needs manual verification — library-go behavior confirmed correct in plan | EDGE-03 covers 2→1→2 |

---

## Verification Checklist (for implementing agent)

1. [ ] `go build ./...` — compiles with new framework helpers
2. [ ] `go vet ./...` — passes
3. [ ] `ginkgo --dry-run --label-filter="csi-operator" ./test/e2e/` — test tree renders correctly
4. [ ] On multi-vCenter lab: `make test-csi-operator GINKGO_FLAGS="-v --label-filter='csi-operator && p0'"` — **10** P0 tests pass
5. [ ] On multi-vCenter lab: `make test-csi-operator` — full suite passes (17 active specs)
6. [ ] Verify tag state via govmomi after each FD-REM test — tags actually detached on vCenter
7. [ ] Verify PV-SAFE tests leave PVCs intact — no data loss
8. [ ] Verify `make restore-lab` works after VC-REM tests — cluster returns to State 0
9. [ ] Update `docs/tests.md` with new test IDs (use existing N-* namespace or dedicated FD-* namespace — decide before implementation)

---

## Adversarial Review — Comments and Questions

### Resolutions

All findings below were cross-referenced against the actual operator code in [PR #348](https://github.com/openshift/vmware-vsphere-csi-driver-operator/pull/348). Resolutions are marked inline.

### Critical

**C1 — Infrastructure patches may never reach the CSI operator** — **RESOLVED: added VAP guards**

Added BeforeSuite VAP compatibility check, per-test skip-with-diagnostic when Machine VAP denies patch, and Lab Readiness Checklist verifying Machine labels match FDs before suite runs.

---

**C2 — P0 count and verification checklist are wrong** — **RESOLVED: corrected to 10 P0**

Classification table confirmed 10 P0 specs. Summary and verification checklist corrected.

---

**C3 — BeforeSuite `lab.Apply` conflicts with existing suite lifecycle** — **RESOLVED: suite is verify-only**

BeforeSuite now calls `lab.Verify` only, not `lab.Apply`. Apply is done externally via `make apply-lab`. AfterSuite restores Infrastructure snapshot only, not `lab.Restore()`. Workflow: `make apply-lab` → `make test-csi-operator` → `make restore-lab`.

---

**C4 — PV-SAFE tests require a schedulable node in the lab FD** — **RESOLVED: added prerequisite**

Added node-in-FD as prerequisite #8. PV-SAFE tests should reuse `CloneMachineSetForFD()` from `csi-storage-plan.md` or require as documented lab setup step.

---

### Major

**M1 — Hardcoded `thin-csi`** — **RESOLVED: replaced with `GetDefaultStorageClass()`**

All `thin-csi` references replaced with annotation-based SC discovery.

---

**M2 — Overlap and ownership vs `csi-storage-plan.md` and operator e2e** — **ACKNOWLEDGED, unresolved**

Overlap exists. Operator repo tests (`failure_domain_lifecycle_test.go`) are upstream unit/e2e; this repo tests are QE integration with real multi-vCenter infrastructure. The operator's `TestVCenterRemovalCleanup` is explicitly skipped upstream — this plan covers that gap. Duplicate maintenance risk remains when metric names or condition types change. Pin operator commit references in constants.

---

**M3 — `OrphanCleanupPending` location** — **RESOLVED by code verification**

Condition lives on the **ClusterOperator** `storage`, pushed via `v1helpers.UpdateStatus()`. Exact type: `VMwareVSphereDriverStorageClassControllerOrphanCleanupPending`. All test specs updated to specify ClusterOperator `storage` as the canonical source.

---

**M4 — FD-REM-04 underspecified** — **RESOLVED: deferred to P2**

Demoted to P2 (DEFERRED). Requires follow-on plan with exact FD spec fields and datastore prerequisites.

---

**M5 — "StorageClass still functional" undefined** — **RESOLVED: added PVC smoke test**

FD-REM-02 and VC-REM-01 now include post-removal PVC bind smoke test.

---

**M6 — vCenter connectivity fallback** — **RESOLVED: hard prerequisite**

Changed to BeforeSuite failure, not per-test skip. Updated vCenter Connection Requirements section.

---

**M7 — No Makefile target** — **RESOLVED: added `test-csi-operator`**

Added Makefile target section with `test-csi-operator` and `test-e2e` phase wiring.

---

**M8 — Framework file conflicts** — **RESOLVED: documented ownership**

Added Framework File Ownership section. Implement shared framework in one PR first.

---

**M9 — Timeouts inconsistent** — **RESOLVED: corrected to 12 minutes**

`successCheckInterval` is 10 minutes (not 1 minute). All timeouts corrected. FD-REM-03 bound changed from 3 minutes to 12 minutes. FD-REM-01 uses tag-detach poll (same approach as FD-REM-03).

---

**M10 — Force cleanup data plane** — **RESOLVED: added product risk note**

PV-SAFE-03 now documents the product risk: force detach leaves PV Bound but topology-aware operations may behave unexpectedly. No post-detach I/O probe — documented as known gap.

---

### Minor

**N1 — Ginkgo `Serial` vs label** — **RESOLVED**

Labels section corrected: `Serial` and `Ordered` are decorators, not labels. Ginkgo label filters don't enforce them.

---

**N2 — Tag naming conventions wrong** — **RESOLVED: corrected from operator source**

Category is `openshift-<infraID>` (not `openshift-<infraID>-region`/`-zone`). Tag is `<infraID>` (not `openshift-<infraID>-<zone>`). The `-region`/`-zone` categories are separate topology categories. All test specs and framework helpers updated.

---

**N3 — Metric names and labels** — **ACKNOWLEDGED**

Metric names confirmed from operator source (`utils/metric.go`):
- `vsphere_csi_tag_operations_total` with labels `operation` (`attach`/`detach`/`skip`) and `result` (`success`/`error`/`pv_blocked`)
- `vsphere_csi_orphan_tags_detected_total` (no labels, plain counter)

Pin in `pkg/framework/constants.go` with reference to operator commit `acb68c32`.

---

**N4 — EDGE-01 lab-destructive** — **RESOLVED: deferred with Skip default**

EDGE-01 marked as DEFERRED with default `Skip` message. Excluded from automated spec count.

---

**N5 — VC-REM-03 scope ambiguity** — **RESOLVED: demoted to P2 observational**

VC-REM-03 demoted from P1 to P2, made observational-only. Logs CCO behavior, does not hard-assert.

---

**N6 — Simulator prerequisite is misleading** — **RESOLVED: restricted to real vCenter**

Prerequisites updated to require real vCenters only. vcsim does not support CNS, SPBM, or tag lifecycle APIs.

---

**N7 — Missing ClusterCSIDriver CRD guard** — **ACKNOWLEDGED**

Add `requireClusterCSIDriver()` skip helper for PV-SAFE-03 (annotation) and condition checks. Follow `csi-storage-plan.md` pattern.

---

**N8 — Test catalog and docs** — **ACKNOWLEDGED**

Decide ID namespace before implementation. Verification checklist updated with note.

---

### Questions for the author (with resolutions where applicable)

1. **Admission vs operator:** Real FD removal is the correct test approach since there is no PV VAP. Machine VAP conflict resolved via BeforeSuite check and per-test skip-with-diagnostic. Tests verify operator behavior, not admission behavior.

2. **Lab state contract:** **RESOLVED.** Added Lab Readiness Checklist table in Prerequisites. Pre-run state is State 1 from `make apply-lab` with verified node in second FD.

3. **Operator version pin:** Still open. Tests must run against a payload containing PR #348 commits (`acb68c32`, `42ac6c66`). Older operators will false-negative on conditions and metrics. Add version check in BeforeSuite if feasible.

4. **Combined suite run:** Still open. PV-SAFE and N-CSI-05/08 cannot run concurrently — both mutate the Infrastructure singleton. Use `Serial` decorator on all containers. Label filter should prevent accidental overlap: `csi-operator` vs `storage`.

5. **Restore ordering after PV-SAFE-03:** Still open. Force cleanup leaves a Bound PV on a datastore whose FD was removed. Restore FD → re-tag → clear annotation sequence needs explicit verification that operator re-tags successfully before next test.

6. **Phase D vs Phase B:** Still open. Recommend Phase D in its own `Ordered` container with a `BeforeEach` State 1 gate — if Phase B cleanup fails, Phase D detects the broken state and skips with diagnostic rather than running against partial state.

7. **Relationship to topology_lifecycle_test.go:** Keep separate. Real-vCenter CSI operator tests use `requireLabConfig()` guard, mirroring `topology_lifecycle_test.go` line 82–84 skip pattern. Fake-vCenter tests (`temp-vcenter-e2e.example.com`) test different code paths.

---

### Additional Findings from Code Review

**A1 — Tag naming convention was WRONG** — **FIXED**

Plan originally said category `openshift-<infraID>-region`/`-zone` and tag `openshift-<infraID>-<zone>`. Actual code (`vmware.go` lines 33-34, 94): category=`openshift-<infraID>`, tag=`<infraID>`. The `-region`/`-zone` categories are topology categories from `utils/topology.go`, separate from the storage policy tag. All specs corrected.

---

**A2 — `successCheckInterval` was wrong** — **FIXED**

Plan said 1-minute sync interval. Actual code: `successCheckInterval = 10 * time.Minute` (`storageclasscontroller.go` line 48). `defaultBackoff.Duration` (1 minute) is the initial *failure* backoff, not the success resync interval. FD-REM-03 timeout corrected from 3 minutes to 12 minutes.

---

**A3 — Force cleanup annotation location confirmed**

Annotation is read from ClusterCSIDriver (`storageclasscontroller.go` lines 219-221), not Infrastructure. Plan was correct; all references now consistently say ClusterCSIDriver.

---

**A4 — `deleteStoragePolicy()` has additional guards**

Policy deletion requires ALL of: `day2Enabled`, `len(failureDomains)==0` for vCenter, `globalFDCount > 0`, `policyCreated`, no orphan detection failures, `len(unresolved)==0`. FD-REM-02 updated to document these guards.

---

**A5 — `datastoreHasCnsVolumes()` is conservatively safe**

Returns `true` (blocks cleanup) on any error: CNS client unavailable, login failure, query error. PV-SAFE-01 updated with note about distinguishing "real PVs" from "CNS query failed" via condition message.

---

**A6 — Stale vCenter cleanup untested**

`Sync()` lines 118-146 detect and remove state for vCenters no longer in active connections, emitting `StaleVCenterRemoved` events. Added to VC-REM-01 assertions and PR code mapping table.

---

**A7 — Operator e2e `TestVCenterRemovalCleanup` is explicitly skipped upstream**

Skipped with: "vCenter removal cleanup is internal controller state not observable from e2e." This QE plan covers the gap. Noted in VC-REM-01.

---

**A8 — `pendingOrphans > 0` bypasses backoff skip**

At `storageclasscontroller.go` line 224, the backoff skip is bypassed when `pendingOrphans > 0`. After PV-SAFE-01 sets OrphanCleanupPending, subsequent syncs are not skipped by backoff. FD-REM-03 updated with note about clean orphan state prerequisite.

---

### Summary

The plan has been updated with all corrections from code review against [PR #348](https://github.com/openshift/vmware-vsphere-csi-driver-operator/pull/348). Key fixes: tag naming conventions corrected, `successCheckInterval` corrected to 10 minutes, P0 count corrected to 10, BeforeSuite changed to verify-only, Machine VAP guards added, vCenter connectivity made a hard prerequisite, `thin-csi` replaced with `GetDefaultStorageClass()`, FD-REM-04 and EDGE-01 deferred, VC-REM-03 made observational. Remaining open items: operator version pin, combined suite concurrency, Phase D state isolation, test ID namespace.
