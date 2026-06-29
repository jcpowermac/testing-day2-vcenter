# Gap Analysis: vSphere Multi-vCenter Day 2 Risk Assessment

**Source document**: [vsphere-multi-vcenter-day2-risk-assessment.md](./vsphere-multi-vcenter-day2-risk-assessment.md)  
**Assessment date**: 2026-06-29  
**Purpose**: Identify coverage holes, stale assumptions, and untested risk areas before building the QE test plan.

---

## Executive Summary

The risk assessment correctly prioritizes ConfigMap ownership migration, CRD xValidation, and VAP enforcement. It is a strong starting point but **under-scopes the feature** (8 PRs / 5 repos vs. 12 PRs / 7 repos in the full implementation), **misses several downstream consumers**, and leaves **functional, operational, and symmetry gaps** between operators unexamined.

This document groups gaps by category so the test plan can close them deliberately.

---

## 1. Scope and Inventory Gaps

### 1.1 PRs not covered in the risk assessment

| PR | Repo | Status | Gap |
|---|---|---|---|
| [library-go#2175](https://github.com/openshift/library-go/pull/2175) | library-go | Merged | Shared INI↔YAML module — root cause of installer/CCCMO parity; no standalone risk or regression strategy |
| [library-go#2195](https://github.com/openshift/library-go/pull/2195) | library-go | Merged | `Node` struct serialization (empty nodes omitted, network CIDR in YAML) — affects multi-network Day 2 |
| [installer#10614](https://github.com/openshift/installer/pull/10614) | installer | Merged | Node network CIDR, vCenter port omission, multi-network GA — post-#10529 follow-on not in matrix |
| [vsphere-problem-detector#224](https://github.com/openshift/vsphere-problem-detector/pull/224) | vsphere-problem-detector | **Open** | `GetVCenter` after FD removal — operational health checks during Day 2 mutations |

**Impact**: Config-format and node-networking risks are split across library-go + two installer PRs but only #10529 is assessed. Problem-detector behavior after topology changes is entirely absent.

### 1.2 Stale status in the source document

| Item | Risk assessment says | Current state |
|---|---|---|
| MAO #1510 | OPEN | **Merged** (2026-06-25) — VAP tests are now in-scope for current builds |
| PR count | 8 PRs, 5 repos | 12 PRs, 7 repos |
| Assessment date | 2026-06-23 | MAO merge and installer #10614 (2026-06-12) post-date some conclusions |

### 1.3 JIRA / ticket traceability gaps

The assessment maps most work to SPLAT tickets but does not link:

- **OCPBUGS-87906** (problem detector) — customer-visible bug path, severity *important*
- **SPLAT-2795** (installer node networking) — distinct from SPLAT-2710

Test plans should trace scenarios to these tickets for release sign-off.

---

## 2. Component and Consumer Gaps

The assessment names Installer, CCCMO, CCO, and CCM. These additional consumers are **not** risk-rated or in the test matrix:

| Consumer | Why it matters | Suggested QE focus |
|---|---|---|
| **Machine Config Operator (MCO)** | Mentioned only for post-install drift (#10529); also renders `ControllerConfig` and node cloud config | Verify no unexpected node reboots / `MachineConfigPool` degraded after Infrastructure Day 2 edits |
| **ControllerConfig CRD** | api#2784 adds parallel xValidation on `controllerconfigs.machineconfiguration.openshift.io` | Explicit tests: invalid ControllerConfig updates blocked; valid Day 2 edits allowed with gate on |
| **vSphere CSI driver** | Uses topology / vCenter context for volume provisioning | StorageClass + PVC create/delete after vCenter/FD add/remove |
| **Cloud Credential Operator (CCO)** | Manages `cloud-credentials` secrets; new vCenters need credentials | Add vCenter Day 2 without matching secret → expected failure mode; with secret → success |
| **Machine API / CPMS / MachineSets** | VAP covers FD removal; not provisioning or scaling | Scale workers across FDs after topology change; CPMS rollout after FD add |
| **Routes / Services (CCM)** | CCM drives LoadBalancer and node lifecycle | Confirm `EXTERNAL-IP` assignment and node addresses after config migration |
| **vsphere-problem-detector** | Cluster health signals; #224 fixes vCenter resolution | Run detector checks before/after FD removal; verify alerts clear and no false positives |
| **Storage / registry / other vSphere integrations** | May read Infrastructure or cloud config indirectly | Spot-check image registry and any platform-specific operators on multi-vCenter topology |

**Gap**: No end-to-end “cluster still works for workloads” definition beyond CCM start.

---

## 3. Functional Scenario Gaps

### 3.1 Covered well in the source document

- Add/remove vCenter (with gate)
- ConfigMap ownership handoff CCO → CCCMO
- VAP blocks FD removal when referenced
- Single-vCenter upgrade path
- Non-vSphere regression (high level)

### 3.2 Missing or under-specified scenarios

| Scenario | Priority | Notes |
|---|---|---|
| **Fresh install with 2+ vCenters and gate enabled** | P0 | Matrix only says “fresh install with gate enabled”; multi-vCenter from day 1 is a distinct code path from “add second vCenter Day 2” |
| **Add failure domain (not just remove)** | P0 | VAP and xValidation focus on removal; adding FD + mapping to new vCenter is core Day 2 value |
| **Modify failure domain attributes** (region/zone, topology tags) | P1 | Unclear if xValidation allows all field edits or only add/remove |
| **vCenter server address swap blocked** | P0 | api#2784 explicitly forbids swapping server on existing entry — not in test matrix |
| **Remove vCenter still referenced by failure domain** | P0 | Listed under xValidation QE focus but **not** in recommended test matrix (only “remove in-use vCenter” via VAP/FD path) |
| **Credential rotation for existing vCenter** | P1 | Day 2 often includes secret updates without topology change |
| **INI → YAML migration on live cluster** | P1 | Legacy clusters may have INI in `openshift-config`; CCCMO converts — needs before/after functional check |
| **Enable gate with CCCMO already upgraded but CCO not yet** (and reverse) | P0 | Matrix mentions “operator rollout ordering” but not explicit version skew matrix |
| **Toggle gate off after migration (rollback)** | P1 | Listed as P1 but no expected behavior defined (who re-owns ConfigMap? INI vs YAML?) |
| **Toggle gate on without cluster admin changing Infrastructure** | P1 | Passive migration: only operators change behavior |
| **Node network / multi-network Day 2** | P1 | installer#10614 + library-go#2195; multi-network now GA — test node CIDR in generated cloud config matches Infrastructure |
| **vCenter port omitted vs explicit** | P2 | #10614 — verify CCM/CSI tolerate absent port field |
| **Hypershift vs self-managed** | P1 | Feature gate manifests include Hypershift profiles; no platform variant in matrix |
| **FeatureGate profile variants** | P1 | DevPreview, TechPreview, CustomNoUpgrade — gate availability differs |

### 3.3 Validation-layer overlap (ambiguous failure modes)

Two independent mechanisms can block Infrastructure edits:

1. **CRD xValidation** (api#2784) — API server rejects invalid patches
2. **ValidatingAdmissionPolicies** (MAO#1510) — admission webhook layer for FD removal vs Machines/CPMS/MachineSets

**Gap**: No guidance on which layer should fire for which error, or how to diagnose when both apply. QE should document expected HTTP/StatusError messages and `kubectl explain` validation rules vs VAP denial reasons.

**Gap**: xValidation may block vCenter removal when FDs still reference it; VAP blocks FD removal when Machines reference it. **Order of operations** for shrinking topology (Machines → FD → vCenter) is not documented for testers.

---

## 4. Operator Symmetry and Timing Gaps

### 4.1 CCO race fix without CCCMO equivalent

CCO #489 fixed dynamic feature-gate reads and startup ordering. The assessment does **not** ask:

- Does CCCMO use a static snapshot or dynamic `FeatureGateAccess`?
- Does CCCMO block startup waiting for feature gates?
- If CCO sees gate **off** (fallback: keep managing) while CCCMO sees gate **on** (manage managed ConfigMap), do both write?

**Risk**: Split-brain or thrashing on `kube-cloud-config` during gate propagation or operator version skew.

### 4.2 Asymmetric graceful degradation

CCO #481: if feature gate lister unavailable → **defaults to managing** (backward compatible).  
CCCMO #442: behavior when gate unavailable is **not** explicitly assessed.

**QE need**: Matrix row for “FeatureGate CR absent / CCO pod restarted / CCCMO pod restarted” with expected single-writer outcome.

### 4.3 `cloud-conf` always-update (#442)

Assessment flags unnecessary reconciliation loops but does not specify verification:

- CCM pod restart count before/after Infrastructure noop change
- Leader election churn
- Node cloud provider taint removal timing

---

## 5. Config Format and Parity Gaps

### 5.1 Three-way parity not operationalized

The assessment says Installer, CCCMO, and CCM must agree. Missing:

| Check | Gap |
|---|---|
| **Golden-file diff** | No procedure to diff bootstrap CM vs `openshift-config-managed/kube-cloud-config` vs `openshift-cloud-controller-manager/cloud-conf` after gate enable |
| **Semantic vs byte equality** | MCO may care about byte-level match; library-go omits zero values — need field-by-field semantic checklist |
| **InsecureFlag location** | Noted for #10529; retest after #10614 and CCCMO transformer |
| **Labels / topology tags** | Infrastructure-derived injection in CCCMO — not listed as explicit validation fields |
| **Legacy INI consumers** | Any component still expecting INI in managed namespace? |

### 5.2 library-go as single point of failure

#2175 centralizes parsing. Regression in library-go affects installer, CCCMO, and tests simultaneously.

**Gap**: No recommendation to run **cross-repo** contract tests or pin compatible library-go versions across operators in CI.

---

## 6. VAP-Specific Gaps (MAO #1510)

Now merged; expand beyond assessment bullets:

| Gap | Detail |
|---|---|
| **ParamNotFound = AllowAction** | If MAO RBAC broken or namespace empty, FD removal might succeed unchecked — negative test with denied RBAC |
| **CEL maintenance** | OpenShift version skew on CEL features — upgrade from cluster without VAP to cluster with VAP |
| **Machine without region/zone labels** | VAP matching logic may not apply — behavior undefined in assessment |
| **User-provisioned vs installer-provisioned Machines** | Different label patterns |
| **Control plane nodes** | CPMS path covered; standalone control plane Machines? |
| **Concurrent Infrastructure edits** | Many Machines → admission latency; only P2 in matrix |
| **VAP disabled when gate off** | Confirm VAP objects absent or inactive; no accidental denial on gate-off clusters |

---

## 7. Problem Detector Gap (OCPBUGS-87906)

PR #224 is **open** but implements behavior needed for safe Day 2 shrink operations.

| Scenario | Expected interest |
|---|---|
| Node in FD A, FD A removed from Infrastructure, node labels unchanged | vCenter resolution until node replaced |
| Multi-FD → single-FD reduction | Deterministic fallback vCenter |
| Missing topology labels | Warning vs error behavior |
| Problem detector alerts during Day 2 maintenance | Alert noise / false positives |

**Gap**: Risk assessment has zero mention; should be **P0** once #224 merges because it affects go/no-go for production Day 2 changes.

---

## 8. Non-Functional and Operational Gaps

| Area | Gap |
|---|---|
| **Observability** | No list of must-check conditions: `ClusterOperator/cloud-controller-manager`, `ClusterOperator/cluster-config-operator`, CCCMO degraded, MCO `Updating` |
| **Runbook validation** | No step to validate customer-facing docs for ordered topology reduction |
| **Rollback / downgrade** | Cluster version downgrade with gate-enabled Infrastructure mutations — undefined |
| **Backup / restore** | etcd restore with mutated Infrastructure — out of scope? |
| **Support bundle** | Which CRs/CMs support needs for Day 2 incidents |
| **Performance baseline** | Infrastructure patch latency with 100+ Machines — P2 only; no SLO |
| **Security** | CCCMO RBAC noted; MAO VAP RBAC noted; no review of who can patch `Infrastructure` to add vCenters |
| **Soak testing** | Long-running cluster with periodic Infrastructure sync — detect slow config drift |

---

## 9. Test Matrix Additions (Proposed)

Rows to add when merging into the formal test plan:

| Scenario | Priority | Closes gap |
|---|---|---|
| Fresh multi-vCenter install, gate on | P0 | §3.2 |
| Add failure domain + map to new vCenter | P0 | §3.2 |
| Block vCenter server address swap | P0 | §3.2, api#2784 |
| Remove vCenter referenced by FD (API rejection) | P0 | §3.3 |
| Ordered topology shrink: drain Machines → remove FD → remove vCenter | P0 | §3.3 |
| ConfigMap three-way semantic parity check | P0 | §5.1 |
| CCO/CCCMO version skew during upgrade | P0 | §4.1, §4.2 |
| Gate on: only CCCMO writes; gate off: only CCO writes | P0 | §4.2 |
| ControllerConfig xValidation with gate on/off | P1 | §2 |
| Node network CIDR in cloud config (post-#10614) | P1 | §1.1, §3.2 |
| CSI volume lifecycle after topology change | P1 | §2 |
| Cloud credentials for added vCenter | P1 | §2 |
| INI legacy → YAML migration | P1 | §3.2, §5.1 |
| Problem detector after FD removal (#224) | P0 when merged | §7 |
| Hypershift control plane | P1 | §3.2 |
| VAP inactive when gate disabled | P1 | §6 |
| CCM pod stability / no hot loop on noop sync | P2 | §4.3 |

---

## 10. Open Questions for Dev / PM

These block complete test design if unanswered:

1. **Rollback contract**: Gate disabled after Day 2 edits — is CCO expected to reclaim YAML, convert to INI, or leave last CCCMO state?
2. **Minimum vCenter count**: Ratcheting allows empty persisted array — is zero vCenters a supported steady state or only upgrade tolerance?
3. **CVO ordering**: Is CCCMO guaranteed before CCO in the same release payload?
4. **CCCMO feature-gate access**: Same dynamic accessor pattern as CCO #489 or not?
5. **Customer upgrade guide**: Required order of operations for removing a datacenter from rotation?
6. **Hypershift**: Full feature parity or subset for Day 2 vCenter management?

---

## 11. Summary

| Category | Severity | Count (approx.) |
|---|---|---|
| Missing PR coverage | High | 4 PRs |
| Missing consumers | High | 7+ components |
| Missing functional scenarios | High | 15+ scenarios |
| Operator symmetry / timing | Critical | 3 major items |
| Stale document status | Medium | 2 items |
| Operational / non-functional | Medium | 8+ items |

**Recommendation**: Treat this gap analysis as an addendum to the risk assessment. The test plan should expand scope to **12 PRs**, mark MAO #1510 as merged, elevate **problem detector #224** when it lands, and add explicit **CCCMO/CCO symmetry** and **three-way config parity** workstreams before sign-off on Day 2 vCenter QE.
