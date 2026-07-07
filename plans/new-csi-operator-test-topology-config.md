# Plan: New CSI Operator Tests — Topology Config + Synthetic Orphan Tags

## Context

14 of 18 CSI operator lifecycle tests are silently skipped because the `vsphere-failure-domain-in-use-by-machineset` VAP blocks FD removal from Infrastructure. Draining won't work when PVs are topology-constrained. The `skipIfVAPDenied()` pattern masks total coverage loss.

Two independent operator behaviors are currently untested:
1. **ClusterCSIDriver `topologyCategories` configuration** — the pre-Infrastructure path for enabling topology, still functional as a fallback
2. **Orphan tag cleanup** — blocked by VAP when testing via Infrastructure FD removal

We address both with new tests that never touch Infrastructure spec.

---

## Part A: ClusterCSIDriver Topology Configuration Tests

### What this tests

The operator reads topology categories from two sources (`vmware-vsphere-csi-driver-operator/pkg/operator/utils/topology.go:GetTopologyCategories`):
- **Infrastructure CR** (priority): hardcoded `["openshift-zone", "openshift-region"]` when >1 FDs exist
- **ClusterCSIDriver CR** (fallback): `spec.driverConfig.vSphere.topologyCategories` — arbitrary category names

On our cluster (>1 FDs), Infrastructure always wins. But the ClusterCSIDriver field is independently reported via metrics and is the configuration surface documented in OKD docs. These tests verify the operator correctly configured topology and that the precedence model works.

### New file: `test/e2e/csi_topology_config_test.go`

Labels: `"csi-operator"`, `"csi-topology"`, `"readonly"` (except TOPO-06 which is `"mutating"`)

**Tests:**

| Test | What it verifies | Mutating? |
|------|-----------------|-----------|
| TOPO-01 | CSI config secret (`vsphere-csi-config-secret` in `openshift-cluster-csi-drivers`, key `cloud.conf`) has `[Labels]` section with `topology-categories` matching discovered topology keys | No |
| TOPO-02 | `csi-provisioner` container in Deployment `vmware-vsphere-csi-driver-controller` (namespace `openshift-cluster-csi-drivers`) has `--feature-gates=Topology=true` and `--strict-topology` args | No |
| TOPO-03 | ConfigMap `internal-feature-states.csi.vsphere.vmware.com` in `openshift-cluster-csi-drivers` has data key `improved-volume-topology` = `"true"` | No |
| TOPO-04 | CSINode objects have topology keys with `topology.csi.vmware.com/` prefix matching discovered category names | No |
| TOPO-05 | `vsphere_topology_tags{source="infrastructure"}` metric = 2, `vsphere_topology_tags{source="clustercsidriver"}` = 0 (baseline) | No |
| TOPO-06 | Set `spec.driverConfig.vSphere.topologyCategories: ["custom-zone"]` on ClusterCSIDriver → metric `{source="clustercsidriver"}` changes to 1; config secret still shows Infrastructure categories (precedence test). Restore spec field in DeferCleanup. | Yes |

TOPO-01 and TOPO-04 derive expected values from `DiscoverCSITopologyKeys()` / `requireCSITopologyKeys()`, not hardcoded strings.

TOPO-06 does NOT trigger a Deployment rollout because `GetTopologyCategories()` still returns Infrastructure categories (>1 FDs). The metric is updated independently by `UpdateMetrics()` on each sync. The spec field (not an annotation) is restored in DeferCleanup.

### Helpers needed in `pkg/framework/storage.go`

| Function | What it does |
|----------|-------------|
| `GetCSIProvisionerArgs(ctx, client)` | Reads Deployment `vmware-vsphere-csi-driver-controller` in `openshift-cluster-csi-drivers`, finds `csi-provisioner` container, returns its Args slice |
| `GetFeatureConfigMapData(ctx, client)` | Reads ConfigMap `internal-feature-states.csi.vsphere.vmware.com` in `openshift-cluster-csi-drivers`, returns `.Data` map |
| `GetCSINodeTopologyKeys(ctx, client)` | Lists CSINode objects, collects topology keys with `topology.csi.vmware.com/` prefix, returns deduplicated slice |
| `SetClusterCSIDriverTopologyCategories(ctx, operatorClient, categories)` | Patches `spec.driverConfig.vSphere.topologyCategories` on ClusterCSIDriver `csi.vsphere.vmware.com`. Pass nil to clear. |

Constants to add to `pkg/framework/constants.go`:

```go
CSIControllerDeployment    = "vmware-vsphere-csi-driver-controller"
CSIProvisionerContainer    = "csi-provisioner"
FeatureStatesConfigMap     = "internal-feature-states.csi.vsphere.vmware.com"
TopologyTagsMetric         = "vsphere_topology_tags"
TopologyTagsSourceInfra    = "infrastructure"
TopologyTagsSourceCCD      = "clustercsidriver"
```

---

## Part B: Synthetic Orphan Tag Tests

### How it works

Each vCenter has a local-disk datastore (ESXi host local storage) that is NOT referenced by any Infrastructure FD. We attach the cluster tag (`infra.Status.InfrastructureName`) to that datastore via govmomi's `TagManager.AttachTag()`. The operator's `findOrphanedTags()` (`storageclasscontroller/vmware.go:582`) discovers it as an orphan on next sync and cleans it up.

**Verified against operator source:** `findOrphanedTags` at line 641 checks `if !currentFDs.Has(key)` where `key = datacenter/datastore`. It does not distinguish operator-managed from manually-attached tags. A manually tagged non-FD datastore is treated identically to a former-FD datastore. The operator will detach the tag and nothing else — it will NOT create SPBM policies or manage the datastore, because SPBM profile deletion at line 265 only triggers when `len(v.failureDomains) == 0` for a vCenter, which is not the case here (second vCenter still has its FD).

### Lab config addition

Add optional field to `LabConfig` in `pkg/labconfig/config.go`:

```yaml
orphanTest:
  datastore: /wldn-120-DC/datastore/wldn-120-esxi01-local
```

**Required for mutating orphan tests.** Auto-discover (`Finder.DatastoreList("*")` filtering out FD paths) is used only as a diagnostic fallback with loud logging of which datastore was chosen. If neither explicit config nor auto-discover finds a non-FD datastore, tests Skip.

Orphan tests are scoped to the **second vCenter only** (lab config credentials available, lower blast radius than primary).

**Prerequisite:** `make apply-lab` must have run so the cluster tag and category exist on the second vCenter. Document in Makefile target.

### New helpers in `pkg/vsphere/tags.go`

| Function | Purpose |
|----------|---------|
| `AttachTagToDatastore(ctx, session, dsPath, tagName)` | Resolves datastore by path, finds tag by name via `FindTagByName`, calls `TagManager.AttachTag()`. Accepts tag name (not ID) for consistency with `IsDatastoreTagged`. Idempotent. |
| `DetachTagFromDatastore(ctx, session, dsPath, tagName)` | Mirror of attach. `TagManager.DetachTag()`. Idempotent (no-op if already detached). |
| `FindNonFDDatastore(ctx, session, infraFDs)` | `Finder.DatastoreList("*")`, filter out FD `Topology.Datastore` paths, return first non-FD path. Returns `("", false, nil)` if none. Logs which datastore was selected. |

All accept tag name (not ID) and resolve internally, matching the `IsDatastoreTagged` pattern.

### New file: `test/e2e/csi_orphan_tag_test.go`

Labels: `"csi-operator"`, `"csi-orphan"`, `"mutating"` (Serial, Ordered)

**BeforeAll**:
1. `requireGateEnabled()`, `requireLabConfigWithFD()`
2. Connect to second vCenter via lab config credentials
3. Look up cluster tag: `FindTagCategoryByName(ctx, sess, "openshift-"+infraID)` then `FindTagByName(ctx, sess, categoryID, infraID)`. Fail if not found (prerequisite: `make apply-lab`).
4. Find non-FD datastore: use `orphanTest.datastore` from lab config if set, otherwise `FindNonFDDatastore()`. Skip if none found.
5. Verify test datastore is NOT already tagged (clean starting state).

**AfterAll**: Safety-net `DetachTagFromDatastore` if still tagged. Close session.

Shared helpers (`waitForTagDetached`, `waitForOrphanConditionFalse`, metric scraping) are already defined in `csi_operator_lifecycle_test.go` at package level — they're accessible from the new file without extraction since all `_test.go` files in `test/e2e/` share the same package.

**Core orphan detection tests (no PVs, no MachineSet needed):**

| Test | What it verifies | Notes |
|------|-----------------|-------|
| SYNTH-01 | Attach tag → operator detaches within `OperatorSyncTimeout` → `OrphanCleanupPending` stays False → operator healthy | OrphanCleanupPending stays False because no PVs block the detach — `unresolvedOrphans` is empty. |
| SYNTH-02 | Measures wall-clock from attach to detach, asserts < `OperatorSyncTimeout` | Logs actual latency for observability. |
| SYNTH-04 | Scrape before → attach tag → wait for detach → scrape after → `vsphere_csi_orphan_tags_detected_total` increased | Per-test before/after scraping for baseline isolation. |
| SYNTH-05 | Scrape before → attach tag → wait for detach → scrape after → `vsphere_csi_tag_operations_total{operation="detach", result="success"}` increased | Same scraping pattern. |
| SYNTH-09 | Attach tag → wait for detach → storage operator Available=True, not Degraded → StorageClass has `StoragePolicyName` | Verifies no side-effect damage from cleanup. |
| SYNTH-10 | Attach tag, wait for detach. Attach again, wait for detach. Second detach happens within `OperatorSyncTimeout`. | Tests operator handles repeated orphans without getting stuck. Not equivalent to EDGE-02 (which tests Infrastructure-restore re-tag). |

**PV-blocked tests: DEFERRED**

SYNTH-06, SYNTH-07, SYNTH-08 require creating PVCs on a local-disk datastore. This may require a MachineSet with providerSpec targeting the specific ESXi host that owns the local datastore (for `WaitForFirstConsumer` binding). The MachineSet providerSpec details (host affinity, datastore path in workspace) are environment-specific and need a separate lab checklist.

Defer to a follow-up plan once the core orphan tests are validated.

### Cleanup strategy

- Every tag-attach gets a `DeferCleanup` calling `DetachTagFromDatastore`
- AfterAll safety net: detach if still tagged, close session
- govmomi `AttachTag`/`DetachTag` are idempotent — safe to call unconditionally
- The operator itself will also clean up on next sync if test cleanup fails

---

## What's NOT covered

| Behavior | Why not replicated |
|----------|--------------------|
| VC-REM-01/02 (vCenter removal from Infrastructure) | Inherently an Infrastructure mutation |
| EDGE-03 (2→1→2 FD topology transition) | Requires Infrastructure changes |
| FD-REM-02 SPBM verification | SPBM profile deletion only triggers when a vCenter's FD count drops to zero. Synthetic orphans on a vCenter that retains its FD don't cause SPBM transitions. |
| PV-SAFE-01/02/03 (PV-blocked cleanup) | Deferred — requires MachineSet on local-disk ESXi host |
| OBS-03 (PV-blocked metric) | Deferred with PV-blocked tests |

These are gaps, not oversights. The existing tests exercise them when the VAP allows (e.g., cluster without MachineSets in second FD). The SPBM gap is inherent to the synthetic approach — it's a different stimulus.

---

## Files to create/modify

| File | Action |
|------|--------|
| `pkg/vsphere/tags.go` | Add `AttachTagToDatastore`, `DetachTagFromDatastore`, `FindNonFDDatastore` |
| `pkg/framework/storage.go` | Add `GetCSIProvisionerArgs`, `GetFeatureConfigMapData`, `GetCSINodeTopologyKeys`, `SetClusterCSIDriverTopologyCategories` |
| `pkg/framework/constants.go` | Add `CSIControllerDeployment`, `CSIProvisionerContainer`, `FeatureStatesConfigMap`, `TopologyTagsMetric`, `TopologyTagsSourceInfra`, `TopologyTagsSourceCCD` |
| `pkg/labconfig/config.go` | Add optional `OrphanTest` struct with `Datastore` field |
| `test/e2e/csi_topology_config_test.go` | New file — 6 topology configuration tests (TOPO-01 through TOPO-06) |
| `test/e2e/csi_orphan_tag_test.go` | New file — 6 synthetic orphan tests (SYNTH-01, 02, 04, 05, 09, 10) |
| `Makefile` | Add `test-csi-orphan` and `test-csi-topology` targets |

### Makefile targets

```makefile
test-csi-topology:
	$(GINKGO) $(GINKGO_FLAGS) $(GINKGO_REPORT)=csi-topology.xml \
		--label-filter="csi-topology" ./test/e2e/

test-csi-orphan:
	@mkdir -p $(REPORT_DIR)
	E2E_LAB_CONFIG=$(LAB_CONFIG) $(GINKGO) $(GINKGO_FLAGS) $(GINKGO_REPORT)=csi-orphan.xml \
		--label-filter="csi-orphan" ./test/e2e/
```

`test-csi-topology` does NOT require `E2E_LAB_CONFIG` (TOPO-01–05 are readonly, TOPO-06 only needs cluster access).
`test-csi-orphan` requires `E2E_LAB_CONFIG` with `orphanTest.datastore` set.

Both are included in `test-csi-operator` via the `csi-operator` label.

## Verification

1. `make build && make vet`
2. Remote: `make -C ~/Development/testing-day2-vcenter test-csi-topology` — readonly topology tests
3. Remote: `make -C ~/Development/testing-day2-vcenter test-csi-orphan` — synthetic orphan tests
4. Remote: `make -C ~/Development/testing-day2-vcenter test-csi-operator` — full suite confirms coexistence
5. Check JUnit reports: TOPO-* and SYNTH-* tests pass or Skip with descriptive messages (not VAP-masked)
