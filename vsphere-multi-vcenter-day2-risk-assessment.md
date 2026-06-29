# QE/QA Risk Assessment: vSphere Multi-vCenter Day 2 Feature

**Feature**: vSphere Multi-vCenter Day 2 (SPLAT initiative)  
**Author**: Neil Girard (vr4manta) across all PRs  
**Scope**: 8 PRs across 5 repositories, all gated behind the `VSphereMultiVCenterDay2` feature gate  
**Assessment Date**: 2026-06-23

---

## Executive Summary

These PRs collectively implement Day 2 lifecycle management for multiple vCenters on vSphere. The changes touch API definitions, CRD validation, cloud config ownership migration between operators (CCO -> CCCMO), installer config generation, and admission policy enforcement. **This is a high-risk feature set** due to cross-operator coordination, ownership migration of a critical ConfigMap, and changes to CRD validation rules that govern cluster infrastructure.

---

## PR-by-PR Risk Assessment

### 1. `openshift/api#2783` - Feature Gate Definition | Risk: **LOW**

- **PR**: https://github.com/openshift/api/pull/2783
- **JIRA**: [SPLAT-2664](https://redhat.atlassian.net/browse/SPLAT-2664)
- **What**: Adds `VSphereMultiVCenterDay2` feature gate to `features.go` and all feature gate payload manifests.
- **Status**: MERGED (2026-03-30)
- **Stats**: +44 / -3, 11 files
- **Risk factors**: None - pure additive, no behavioral change on its own.
- **QE needs**: Verify the gate appears in `FeatureGate/cluster` status under the correct profiles (DevPreview, TechPreview, etc.).

---

### 2. `openshift/api#2784` - CRD xValidation Rules for Day 2 | Risk: **HIGH**

- **PR**: https://github.com/openshift/api/pull/2784
- **JIRA**: [SPLAT-2649](https://redhat.atlassian.net/browse/SPLAT-2649)
- **What**: Modifies Infrastructure CRD xValidation rules to allow Day 2 vCenter mutations (add/remove vCenters, modify failure domains) when the feature gate is enabled. Changes the existing ungated validation to be feature-gate-aware with `featureGate=""` so the old restriction stays off by default.
- **Status**: MERGED (2026-04-30)
- **Stats**: +7827 / -628, 55 files (mostly generated CRD manifests and test fixtures)
- **Key changes**:
  - Removes hard `minItems: 1` constraint on vCenters (ratcheting: allows empty persisted values to pass validation)
  - Adds new `VSphereMultiVCenterDay2.yaml` test files with 1067+ lines of test cases
  - Prevents vCenter server address swapping (you can add/remove but not change an existing entry's server)
  - Also touches `machineconfiguration/v1` ControllerConfig CRDs
- **Risk factors**:
  - **CRD validation regressions**: Changes to xValidation rules can inadvertently block legitimate updates or allow invalid ones. The ratcheting behavior (allowing empty persisted vCenters arrays) is nuanced.
  - **Cross-CRD consistency**: Both `infrastructures.config.openshift.io` and `controllerconfigs.machineconfiguration.openshift.io` must have matching validation logic.
  - **Upgrade path**: Clusters upgrading with existing (possibly invalid) vCenter configurations must not be blocked by new validation rules.
- **QE focus areas**:
  - Test adding a second vCenter to a running single-vCenter cluster
  - Test removing a vCenter that is no longer in use
  - Verify that existing single-vCenter clusters upgrading to this version are not broken
  - Test that removing a vCenter still referenced by failure domains is blocked
  - Test ratcheting: clusters with empty vCenters array (edge case) can still update other fields

---

### 3. `openshift/cluster-cloud-controller-manager-operator#442` - Cloud Config Ownership Migration | Risk: **CRITICAL**

- **PR**: https://github.com/openshift/cluster-cloud-controller-manager-operator/pull/442
- **JIRA**: [SPLAT-2651](https://redhat.atlassian.net/browse/SPLAT-2651)
- **What**: CCCMO takes ownership of `openshift-config-managed/kube-cloud-config` ConfigMap from CCO for vSphere when the feature gate is enabled. Includes INI-to-YAML conversion, Infrastructure-derived value injection, and new RBAC (create/update/patch on ConfigMaps in `openshift-config-managed`).
- **Status**: MERGED (2026-05-05)
- **Stats**: +26507 / -2109, 88 files (bulk is vendor updates and generated CRD manifests)
- **Key changes**:
  - New `shouldManageManagedConfigMap()` gated on `VSphereMultiVCenterDay2`
  - New `syncManagedCloudConfig()` method creates/updates `kube-cloud-config` in `openshift-config-managed`
  - Refactored `syncCloudConfigData()` - removed equality check for target CM (always updates)
  - RBAC expansion for CCCMO service account
  - Deleted `vsphere_cloud_config/config_test.go` (moved to library-go)
- **Risk factors**:
  - **Ownership migration gap**: If CCCMO and CCO disagree on who owns the ConfigMap, it can lead to config thrashing or stale config. This must be coordinated with PR #5 (CCO side).
  - **INI-to-YAML conversion correctness**: Any mismatch between how CCCMO generates YAML and what consumers expect will break cloud provider functionality.
  - **RBAC escalation**: New create/update/patch verbs on ConfigMaps in `openshift-config-managed` - verify least privilege.
  - **Always-update behavior**: The target CM (`cloud-conf` in `openshift-cloud-controller-manager`) is now always updated without equality check, which could cause unnecessary reconciliation loops downstream.
  - **Minimal config bootstrap**: When no source config exists, CCCMO creates a minimal `global: {}\nvcenter: {}\n` - verify this is sufficient for all downstream consumers.
- **QE focus areas**:
  - **Migration scenario**: Enable the feature gate on a running cluster and verify the ConfigMap transitions smoothly from CCO-managed to CCCMO-managed without disruption
  - **Dual-operator behavior**: Verify CCO stops writing when the gate is on and CCCMO starts
  - **Config content validation**: Compare the CCCMO-generated config against what CCO produced - ensure no functional differences
  - **Fresh install with gate enabled**: Verify the ConfigMap is created correctly on a new cluster
  - **Rollback scenario**: What happens if the feature gate is disabled after migration?

---

### 4. `openshift/cluster-cloud-controller-manager-operator#469` - vCenter Cleanup Bug Fix | Risk: **LOW**

- **PR**: https://github.com/openshift/cluster-cloud-controller-manager-operator/pull/469
- **JIRA**: [SPLAT-2792](https://redhat.atlassian.net/browse/SPLAT-2792)
- **What**: Fixes a bug where previously removed vCenters persisted in the cloud config after reconfiguration. The fix clears the `cfg.Vcenter` map before repopulating it from the Infrastructure spec.
- **Status**: MERGED (2026-06-08)
- **Stats**: +74 / -9, 3 files
- **Key change**: 3 lines added - `cfg.Vcenter = make(map[string]*ccmConfig.VirtualCenterConfig)`
- **Risk factors**: Minimal - straightforward fix with good test coverage. Could theoretically break if old vCenters were intentionally preserved, but the PR description confirms this was a bug.
- **QE focus areas**:
  - Add a second vCenter, then remove the first - verify only the second remains in the config
  - Verify that existing single-vCenter configurations are unaffected

---

### 5. `openshift/cluster-config-operator#481` - CCO Stops Managing vSphere Config | Risk: **HIGH**

- **PR**: https://github.com/openshift/cluster-config-operator/pull/481
- **JIRA**: [SPLAT-2717](https://redhat.atlassian.net/browse/SPLAT-2717)
- **What**: CCO's `KubeCloudConfigController` now checks the `VSphereMultiVCenterDay2` feature gate and skips managing `kube-cloud-config` for vSphere when it's enabled.
- **Status**: MERGED (2026-05-05, same day as PR #3)
- **Stats**: +5162 / -314, 91 files (bulk is vendor updates)
- **Key changes**:
  - New `shouldManageCloudConfig()` and `isFeatureGateEnabled()` methods
  - Feature gate lister wired into the controller
  - Graceful degradation: if feature gate lister is unavailable, defaults to managing (backwards compatible)
  - Switch-based extensibility for other platforms
- **Risk factors**:
  - **Coordination with PR #3**: Both PRs merged on the same day. If the CCO update rolls out before the CCCMO update, there's a window where nobody manages the ConfigMap for vSphere.
  - **Feature gate timing**: If the feature gate resource isn't available (e.g., during early bootstrap), CCO defaults to managing - verify this doesn't conflict with CCCMO also trying to manage.
  - **Non-vSphere platforms**: The switch statement defaults to always managing for AWS/Azure/GCP - verify no regressions on those platforms.
- **QE focus areas**:
  - **Operator rollout ordering**: Determine if the CVO rolls out CCCMO or CCO first, and verify the transition is seamless
  - **Feature gate unavailability**: Test behavior when the FeatureGate CR is temporarily unavailable
  - Verify AWS, Azure, and GCP cloud config management is completely unaffected

---

### 6. `openshift/cluster-config-operator#489` - Feature Gate Race Condition Fix | Risk: **MEDIUM**

- **PR**: https://github.com/openshift/cluster-config-operator/pull/489
- **JIRA**: [SPLAT-2747](https://redhat.atlassian.net/browse/SPLAT-2747)
- **What**: Fixes a race condition where the `KubeCloudConfigController` used stored feature gates (captured at init time) instead of dynamically querying the accessor. Also reorders startup so `featureGateController` runs first, then the accessor waits up to 5 minutes for initialization.
- **Status**: MERGED (2026-05-19)
- **Stats**: +52 / -40, 3 files
- **Key changes**:
  - Replaced `currentFeatureGates featuregates.FeatureGate` field with `featureGateAccessor featuregates.FeatureGateAccess`
  - Changed receiver from value (`c KubeCloudConfigController`) to pointer (`c *KubeCloudConfigController`)
  - 5-minute timeout with `time.After` for feature gate initialization
  - Reordered controller startup: `featureGateController.Run()` -> `featureGateAccessor.Run()` -> wait for observation -> start dependent controllers
  - Logging downgraded from event recorder to `klog.V(4)` for the "skipping" message (was firing every minute)
- **Risk factors**:
  - **Startup blocking**: A 5-minute timeout during operator startup could delay cluster initialization if feature gates are slow to propagate. In degraded clusters, this could compound issues.
  - **Controller ordering change**: Moving `featureGateController` to start before other controllers changes the startup sequence - could surface latent issues.
  - **Pointer receiver change**: Changing from value to pointer receiver is a correctness fix but could theoretically break any code that copies the struct (unlikely but worth noting).
- **QE focus areas**:
  - Test operator startup under degraded conditions (API server slow, etcd degraded)
  - Verify the 5-minute timeout doesn't trigger under normal conditions
  - Confirm feature gate changes are detected dynamically (no restart required)

---

### 7. `openshift/installer#10529` - Installer vSphere Day 2 Support | Risk: **MEDIUM**

- **PR**: https://github.com/openshift/installer/pull/10529
- **JIRA**: [SPLAT-2710](https://redhat.atlassian.net/browse/SPLAT-2710)
- **What**: Changes the installer's vSphere cloud provider config generation to use `openshift/library-go` instead of `k8s.io/cloud-provider-vsphere`. Adds a hidden `--vsphere-ini-format` CLI flag for migration testing.
- **Status**: MERGED (2026-05-06)
- **Stats**: +5092 / -1726, 82 files (bulk is vendor updates)
- **Key changes**:
  - Switched from `cloudconfig.VirtualCenterConfigYAML` to `cloudconfig.VirtualCenterConfig` (library-go types)
  - Removed `k8s.io/cloud-provider-vsphere` dependency entirely
  - Output format changes: zero-value fields are now omitted, field ordering changed, `InsecureFlag` moved from per-vCenter to global-only
  - Labels changed from `cloudconfig.LabelsYAML` to `cloudconfig.Labels` with pointer semantics
- **Risk factors**:
  - **Config format change**: The output YAML is structurally different (see test fixture update). While semantically equivalent, any consumer doing string comparison or expecting specific field ordering could break.
  - **InsecureFlag moved**: `InsecureFlag` removed from per-vCenter config and only set in global section. Verify all consumers read from global.
  - **Hidden flag**: The `--vsphere-ini-format` flag bypasses normal codepaths - verify it's truly hidden and not exposed in help text.
  - **MCO implications**: The PR description notes this prevents MCO from thinking bootstrap config doesn't match initial MachineConfig. Verify MCO reconciliation is clean.
- **QE focus areas**:
  - **Fresh install on vSphere**: End-to-end install and verify cloud provider config is correct and CCM starts successfully
  - **Compare old vs new config**: Ensure the new library-go-generated config is functionally identical to the old one
  - **MCO reconciliation**: Verify no MachineConfig drift after install

---

### 8. `openshift/machine-api-operator#1510` - ValidatingAdmissionPolicies for Failure Domain Protection | Risk: **HIGH** (STILL OPEN)

- **PR**: https://github.com/openshift/machine-api-operator/pull/1510
- **JIRA**: [SPLAT-2790](https://redhat.atlassian.net/browse/SPLAT-2790)
- **What**: Adds three VAPs that prevent removing a vSphere failure domain from `Infrastructure/cluster` when it's still referenced by Machines, ControlPlaneMachineSets, or MachineSets.
- **Status**: **OPEN** (not yet merged)
- **Stats**: +804 / -13, 7 files
- **Key changes**:
  - Three new `ValidatingAdmissionPolicy` + `ValidatingAdmissionPolicyBinding` pairs
  - Complex CEL expressions matching Machine region/zone labels against failure domain definitions
  - CPMS check compares `spec.template.machines_v1beta1_machine_openshift_io.failureDomains`
  - MachineSet check examines `spec.template.spec.providerSpec` for vSphere failure domain references
  - New RBAC for MAO to read Machines, CPMS, MachineSets and manage VAPs
  - Feature-gated behind `VSphereMultiVCenterDay2`
- **Risk factors**:
  - **CEL expression complexity**: The VAP CEL expressions are substantial and must handle all edge cases (missing labels, empty lists, nil fields). A bug here could block legitimate infrastructure updates.
  - **Param binding scope**: Each Machine/CPMS/MachineSet in `openshift-machine-api` triggers a separate evaluation - performance impact on clusters with many machines.
  - **RBAC expansion**: MAO service account gains read access to Machines, CPMS, and MachineSets, plus write access to VAPs/VAP bindings.
  - **ParamNotFound behavior**: Set to `AllowAction` - if no Machines exist, the check is skipped. Verify this is the correct behavior.
  - **MachineSet with 0 replicas**: The MachineSet VAP specifically catches this case (no Machines but MachineSet template still references the FD). Good coverage but adds complexity.
- **QE focus areas**:
  - Try to remove a failure domain that has running Machines - verify admission is denied with clear error
  - Try to remove a failure domain referenced by CPMS - verify denial
  - Try to remove a failure domain referenced by a MachineSet (including 0-replica) - verify denial
  - Remove a failure domain with no references - verify it's allowed
  - Performance: test on clusters with 100+ Machines to verify no latency impact on Infrastructure updates
  - **Upgrade testing**: Verify VAPs are created on upgrade and don't block ongoing operations

---

## Cross-Cutting Risk Areas

### 1. Operator Coordination / Migration Ordering (CRITICAL)

The most significant risk is the ownership migration of `kube-cloud-config` from CCO to CCCMO. If the CVO rollout order doesn't guarantee CCCMO is updated before CCO stops managing the ConfigMap, there's a window of unmanaged state. **QE must verify the CVO rollout order and test the transition under realistic upgrade conditions.**

### 2. Feature Gate Lifecycle

All changes are gated behind `VSphereMultiVCenterDay2`, but:
- What happens if the gate is enabled, config is migrated, then the gate is disabled (rollback)?
- What happens during upgrade when the gate status may be temporarily unknown?
- PR #6 specifically addresses a race condition here, but the 5-minute startup timeout introduces its own risk.

### 3. Config Format Compatibility

Three different components now generate/consume vSphere cloud config:
- **Installer** (generates initial config using library-go)
- **CCCMO** (transforms and manages config using library-go)
- **CCM** (consumes the config)

All three must agree on format. The switch from `k8s.io/cloud-provider-vsphere` types to `openshift/library-go` types in both the installer and CCCMO reduces divergence risk, but the format change (omitted zero-value fields, field ordering) needs validation against CCM's parser.

### 4. CRD Validation Upgrade Safety

The xValidation changes in PR #2 modify how the Infrastructure CRD validates vCenter arrays. Clusters upgrading with edge-case configurations (empty vCenters, single vCenter without explicit array entry) must not be blocked by the new rules. The ratcheting tests cover several scenarios but real-world configurations may differ.

---

## Recommended Test Matrix

| Scenario | Priority | PRs Involved |
|---|---|---|
| Fresh vSphere install with feature gate enabled | P0 | All |
| Upgrade single-vCenter cluster, enable feature gate | P0 | #2, #3, #5, #6, #7 |
| Add second vCenter Day 2 (end-to-end) | P0 | #2, #3, #4 |
| Remove unused vCenter Day 2 | P0 | #2, #3, #4, #8 |
| Remove in-use vCenter (should be blocked) | P0 | #2, #8 |
| Operator rollout ordering during upgrade | P0 | #3, #5 |
| Feature gate disable after migration (rollback) | P1 | #3, #5, #6 |
| AWS/Azure/GCP cluster - no regressions | P1 | #3, #5 |
| Degraded cluster startup (slow API server) | P1 | #6 |
| 100+ Machine cluster performance with VAPs | P2 | #8 |
| MCO reconciliation after install | P2 | #7 |

---

## Summary Risk Rating

| PR | Repo | Risk | Status |
|---|---|---|---|
| #2783 | openshift/api | LOW | Merged |
| #2784 | openshift/api | HIGH | Merged |
| #442 | openshift/cluster-cloud-controller-manager-operator | **CRITICAL** | Merged |
| #469 | openshift/cluster-cloud-controller-manager-operator | LOW | Merged |
| #481 | openshift/cluster-config-operator | HIGH | Merged |
| #489 | openshift/cluster-config-operator | MEDIUM | Merged |
| #10529 | openshift/installer | MEDIUM | Merged |
| #1510 | openshift/machine-api-operator | HIGH | **OPEN** |

**Overall feature risk: HIGH.** The cross-operator migration of ConfigMap ownership is the single highest-risk element, followed by the CRD validation changes and the new VAP enforcement. The feature gate provides a safety net, but the migration path (gate-off to gate-on) and rollback (gate-on to gate-off) must be thoroughly tested on real vSphere infrastructure.
