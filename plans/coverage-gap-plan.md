# Coverage Gap Plan: vSphere Multi-vCenter Day 2

## Context

We have 41 test specs across 10 files. Most are xValidation dry-run tests and read-only observations. The user's concern: we're testing the admission layer but not verifying that CPMS, MAO, CSI, and the operators actually **work** with multi-vCenter. QE needs positive/integration tests alongside negative tests.

This plan maps all 12 feature PRs to current coverage, identifies gaps, and proposes new tests grouped by QE category.

---

## PR-to-Coverage Matrix

| # | PR | What it does | Current coverage | Gap |
|---|---|---|---|---|
| 1 | api#2783 | Feature gate `VSphereMultiVCenterDay2` | featuregate_test.go: gate exists, enabled state ✓ | None |
| 2 | api#2784 | xValidation CRD rules (add/remove/swap/dup/max3) | infrastructure_validation_test.go: N-INF-01–12 ✓ | N-INF-12 exposes product gap (SPLAT-2827), covered |
| 3 | MAO#1510 | 3 VAPs: machine, cpms, machineset FD protection | vap_test.go: existence ✓, Machine denial ✓, unreferenced removal ✓ | **No CPMS denial test. No MachineSet denial test. No active VAP test (create MS, verify denial, delete MS).** |
| 4 | CCCMO#442 | CCCMO manages kube-cloud-config (INI→YAML, FG-gated) | configmap_ownership_test.go: CM exists ✓, stable ✓ | **No verify CCO stopped writing. No verify CCCMO is the writer.** |
| 5 | CCCMO#469 | Stale vCenter cleanup in cloud config | configmap_content_test.go: no stale ✓, topology_lifecycle: add/remove ✓ | Covered |
| 6 | CCO#481 | CCO skips kube-cloud-config when gate ON | configmap_ownership: steady-state ✓ | **No explicit verify CCO is NOT reconciling the CM** |
| 7 | CCO#489 | FeatureGateAccessor race fix | operator_health_test.go: CO health ✓ | Covered (regression is operator crash, which health check catches) |
| 8 | installer#10529 | Cloud config format alignment (library-go structs) | configmap_content: parsing ✓ | Covered |
| 9 | installer#10614 | Node network CIDR in cloud config | configmap_content: nodes section ✓ | Covered |
| 10 | library-go#2175 | vSphere cloud config INI/YAML shared module | Indirectly via config parsing ✓ | Covered |
| 11 | library-go#2195 | Node struct pointer→value fix | Indirectly ✓ | Covered |
| 12 | vpd#224 | vsphere-problem-detector GetVCenter FD removal handling | problem_detector_test.go: CO health stub ✓ | **No functional test. Stub only.** |

---

## Gaps by QE Category

### 1. Positive / Integration Tests (missing)

These verify the happy path works end-to-end. We have almost none.

#### a. CPMS references failure domains correctly
- Read CPMS, verify each `failureDomains` entry maps to an Infrastructure FD by **name**
- Verify CPMS FD names match Infrastructure FD names
- Verify CPMS providerSpec datacenter/datastore match the Infrastructure FD topology

#### b. MachineSet references failure domains correctly
- List MachineSets, extract providerSpec (vSphere), verify datacenter/datastore/network match an Infrastructure FD topology
- Verify MachineSet labels (region/zone) match an Infrastructure FD

#### c. Machine placement verification
- List Machines, verify each Machine's providerSpec/providerID maps to the correct vCenter
- Verify Machine labels (region/zone) match an Infrastructure FD
- Group Machines by vCenter — confirm distribution matches expectations

#### d. CSI driver has correct cloud config
- Read `openshift-cluster-csi-drivers/vmware-vsphere-cloud-credentials` secret — verify it has entries for all vCenters
- Read CSI driver config secret (`vsphere-csi-config-secret`) — verify it references all vCenters
- Verify ClusterCSIDriver is Available and not Degraded
- **(stretch)** Create a PVC, verify it binds, delete it — confirms CSI can actually provision on multi-vCenter

#### e. CCM (Cloud Controller Manager) pods healthy
- List CCM pods, verify Running, no restart loops
- Verify CCM cloud-conf ConfigMap matches Infrastructure vCenters

#### f. Credentials propagation
- Verify `kube-system/vsphere-creds` has entries for ALL vCenters in Infrastructure (username + password keys per server)
- Verify `openshift-cloud-controller-manager/vsphere-cloud-credentials` matches
- Verify `openshift-machine-api/vsphere-cloud-credentials` matches
- Verify `openshift-cluster-csi-drivers/vmware-vsphere-cloud-credentials` matches

### 2. VAP Integration Tests (partially missing)

Current tests only check existing Machine labels passively. Missing active tests:

#### a. CPMS VAP denial
- Read CPMS, find which FD names it references
- Attempt to remove that FD from Infrastructure (dry-run) — expect denial by `vsphere-failure-domain-in-use-by-cpms`

#### b. MachineSet VAP denial
- List MachineSets, find one with region/zone labels matching an Infrastructure FD
- Attempt to remove that FD (dry-run) — expect denial by `vsphere-failure-domain-in-use-by-machineset`

#### c. Active VAP test (mutating, real-vcenter only)
- Create a new MachineSet targeting an existing FD with 0 replicas
- Attempt to remove that FD (dry-run) — expect denial by MachineSet VAP
- Delete the MachineSet
- Retry FD removal — expect allowed (or only blocked by Machine VAP if Machines exist)

### 3. Regression Tests (partially missing)

These verify that new code didn't break existing functionality.

#### a. Cloud config three-way consistency after Day 2 add
- After adding a second vCenter (test-real scenario): verify managed CM, CCM CM, and CSI config all agree
- Currently `real_vcenter_test.go` checks Infrastructure + managed CM. Missing: CCM CM and CSI config

#### b. Operator health after Day 2 add
- After adding second vCenter: verify all relevant COs remain Available/not-Degraded
- Currently checked in BeforeSuite but not re-checked after mutations

#### c. vsphere-problem-detector after multi-vCenter
- Verify CO `vsphere-problem-detector` is Available after the cluster has 2+ vCenters
- PR #224 fixed GetVCenter for FD removal — test that the detector doesn't degrade when FDs exist across multiple vCenters

### 4. Smoke Tests (partially covered)

#### a. Post-mutation smoke
- After any mutating test (add/remove vCenter): verify operators healthy, cloud configs parseable, Machines still Running
- Currently topology_lifecycle_test.go checks cloud config but not operator health or Machine state

### 5. API Tests (partially covered)

#### a. Infrastructure CR status fields
- After Day 2 add: verify `status.platformStatus.vsphere` reflects the new vCenters
- Verify `status.controlPlaneTopology` is unchanged

#### b. FeatureGate status fields
- Verify gate appears in correct version in status (already covered)

---

## Proposed New Test Files / Additions

### New: `test/e2e/machine_integration_test.go` [readonly, p0, integration]

```
Describe "Machine integration"
  It "should have all Machines in Running phase"
  It "should label every Machine with region and zone"
  It "should map every Machine to a valid Infrastructure failure domain"
  It "should have Machine providerSpec matching Infrastructure FD topology"
```

### New: `test/e2e/cpms_integration_test.go` [readonly, p0, integration]

```
Describe "CPMS integration"
  It "should reference failure domain names that exist in Infrastructure"
  It "should have providerSpec matching Infrastructure FD topology"
```

### New: `test/e2e/machineset_integration_test.go` [readonly, p0, integration]

```
Describe "MachineSet integration"
  It "should have providerSpec matching an Infrastructure FD topology"
```

### Additions to existing: `test/e2e/vap_test.go` (per reviewer suggestion #3)

```
  It "should deny removing a failure domain referenced by CPMS (N-SEQ-02)"
  It "should deny removing a failure domain referenced by a MachineSet (N-SEQ-03)"
```

### New: `test/e2e/csi_integration_test.go` [readonly, p1, integration]

```
Describe "CSI driver integration"
  It "should have ClusterCSIDriver vsphere Available and not Degraded"
  It "should have CSI config secret referencing all Infrastructure vCenters"
  It "should have CSI credentials for all Infrastructure vCenters"
```

### New: `test/e2e/credentials_test.go` [readonly, p0, integration]

```
Describe "vSphere credentials propagation"
  It "should have vsphere-creds entries for all Infrastructure vCenters"
  It "should have MAO vsphere-cloud-credentials for all vCenters"
  It "should have CCM vsphere-cloud-credentials for all vCenters"
  It "should have CSI vsphere-cloud-credentials for all vCenters"
```

### Additions to existing: `test/e2e/operator_health_test.go`

```
  It "should keep CCM pods running without restart loops"
  It "should keep vsphere-problem-detector Available with multi-vCenter"
```

### Additions to existing: `test/e2e/configmap_ownership_test.go`

```
  It "should have CCCMO as the managing controller when gate is enabled"
```

### Additions to existing: `test/e2e/real_vcenter_test.go`

```
  It "should have CCM cloud-conf matching Infrastructure after Day 2 add"
  It "should have credentials propagated for the second vCenter"
  It "should keep all operators healthy after Day 2 add"
```

### Additions to existing: `test/e2e/topology_lifecycle_test.go` [mutating]

```
Context "MachineSet VAP active test"
  It "should deny FD removal after creating a MachineSet targeting that FD"
    — create 0-replica MachineSet with providerSpec pointing to FD
    — dry-run remove FD, expect VAP denial
    — delete MachineSet
    — DeferCleanup deletes MachineSet if test fails
```

---

## New Framework Helpers Needed

| Helper | File | Purpose |
|---|---|---|
| `GetCSIDriverCondition` | `pkg/framework/conditions.go` | Check ClusterCSIDriver Available/Degraded |
| `ListPodsByLabel` | `pkg/framework/pods.go` | List pods by label selector (for CCM pod health) |
| `GetSecretKeys` | `pkg/framework/secrets.go` | Read secret data keys for credential checks |
| `ExtractMachineProviderSpec` | `pkg/framework/machine.go` | Unmarshal Machine providerSpec to vSphere type |
| `ExtractCPMSFailureDomains` | `pkg/framework/machine.go` | Get FD names from CPMS spec |
| `ExtractMachineSetProviderSpec` | `pkg/framework/machine.go` | Unmarshal MachineSet providerSpec |
| `CreateMachineSet` / `DeleteMachineSet` | `pkg/framework/machine.go` | For active VAP test |

---

## Priority and Labels

| Test group | Label | Priority | Type |
|---|---|---|---|
| Credentials propagation | readonly, integration, p0 | P0 | Positive |
| Machine integration | readonly, integration, p0 | P0 | Positive |
| CPMS integration | readonly, integration, p0 | P0 | Positive |
| MachineSet integration | readonly, integration, p0 | P0 | Positive/VAP |
| CSI integration | readonly, integration, p1 | P1 | Positive |
| Operator health additions | readonly, operator, p0 | P0 | Smoke |
| Real vCenter additions | real-vcenter, p0 | P0 | Regression |
| Active MachineSet VAP | mutating, admission, p1 | P1 | Integration |
| ConfigMap ownership verify | readonly, config, p1 | P1 | Positive |

---

## Implementation Order

1. `pkg/framework/machine.go` + `secrets.go` — helpers for providerSpec extraction, secret reading
2. `test/e2e/credentials_test.go` — quick win, validates all 4 credential secrets match Infrastructure
3. `test/e2e/machine_integration_test.go` — Machine placement validation
4. `test/e2e/cpms_integration_test.go` — CPMS FD references + CPMS VAP denial
5. `test/e2e/machineset_integration_test.go` — MachineSet FD references + MachineSet VAP denial
6. Additions to `operator_health_test.go` — CCM pod health, problem-detector multi-vCenter
7. `test/e2e/csi_integration_test.go` — CSI driver health + config
8. Additions to `real_vcenter_test.go` — post-Day-2 regression checks
9. Active MachineSet VAP test in `topology_lifecycle_test.go`
10. Additions to `configmap_ownership_test.go` — CCCMO ownership verification

## Decisions

- **MachineSet VAP**: Create a real 0-replica MachineSet (no VMs spawned), verify VAP blocks FD removal, delete it. Mutating label.
- **CSI**: Full depth — verify CSI config + credentials + create/bind/delete a test PVC to prove provisioning works.

## Verification

After implementation:
1. `make build` — compiles
2. `make test-readonly` on remote — all new readonly tests pass
3. `make test-mutating` on remote — MachineSet VAP test passes
4. `make test-real` on remote — post-Day-2 regression checks pass
5. Review `docs/tests.md` — update with new test descriptions

---

## Adversarial Review (for implementing agent)

**Reviewer role:** adversarial QE / red-team  
**Review date:** 2026-06-29  
**Scope:** challenge assumptions, catch false confidence, and force explicit trade-offs before implementation  
**Baseline checked:** 41 `It()` specs in `test/e2e/`

### Reviewer corrections (2026-06-29)

**The adversarial reviewer was wrong about credentials.** Do **not** treat AR-C1 as blocking or authoritative.

| What the reviewer got wrong | What is actually true |
|---|---|
| Claimed §1f / `credentials_test.go` had the wrong secret matrix; asserted `openshift-config/cloud-credentials` “often does not exist” and that CCM has no credentials secret | The coverage-gap plan’s consumer-secret matrix is correct: **`openshift-cloud-controller-manager/vsphere-cloud-credentials`**, **`openshift-machine-api/vsphere-cloud-credentials`**, **`openshift-cluster-csi-drivers/vmware-vsphere-cloud-credentials`**, plus **`kube-system/vsphere-creds`**. E2E credential tests should assert parity across these consumers, not treat CCM as config-only. |
| Recommended optional secret discovery and de-emphasizing `openshift-config/cloud-credentials` | The real `apply-lab` bug was **`cloud-provider-config` data key mismatch** — source CM uses key **`config`**, managed/CCM use **`cloud.conf`** (`SourceCloudConfigDataKey` vs `CloudConfigDataKey`). Fixed in `85cde62`. |
| AR-M2 flagged `config-operator` vs `cluster-config-operator` as unresolved | **Fixed in `85cde62`** — ClusterOperator name is **`config-operator`**. |

**Implementing agent:** follow §1f and the proposed `credentials_test.go` as written. For lab apply, use `SourceCloudConfigDataKey` when reading/writing `openshift-config/cloud-provider-config`.

---

### Verdict (pre-implementation)

The gap analysis is directionally correct — admission-layer coverage dominates and operator/consumer integration is thin. The proposed additions are **not wrong**, but several items are **underspecified, overlapping with existing tests, or likely to flake/skip on real clusters**. Treat this plan as a backlog, not a merge-ready spec, until the implementing agent resolves the findings below.

**AR-C1 is withdrawn.** Remaining critical items: AR-C2–C5.

---

### Critical findings (must resolve before coding)

| ID | Finding | Why it matters | Required response from implementing agent |
|---|---|---|---|
| ~~**AR-C1**~~ | ~~Credential secret names wrong in plan~~ | **WITHDRAWN — reviewer error. See corrections above.** | N/A |
| **AR-C2** | **CSI “full depth” PVC create/bind/delete contradicts `readonly` label and lab safety** | Decisions say “create/bind/delete a test PVC”; Priority table labels CSI `readonly`. PVC provisioning is mutating, needs StorageClass, quota, cleanup, and may leave volumes on failure. | Pick one: (a) readonly credential/config parity only, or (b) mutating `real-vcenter` PVC smoke behind explicit label + skip gates. Do not ship under `readonly`. |
| **AR-C3** | **VAP dry-run reliability is unaddressed for new CPMS/MachineSet denial tests** | Existing suite probes `vapDryRunWorks` and falls back to **non-dry-run** patch attempts for Machine VAP (`helpers_test.go`). New CPMS/MachineSet tests assume dry-run denial works. On clusters where dry-run bypasses VAP, proposed tests will false-pass or need destructive paths. | Mirror `expectFailureDomainRemovalDenied` pattern: dry-run first, fallback with guarded real patch + immediate restore. Document which VAP each path validates. |
| **AR-C4** | **“CCMO is the writer” / “CCO is NOT reconciling” tests may be untestable as specified** | Steady-state stability (`WaitForConfigMapStable`) already exists. Proving **which controller** writes requires field-manager/owner-ref/audit evidence not in current framework. “CCO stopped writing” is a negative proof — absence of writes in 60s ≠ CCO disabled. | Propose concrete signal: e.g. `metadata.managedFields` manager=`cluster-cloud-controller-manager-operator`, or drop test and cite N-OP steady-state as sufficient. |
| **AR-C5** | **Active MachineSet VAP test (0-replica create) has undeclared prerequisites** | Requires cluster admin MachineSet create, valid providerSpec template, compatible failure domain, and may interact with CPMS/machine-webhooks. Zero replicas still creates an API object that VAP must reference. Clusters without CPMS or with single FD topology may skip entirely. | List preconditions, RBAC needs, template source (clone existing MS vs synthetic), and cleanup guarantees. Confirm VAP binds to MachineSet **objects** not just Machines. |

---

### Major findings (address in plan revision)

| ID | Finding | Recommendation |
|---|---|---|
| **AR-M1** | **Duplicate coverage with existing tests** | `configmap_content_test.go` already parses CCM `cloud-conf` and checks managed CM vCenter parity. `real_vcenter_test.go` runs `lab.Verify` (operators + managed CM). New “three-way consistency” and several `real_vcenter` additions overlap — specify *incremental* assertions only. |
| ~~**AR-M2**~~ | ~~Operator name inconsistency~~ | **Resolved in `85cde62`** — use `config-operator` everywhere. |
| **AR-M3** | **Machine integration tests are over-strict** | “All Machines in Running phase” fails on deleting/updating/failed machines during upgrades. Filter to owned MachineSets, exclude `Deleting`, allow `Provisioned`/`Running`, or scope to worker pool only. |
| **AR-M4** | **CPMS / MachineSet helpers partially exist** | `listCPMS()`, `listMachineSets()`, `listMachines()` already in `helpers_test.go`. Framework table proposes duplicates — extend existing helpers or move shared code to `pkg/framework/machine.go` once, not twice. |
| **AR-M5** | **P0 inflation** | Six new P0 files + active mutating VAP + CSI PVC would roughly **double** suite size and CI time. Most value for Day-2 sign-off is credentials + CPMS/MS VAP + post-apply operator smoke — not CSI provisioning. Re-tier: credentials/CPMS/MS/VAP = P0; CSI config-only = P1; PVC = P2/manual. |
| **AR-M6** | **`problem_detector` functional test blocked** | PR #224 not merged; placeholder skipped. Plan item 6 promises multi-vCenter detector behavior — either keep explicitly deferred or define interim CO-health-only acceptance. |
| **AR-M7** | **N-INF-12 / N-SEQ-04 product gap ignored in integration scope** | Topology tests **fail intentionally** on vCenter removal while FD references (SPLAT-2827). Integration tests assuming full FD/vCenter lifecycle symmetry may encode known product bugs as failures. Tag expected failures or skip until SPLAT-2827 closes. |

---

### Minor findings / nits

| ID | Finding |
|---|---|
| **AR-N1** | Spec count in header was stale (39 vs 41) — fixed in plan header. |
| **AR-N2** | `Infrastructure status.platformStatus` may lag spec; tests need `Eventually` after Day-2 apply, not single read. |
| **AR-N3** | `docs/tests.md` update is in Verification but not Implementation Order — add explicit step 11. |
| **AR-N4** | No skip-budget metric — if >30% of new specs skip on reference cluster, plan fails its purpose. Define max acceptable skip rate. |
| **AR-N5** | Gate-off regression still delegated to openshift/api fixtures — confirm new integration tests all `requireGateEnabled()` or document exceptions. |
| **AR-N6** | README still says `cluster-config-operator` in apply-lab step 4 — should be `config-operator`. |

---

### Questions the implementing agent must answer

1. ~~**Credential matrix**~~ — **Answered by plan §1f**; do not re-litigate. Optional: document key format per secret (`{server}.username` vs YAML) in test helpers.
2. **VAP proof strategy:** Will CPMS/MachineSet denial tests use the same dry-run fallback as Machine VAP? Under what conditions is a real patch attempt acceptable in CI?
3. **MachineSet template:** Where does the 0-replica MachineSet providerSpec come from — cloned from an existing MachineSet, or synthesized from Infrastructure FD topology?
4. **CCMO ownership:** What observable API field proves CCCMO ownership vs CCO non-writes?
5. **Real-vCenter ordering:** Do new `real_vcenter_test.go` specs assume `make apply-lab` already ran, or self-contain setup?
6. **CSI scope cutoff:** Config/credential parity only, or PVC provisioning? If PVC, which StorageClass and cleanup policy?

---

### Suggested plan revisions (accept or reject with rationale)

1. **Add `pkg/framework/credentials.go`** — shared helpers for §1f parity checks (read consumer secrets, assert per-vCenter keys). Do **not** narrow scope to kube-system only.
2. **Collapse proposed `credentials_test.go` + parts of `real_vcenter_test.go`** into one credentials parity describe block; avoid four secrets × two files.
3. **Add CPMS/MachineSet VAP cases to `vap_test.go`** (same labels/helpers) instead of three integration files that each re-list machines.
4. **Defer CSI PVC test** to manual QA or `real-vcenter` mutating until readonly suite is stable.
5. **Add post-mutation operator check** to `topology_lifecycle_test.go` DeferCleanup/AfterEach — cheaper than new smoke describe.

---

### Implementing agent response checklist

The implementing agent should edit this section (or link a PR comment) and mark each item:

- [x] **AR-C1** — **Withdrawn** (reviewer error; plan §1f is correct)
- [x] **AR-C2** — CSI split: readonly=config+creds, mutating/real-vcenter/p2=PVC lifecycle
- [x] **AR-C3** — Mirror expectFailureDomainRemovalDenied pattern; dry-run + fallback
- [x] **AR-C4** — Check managedFields for cloud-controller-manager; drop if signal absent
- [x] **AR-C5** — Clone existing MS, replicas=0, DeferCleanup, skip if single-FD
- [x] **AR-M1** — Only incremental assertions (CCM parity, creds) in real_vcenter
- [x] **AR-M2** — Resolved (`config-operator` in `85cde62`)
- [x] **AR-M3** — Filter to MS-owned, accept Running/Provisioned, exclude Deleting
- [x] **AR-M4** — Extend helpers_test.go lists; new shared helpers in pkg/framework/
- [x] **AR-M5** — P0=creds+machine+CPMS/MS VAP; P1=CSI config+active MS+ownership; P2=PVC
- [x] **AR-M6** — CO-health-only; deferred until #224 merges
- [x] **AR-M7** — New tests validate placement/VAP, not removal symmetry
- [x] **Questions 2–6** — Answered in agent responses above

#### Agent responses

```
AR-C1: Withdrawn — follow plan §1f.

AR-C2: CSI split into two scopes:
  - readonly p1: config secret refs all vCenters + ClusterCSIDriver CO healthy + credential secret parity
  - mutating real-vcenter p2: PVC create/bind/delete with default thin StorageClass, DeferCleanup
  Labels corrected — PVC is NOT readonly.

AR-C3: CPMS/MachineSet VAP tests mirror expectFailureDomainRemovalDenied:
  - dry-run first if vapDryRunWorks
  - fallback to real patch (rejection = no state change, safe)
  Each test documents which VAP it validates in the test name.

AR-C4: Check metadata.managedFields for manager containing "cloud-controller-manager".
  If managedFields doesn't carry that signal, drop the ownership assertion
  and rely on steady-state stability (already tested in configmap_ownership).
  Not worth operator log scraping or audit events.

AR-C5: Active MachineSet VAP test:
  - Template: clone existing worker MachineSet, rename to e2e-vap-probe, replicas=0
  - Preconditions: ≥1 MachineSet exists, cluster-admin (already have it)
  - Cleanup: DeferCleanup deletes MachineSet unconditionally
  - VAP binds to Infrastructure updates — checks MachineSet labels against proposed FD list
  - Skip if no MachineSet found or single-FD topology

AR-M1: Only add incremental assertions to real_vcenter_test.go:
  - CCM cloud-conf vCenter parity (not already checked)
  - Credential secret parity for second vCenter
  Won't duplicate existing managed CM or lab.Verify checks.

AR-M3: Machine phase filter:
  - Only Machines owned by a MachineSet (exclude orphans)
  - Accept Running or Provisioned
  - Exclude Deleting phase
  - Scope to openshift-machine-api namespace (already implied)

AR-M4: helpers_test.go already has listMachines/listMachineSets/listCPMS.
  New shared helpers (providerSpec extraction, secret reading) go in pkg/framework/
  since multiple test files need them. No duplication.

AR-M5: Re-tiered priorities:
  P0: credentials, Machine integration, CPMS VAP denial, MachineSet VAP denial
  P1: CSI config+health, active MachineSet create VAP, CCCMO ownership, operator additions
  P2: CSI PVC lifecycle (mutating, manual/real-vcenter)
  This keeps P0 focused on what blocks Day-2 sign-off.

AR-M6: problem-detector stays CO-health-only. Functional test deferred until #224 merges.
  problem_detector_test.go stub already documents this with Skip message.

AR-M7: New integration tests don't attempt full FD/vCenter removal lifecycle.
  N-INF-12 / N-SEQ-04 already document SPLAT-2827. New tests validate placement
  and VAP denial, not removal symmetry.

Q2: Same dry-run + fallback as Machine VAP. Real patch acceptable — rejection = no mutation.
Q3: Clone existing worker MachineSet, rename, replicas=0.
Q4: metadata.managedFields manager check. Drop if absent.
Q5: real_vcenter specs assume apply-lab already ran (vcenter-120 present). Verification only.
Q6: readonly = config/credential parity. mutating real-vcenter p2 = PVC with thin SC + DeferCleanup.

Revised implementation order:
  1. pkg/framework/machine.go + secrets.go — providerSpec extraction, secret reading
  2. credentials_test.go (P0, readonly)
  3. machine_integration_test.go (P0, readonly)
  4. CPMS VAP denial added to vap_test.go (P0, readonly) — per AR suggestion #3
  5. MachineSet VAP denial added to vap_test.go (P0, readonly)
  6. Additions to operator_health_test.go (P1)
  7. csi_integration_test.go — config/credential only (P1, readonly)
  8. Additions to real_vcenter_test.go — CCM parity + credentials (P0, real-vcenter)
  9. Active MachineSet VAP in topology_lifecycle_test.go (P1, mutating)
  10. CCCMO ownership in configmap_ownership_test.go (P1, readonly)
  11. CSI PVC lifecycle (P2, mutating, real-vcenter)
  12. Update docs/tests.md
```

---

### Adversarial sign-off

| Role | Status | Notes |
|---|---|---|
| Adversarial reviewer | **Corrected** | AR-C1 withdrawn; credentials follow plan §1f |
| Implementing agent | _pending_ | Complete checklist for AR-C2–C5, M1, M3–M7 |
| QE lead | _pending_ | Confirms P0 scope and skip budget after revision |
