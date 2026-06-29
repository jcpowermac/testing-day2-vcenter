# Plan: vSphere Multi-vCenter Day 2 Ginkgo/Gomega E2E Tests

## Context

The vSphere Multi-vCenter Day 2 feature (12 PRs across 7 repos, gated behind `VSphereMultiVCenterDay2`) adds lifecycle management for multiple vCenters on OpenShift/vSphere. Three planning documents define test scope:

- **Risk assessment**: 8 PRs rated LOW to CRITICAL; test matrix of 11 scenarios
- **Gap analysis**: Expands to 12 PRs, 7+ missing consumers, 15+ missing scenarios, operator symmetry concerns
- **Negative testing plan**: 41 structured negative cases (N-INF, N-SEQ, N-OP, N-CFG, N-ENV)

Tests run against a **live OpenShift cluster** and must be **non-destructive** — every mutation is reverted via `DeferCleanup`. Server-side dry-run (`--dry-run=server`) is used for "should be blocked" validation/admission tests so nothing is persisted.

---

## Project Structure

```
testing-day2-vcenter/
  go.mod
  go.sum
  pkg/
    framework/
      client.go          # K8s/OpenShift client initialization (KUBECONFIG)
      infrastructure.go  # Infrastructure CR backup/restore/patch helpers
      configmap.go       # ConfigMap snapshot, diff, ownership helpers
      featuregate.go     # Feature gate query helpers
      conditions.go      # ClusterOperator condition checks
      constants.go       # Timeouts, namespaces, resource names
    vsphere/
      types.go           # vCenter/FD builder types for test fixtures
      cloud_config.go    # YAML cloud config parser/comparator
  test/
    e2e/
      e2e_suite_test.go           # Ginkgo bootstrap + BeforeSuite/AfterSuite
      featuregate_test.go          # Feature gate existence and profile (PR #2783)
      infrastructure_validation_test.go  # CRD xValidation rules (PR #2784) — N-INF-*
      configmap_ownership_test.go  # CCO/CCCMO ownership migration (PR #442, #481) — N-OP-*
      configmap_content_test.go    # Config format parity, cleanup (PR #469, #10529) — N-CFG-*
      vap_test.go                  # ValidatingAdmissionPolicies (PR #1510) — N-SEQ-*
      operator_health_test.go      # Operator stability, CO conditions, race fix (PR #489)
      topology_lifecycle_test.go   # End-to-end add/remove FD + vCenter sequences
```

---

## Key Dependencies (go.mod)

- `github.com/onsi/ginkgo/v2`
- `github.com/onsi/gomega`
- `k8s.io/client-go` (dynamic client, discovery, rest config)
- `k8s.io/apimachinery` (unstructured, schema, types, strategic merge patch)
- `k8s.io/api` (core/v1, admissionregistration/v1)
- `github.com/openshift/api` (config/v1 for Infrastructure, FeatureGate types)
- `github.com/openshift/client-go` (typed OpenShift clients)
- `github.com/openshift/machine-api-operator/pkg/apis` (Machine, MachineSet, CPMS types)
- `gopkg.in/yaml.v3` (cloud config parsing)

---

## Framework Package (`pkg/framework/`)

### `client.go`
- `NewClients() (*Clients, error)` — builds rest.Config from `KUBECONFIG` env, returns struct holding:
  - `kubernetes.Interface` (core k8s)
  - `configclient.Interface` (openshift config)
  - `machineclient.Interface` (machine API)
  - `dynamic.Interface` (for unstructured patches)
  - `discovery.DiscoveryInterface`

### `infrastructure.go`
- `BackupInfrastructure(ctx, client) (*configv1.Infrastructure, error)` — deep-copy current `Infrastructure/cluster`
- `RestoreInfrastructure(ctx, client, backup *configv1.Infrastructure) error` — patch back to original state
- `PatchInfrastructure(ctx, client, patch []byte, dryRun bool) (*configv1.Infrastructure, error)` — apply strategic merge patch; when `dryRun=true` passes `fieldManager=e2e-test&dryRun=All` so API server validates without persisting
- `GetVCenters(infra *configv1.Infrastructure) []configv1.VSpherePlatformVCenterSpec`
- `GetFailureDomains(infra *configv1.Infrastructure) []configv1.VSpherePlatformFailureDomainSpec`

### `configmap.go`
- `GetConfigMap(ctx, client, namespace, name) (*corev1.ConfigMap, error)`
- `SnapshotConfigMap(ctx, client, namespace, name) (*ConfigMapSnapshot, error)` — captures data + annotations + resource version
- `CompareConfigMaps(a, b *ConfigMapSnapshot) []Diff` — field-level semantic diff of cloud config YAML
- `GetConfigMapOwner(cm *corev1.ConfigMap) string` — parse annotations for managing operator

### `featuregate.go`
- `IsFeatureGateEnabled(ctx, client, gateName string) (bool, error)` — checks `FeatureGate/cluster` status for the named gate
- `WaitForFeatureGate(ctx, client, gateName string, enabled bool, timeout time.Duration) error`

### `conditions.go`
- `WaitForClusterOperatorAvailable(ctx, client, name string, timeout time.Duration) error`
- `GetClusterOperatorCondition(ctx, client, name, conditionType string) (*configv1.ClusterOperatorStatusCondition, error)`
- `CheckOperatorNotDegraded(ctx, client, operators []string) error` — bulk check

### `constants.go`
```go
const (
    InfrastructureName              = "cluster"
    FeatureGateName                 = "cluster"
    VSphereMultiVCenterDay2Gate     = "VSphereMultiVCenterDay2"
    ManagedConfigNamespace          = "openshift-config-managed"
    ManagedConfigName               = "kube-cloud-config"
    CCMConfigNamespace              = "openshift-cloud-controller-manager"
    CCMConfigName                   = "cloud-conf"
    MachineAPINamespace             = "openshift-machine-api"
    DefaultTimeout                  = 5 * time.Minute
    DefaultPolling                  = 10 * time.Second
    ShortTimeout                    = 30 * time.Second
    LongTimeout                     = 15 * time.Minute
)
```

---

## Test Suite Bootstrap (`test/e2e/e2e_suite_test.go`)

```go
var (
    clients   *framework.Clients
    infraBackup *configv1.Infrastructure
    gateEnabled bool
)

var _ = BeforeSuite(func() {
    // 1. Initialize clients from KUBECONFIG
    // 2. Verify cluster is vSphere platform (skip otherwise)
    // 3. Check if VSphereMultiVCenterDay2 gate is enabled
    // 4. Backup Infrastructure CR (global safety net)
    // 5. Verify cluster operators are healthy before starting
})

var _ = AfterSuite(func() {
    // Restore Infrastructure CR if modified and not already restored
})
```

---

## Test Files — What Each Covers

### 1. `featuregate_test.go` — Feature Gate Verification
Source: Risk assessment PR #2783, Gap analysis section 3.2

| Test | Type | Method |
|---|---|---|
| Gate exists in FeatureGate/cluster status | Read-only | Get FeatureGate CR, inspect `.status.featureGates` |
| Gate assigned to correct profile (TechPreview) | Read-only | Check profile membership |
| Gate status matches expected enabled/disabled state | Read-only | Compare against cluster config |

### 2. `infrastructure_validation_test.go` — CRD xValidation (N-INF-*)
Source: Risk assessment PR #2784, Negative testing section 4.1

All "should be blocked" tests use **server-side dry-run** — no mutation persisted.

| Test | ID | Method |
|---|---|---|
| Add second vCenter (gate ON) — allowed | Positive | Dry-run patch adding vCenter entry |
| Duplicate vCenter server rejected | N-INF-01/02 | Dry-run patch with duplicate `server` |
| Reduce vcenters to empty array rejected | N-INF-03 | Dry-run patch with `vcenters: []` |
| Remove vcenters field rejected | N-INF-04 | Dry-run patch omitting field |
| Swap vCenter server address rejected | N-INF-05 | Dry-run changing existing entry's `server` |
| Add and remove vCenter simultaneously rejected | N-INF-06/07 | Dry-run patch with both operations |
| Gate OFF: add second vCenter rejected | N-INF-09 | Dry-run (skip if gate ON) |
| Gate OFF: remove only vCenter rejected | N-INF-10 | Dry-run (skip if gate ON) |
| Ratcheting: update non-vCenter fields with edge-case config | Positive | Dry-run patching unrelated field |
| >3 vCenters rejected | N-INF-11 | Dry-run with 4+ entries |

Pattern for each validation test:
```go
It("should reject duplicate vCenter server values", func() {
    patch := buildVCenterPatch(duplicateServerEntries)
    _, err := framework.PatchInfrastructure(ctx, clients, patch, true) // dryRun=true
    Expect(err).To(HaveOccurred())
    Expect(err.Error()).To(ContainSubstring("vcenters must have unique server values"))
})
```

### 3. `configmap_ownership_test.go` — CCO/CCCMO Migration (N-OP-*)
Source: Risk assessment PR #442/#481, Gap analysis section 4, Negative testing section 4.3

Read-only observation tests (no mutations):

| Test | ID | Method |
|---|---|---|
| `kube-cloud-config` exists in `openshift-config-managed` | Positive | Get ConfigMap |
| ConfigMap has correct managing operator annotation | Positive/N-OP-* | Check annotations |
| Gate ON: CCCMO manages, CCO does not write | N-OP-01/02 | Check CM annotations + operator logs |
| CCCMO RBAC includes ConfigMap create/update/patch | Positive | Check ClusterRole/RoleBinding |
| Both operators not fighting (no rapid resourceVersion changes) | N-OP-04 | Consistently check CM is stable over 60s |
| Managed ConfigMap recreated if deleted | N-OP-07 | Delete + Eventually check recreation (ONLY if gate ON) |
| Gate ON on non-vSphere cluster is no-op | N-OP-08 | Skip if vSphere; verify no managed CM changes |

### 4. `configmap_content_test.go` — Config Format Parity (N-CFG-*)
Source: Risk assessment PR #469/#10529, Gap analysis section 5, Negative testing section 4.4

| Test | ID | Method |
|---|---|---|
| Three-way parity: managed CM vs CCM CM vs Infrastructure spec | N-CFG-06, Gap 5.1 | Semantic comparison of YAML content |
| InsecureFlag only in global section, not per-vCenter | Gap 5.1 | Parse YAML, check structure |
| All Infrastructure vCenters present in cloud config | Positive | Cross-reference vCenter entries |
| Removed vCenter not in cloud config (#469 regression) | N-CFG-06 | Read cloud config, verify no stale entries |
| Cloud config YAML is valid (parseable) | N-CFG-01/02/03 | Parse managed CM data |
| Labels/topology from Infrastructure reflected in config | Gap 5.1 | Compare topology fields |
| Node network CIDR present if configured (installer #10614) | Gap 3.2 | Check `Nodes` section in config |

### 5. `vap_test.go` — ValidatingAdmissionPolicies (N-SEQ-*)
Source: Risk assessment PR #1510, Gap analysis section 6, Negative testing section 4.2

VAP existence tests are read-only. Denial tests use server-side dry-run against Infrastructure CR.

| Test | ID | Method |
|---|---|---|
| VAP resources exist (3 policies + 3 bindings) | Positive | List VAPs, check names |
| VAPs are active (not disabled) when gate ON | Gap 6 | Check VAP `.spec` |
| Remove FD with active Machine — denied | N-SEQ-01 | Dry-run patch removing FD, expect VAP denial |
| Remove FD referenced by CPMS — denied | N-SEQ-02 | Dry-run patch, expect denial |
| Remove FD referenced by MachineSet — denied | N-SEQ-03 | Dry-run patch, expect denial |
| Remove FD with 0-replica MachineSet — denied | N-SEQ-03 variant | Dry-run patch, expect denial |
| Remove unreferenced FD — allowed | Positive | Dry-run patch (may need to identify unreferenced FD) |
| Machine without region/zone labels — FD removal allowed | Gap 6 | Dry-run after confirming label state |
| VAPs inactive/absent when gate OFF | Gap 6 | Skip if gate ON; verify no VAP objects |

Expected denial message pattern:
```
Infrastructure update would remove vSphere failure domain (region="<r>", zone="<z>") that is still in use by Machine '<name>'
```

### 6. `operator_health_test.go` — Operator Stability
Source: Risk assessment PR #489, Gap analysis section 4, Negative testing section 4.3

All read-only:

| Test | Method |
|---|---|
| ClusterOperator/cloud-controller-manager is Available, not Degraded | Check CO conditions |
| ClusterOperator/kube-controller-manager is Available, not Degraded | Check CO conditions |
| ClusterOperator/machine-api is Available, not Degraded | Check CO conditions |
| CCCMO pods are running, no restart loops | Check pod status + restart count |
| CCO pods are running, no restart loops | Check pod status + restart count |
| MAO pods are running, no restart loops | Check pod status + restart count |
| CCM pods are running, no crash loops | Check pod restart count over time window |

### 7. `topology_lifecycle_test.go` — End-to-End Sequences
Source: Gap analysis section 3.2, Negative testing section 4.2

These are the **only tests that mutate cluster state** (with full backup/restore).

Gated by build tag and/or Ginkgo label `mutating` — skipped by default, opt-in only.

| Test | Method |
|---|---|
| Add second vCenter: Infrastructure updated, cloud config regenerated, operators healthy | Patch infra, Eventually check CM, check COs; DeferCleanup restore |
| Add failure domain mapped to new vCenter | Patch infra with FD + vCenter; verify config; restore |
| Ordered shrink: remove FD then vCenter (correct order) | Sequential patches; verify each step; restore |
| Verify cloud config updated within timeout after Infrastructure change | Patch + Eventually compare CM content |

---

## Non-Destructive Strategy Summary

| Technique | Used For |
|---|---|
| **Server-side dry-run** (`dryRun=All`) | All xValidation and VAP denial tests — triggers full admission chain without persisting |
| **Read-only observation** | ConfigMap content, operator health, feature gate status, VAP existence |
| **DeferCleanup + backup/restore** | Any test that mutates Infrastructure CR (topology lifecycle tests only) |
| **Ginkgo labels** | `mutating` label on tests that change cluster state; default runs are read-only |
| **Consistently** | Verify no thrashing (CM stable, no operator restarts over observation window) |
| **Eventually** | Wait for async reconciliation after any mutation |

---

## Clean Slate Strategy

1. **BeforeEach**: Snapshot current Infrastructure CR and relevant ConfigMaps
2. **DeferCleanup** (registered in BeforeEach): Restore Infrastructure CR to snapshot if modified
3. **Ordered containers**: Where test sequence matters (e.g., add vCenter before testing removal)
4. **Independent tests**: Each Describe block is self-contained — no cross-file dependencies
5. **Skip conditions**: Tests skip if preconditions aren't met (wrong platform, gate state, missing resources)

---

## Ginkgo Labels and Filtering

```
Labels:
  "readonly"   — safe to run on any cluster, no mutations
  "mutating"   — modifies Infrastructure CR (backup/restore)
  "validation" — CRD xValidation tests
  "admission"  — VAP tests  
  "operator"   — operator health/behavior tests
  "config"     — ConfigMap content/ownership tests
  "p0"         — P0 priority from test matrix
  "p1"         — P1 priority
  "p2"         — P2 priority
```

Run examples:
- `ginkgo --label-filter="readonly"` — safe default
- `ginkgo --label-filter="p0 && readonly"` — P0 non-destructive only
- `ginkgo --label-filter="mutating"` — full lifecycle tests (requires restore capability)

---

## Verification

After implementation:
1. `go vet ./...` — no warnings
2. `go build ./...` — compiles
3. `ginkgo --dry-run ./test/e2e/` — test tree renders correctly (no cluster needed)
4. On a vSphere cluster: `ginkgo --label-filter="readonly" ./test/e2e/` — read-only tests pass
5. On a vSphere cluster: `ginkgo --label-filter="mutating" ./test/e2e/` — full lifecycle tests (opt-in)

### Implementation status (2026-06-29)

Implemented in repo:

- `pkg/framework/` — clients, Infrastructure patch/restore, ConfigMap helpers, feature gates, CO conditions
- `pkg/vsphere/` — cloud config YAML parsing and Infrastructure helpers
- `test/e2e/` — 37 specs across 9 files (dry-run verified)
- `Makefile` — `vet`, `build`, `test-readonly`, `test-p0`, `test-mutating`

VAP dry-run behavior is probed at suite startup (`vapDryRunWorks`); denial tests fall back to real patch when dry-run does not trigger VAP.

---

## QE Review (2026-06-29)

**Reviewer role**: QA/QE sign-off on plan vs [risk assessment](../vsphere-multi-vcenter-day2-risk-assessment.md), [gap analysis](../vsphere-multi-vcenter-day2-gap-analysis.md), and [negative testing plan](../vsphere-multi-vcenter-day2-negative-testing.md).

**Verdict**: **Approve with changes.** Strong QE automation blueprint for rule-based and read-only validation. Not yet sufficient as the sole P0 sign-off vehicle — core Day 2 mutations are opt-in, operator skew/credentials/detector are missing, and VAP+dry-run is assumed.

### Strengths

1. **Fused scope** — Files map to PRs and negative IDs (`N-INF-*`, `N-SEQ-*`, etc.), aligned with QE automates rules / QA explores chaos.
2. **Safety model** — `BeforeSuite` backup, `DeferCleanup`, dry-run for admission/xValidation, and `mutating` as opt-in are appropriate for a shared vSphere lab.
3. **High-value automation targets** — xValidation (#2784), VAP denials (#1510), config parity (#469, #442), operator health (#489) cover most P0 rule-based negatives.
4. **Labels and run modes** — `readonly` / `p0` / `mutating` filtering is practical for CI tiers.
5. **Framework helpers** — Infrastructure backup/restore, semantic ConfigMap diff, and feature-gate helpers are the right abstractions.

### Critical gaps (address before / during implementation)

#### 1. P0 end-to-end scenarios are opt-in only

Risk matrix P0 items (add second vCenter Day 2, remove unused vCenter, upgrade + enable gate) live only in `topology_lifecycle_test.go` behind `mutating`, skipped by default. CI running `readonly` only never signs off the feature’s core value.

**Action required** — Define CI tiers:

| Tier | Filter | When |
|---|---|---|
| PR smoke | `p0 && readonly` | Every run |
| Nightly | `p0` (includes `mutating` on dedicated lab) | Required before payload sign-off |
| Manual | Exploratory charter | Pre-release |

#### 2. N-OP-01 / N-OP-02 are misclassified

Listed as read-only observation (annotations + logs). These cases are **version skew during CVO rollout** (CCO updated before CCCMO or reverse). Cannot be validated on a single steady-state cluster.

**Action required** — Move N-OP-01/02 to manual QA or a separate upgrade job. Replace in `configmap_ownership_test.go` with **steady-state single-writer** observation, or rename tests accordingly.

#### 3. Wrong ClusterOperator for CCO

`operator_health_test.go` checks `kube-controller-manager`, not **`cluster-config-operator`**. CCO owns `kube-cloud-config` skip logic (#481, #489).

**Action required** — Add `cluster-config-operator` to operator health checks.

#### 4. VAP + dry-run is unverified (do not assume)

VAPs use param resources (one evaluation per Machine/CPMS/MachineSet). Server-side dry-run on `Infrastructure/cluster` may or may not run the full VAP chain depending on Kubernetes/OpenShift version and binding configuration.

**Action required** — First implementation spike:

```text
Prove: PatchInfrastructure(..., dryRun=true) removing an in-use FD returns VAP denial, not success.
If not: fall back to mutating test with immediate restore, or document alternative approach.
```

Document result in this plan before completing `vap_test.go`.

#### 5. Gate-OFF negatives will rarely execute

N-INF-09/10 and “VAPs absent when gate OFF” skip when gate is ON. TechPreview lab clusters will almost always have the gate enabled.

**Action required** — Either:

- Dedicated gate-off vSphere profile for those tests, **or**
- Rely on openshift/api CRD fixture tests for gate-off rules and document in plan with explicit `Skip` reason + link to fixture coverage.

### Coverage gaps vs negative-testing plan (41 cases)

| Area | Missing / weak in plan | Priority |
|---|---|---|
| N-SEQ-04/05 | Remove vCenter while FD still references it; wrong-order shrink | P0 — add dry-run or mutating |
| N-INF-12 | FD points at removed vCenter server | P0 |
| N-CFG-04/05 | New vCenter without credentials / wrong password | P0 — manual QA or mutating + secret |
| N-CFG-08 | Problem detector after FD removal (#224) | P0 when PR merges — no test file planned |
| N-OP-03 | Gate disable after migration | P1 |
| N-OP-05 | CCO startup before FG observed | P1 — manual/degraded lab |
| N-ENV-* | Degraded API, vCenter down, partition | P1/P2 — OK out of scope if documented |
| ControllerConfig xValidation | Gap analysis §2 | P1 — no test file |
| Bootstrap CM | Three-way parity omits `openshift-config` source | P1 — gap §5.1 |
| CCCMO FG symmetry | Gap §4.1 — dynamic gate vs CCO | P1 |
| CSI / Routes / MAPI scale | Gap §2 consumers | P2 — acceptable out of scope if stated |

**Rough coverage today**: ~25/41 negative IDs touched; ~12 P0 negatives missing or mislabeled.

### Technical / design concerns

1. **N-OP-07 delete ConfigMap** — Listed under read-only but deletes a ConfigMap. Must be `mutating` with `DeferCleanup` or a dedicated destructive label.
2. **`topology_lifecycle_test.go`** — “Ordered shrink” is happy path only. Add explicit negative dry-runs: remove FD with Machines present (before mutating shrink suite).
3. **Stale vCenter (#469)** — `configmap_content_test.go` read-only only proves regression if cluster already had a removal. Need mutating: add vCenter → remove vCenter → assert absent from YAML.
4. **Dependency risk** — `machine-api-operator/pkg/apis` is brittle. Prefer `github.com/openshift/api/machine/v1beta1` + client-go.
5. **`GetConfigMapOwner`** — Confirm CCCMO/CCO set a consistent managing-operator annotation; if not, test will flake.
6. **No `problem_detector_test.go`** — For #224, even read-only checks (CO `vsphere-problem-detector`) would close a gap.
7. **Ratcheting** — Only one positive dry-run noted. Port cases from `AAA_ungated.yaml` or document delegation to openshift/api tests.

### Alignment with planning docs

| Document | Alignment |
|---|---|
| Risk assessment P0 matrix | Partial — validation yes; lifecycle mutating under-prioritized in CI |
| Gap analysis | Missing consumers (detector, CSI, credentials, ControllerConfig) |
| Negative testing QE/QA split | Strong on QE; needs explicit manual QA remainder section |

### Out of scope for Ginkgo (Manual QA)

Document explicitly; do not imply Ginkgo covers full P0 sign-off:

- Operator version skew during upgrade (N-OP-01/02)
- Credential / cloud-credentials negative paths (N-CFG-04/05)
- Environmental chaos (N-ENV-*)
- Exploratory wrong-order shrink without automated restore safety
- Hypershift variant

### Recommended plan edits (priority order)

- [ ] Fix CO name: `cluster-config-operator` (not `kube-controller-manager` for CCO)
- [ ] Add VAP dry-run spike / fallback strategy to Verification section
- [ ] Split steady-state ownership tests from upgrade skew; re-home skew to manual QA
- [ ] Add `problem_detector_test.go` (stub until vsphere-problem-detector#224 merges)
- [ ] Add negative dry-runs: N-SEQ-04, N-INF-12, vCenter removal with FD still referencing server
- [ ] Define CI tiers so `p0 && mutating` runs nightly on dedicated cluster
- [ ] Relabel N-OP-07 as `mutating`
- [ ] Extend parity check to source CM in `openshift-config` when present
- [ ] Document gate-off coverage strategy (second cluster vs api fixtures)
- [ ] Add **Out of scope for Ginkgo** section to main plan body (above)

### Response requested from implementing agent

Please reply in this document (new subsection below) or in PR comments with:

1. Acceptance or pushback on each **Critical gap** item (1–5)
2. Updated test file list if adding `problem_detector_test.go` / ControllerConfig tests
3. Result of VAP dry-run spike (pass/fail + OpenShift version)
4. CI tier definition for `mutating` P0 tests
5. Checkbox updates on **Recommended plan edits**

### Implementation Response

#### Critical Gap Responses

**1. P0 end-to-end opt-in only — ACCEPTED.**

CI tiers defined:

| Tier | Filter | When | Cluster |
|---|---|---|---|
| PR smoke | `p0 && readonly` | Every PR, every push | Any vSphere cluster |
| Nightly | `p0` (includes `mutating`) | Nightly job | Dedicated disposable vSphere lab |
| Full | all labels | Pre-release | Dedicated lab + manual QA |

The `mutating` label is opt-in at the Ginkgo filter level, not at the code level. On nightly CI, the job definition passes `--label-filter="p0"` which includes mutating P0 tests. This is an operational distinction, not a code change.

**2. N-OP-01/02 misclassified — ACCEPTED.**

N-OP-01 (CCO upgraded, CCCMO not) and N-OP-02 (reverse) are version-skew scenarios that require controlled CVO rollout ordering. These cannot be tested on a single steady-state cluster.

Changes:
- Remove N-OP-01/02 from `configmap_ownership_test.go`
- Replace with **steady-state single-writer assertion**: verify only one operator is actively reconciling `kube-cloud-config` by observing annotation consistency + no resourceVersion churn over 2 minutes
- Add N-OP-01/02 to the **Out of scope for Ginkgo** section as manual QA / upgrade job

**3. Wrong ClusterOperator for CCO — ACCEPTED.**

Fix: Replace `kube-controller-manager` with `cluster-config-operator` in `operator_health_test.go`. The relevant COs are:
- `cloud-controller-manager` (CCCMO)
- `cluster-config-operator` (CCO — owns skip logic)
- `machine-api` (MAO — owns VAPs)

**4. VAP + dry-run unverified — ACCEPTED.**

Adding a verification spike to the implementation plan:

**Spike approach**: Before writing `vap_test.go`, run a manual probe:
```go
// In e2e_suite_test.go BeforeSuite or a dedicated spike test:
// 1. Get current Infrastructure CR
// 2. Identify an in-use FD (Machine exists with matching region/zone labels)
// 3. Build patch removing that FD
// 4. Apply with dryRun=All
// 5. If error contains VAP denial message → dry-run works, proceed as planned
// 6. If no error → dry-run bypasses VAPs, fall back to:
//    a. Mutating test: apply patch (real), assert denial, no restore needed (rejected = no change)
//    b. OR if VAPs only fire on successful admission: mutating + immediate restore
```

**Fallback strategy if dry-run bypasses VAPs**: Use real (non-dry-run) patches for VAP denial tests. Since admission denial means the patch is rejected and no state change occurs, these are effectively non-destructive even without dry-run. The only risk is if the VAP has a bug and allows the patch — mitigated by `DeferCleanup` restore.

Result will be documented in this plan before `vap_test.go` is finalized.

**5. Gate-OFF negatives rarely execute — ACCEPTED.**

Strategy: **Delegate gate-off xValidation to openshift/api fixture tests** (already covered by `VSphereMultiVCenterDay2.yaml` and `AAA_ungated.yaml` in the api repo). Our Ginkgo tests for N-INF-09/10 will:
- Include the test code with `Skip` + reason pointing to api repo fixture coverage
- Only run if gate is actually OFF (e.g., on a Default profile cluster)
- Document this delegation explicitly in the test file

This avoids maintaining a separate gate-off vSphere profile just for 2 test cases already covered upstream.

#### Coverage Gap Responses

| Gap | Response |
|---|---|
| N-SEQ-04 (remove vCenter while FD references it) | **Add** to `infrastructure_validation_test.go` as dry-run negative |
| N-SEQ-05 (wrong-order shrink) | **Add** to `topology_lifecycle_test.go` as dry-run negative before the happy-path ordered shrink |
| N-INF-12 (FD points at removed vCenter) | **Add** to `infrastructure_validation_test.go` — dry-run removing vCenter while FD still references its server |
| N-CFG-04/05 (credentials) | **Out of scope** — requires real vCenter endpoints + secret manipulation. Manual QA |
| N-CFG-08 (problem detector) | **Add `problem_detector_test.go`** as stub (read-only CO health check). Full tests when #224 merges |
| N-OP-03 (gate disable rollback) | **Out of scope** — toggling feature gates on TechPreview clusters is not supported operationally. Manual QA |
| N-OP-05 (CCO startup timing) | **Out of scope** — requires degraded cluster. Manual QA |
| N-ENV-* | **Out of scope** — environmental chaos. Manual QA |
| ControllerConfig xValidation | **Add** to `infrastructure_validation_test.go` — dry-run ControllerConfig patches to verify parallel xValidation rules |
| Bootstrap CM three-way parity | **Extend** `configmap_content_test.go` — include source CM from `openshift-config` if it exists |
| CCCMO FG symmetry (gap §4.1) | **Add** to `configmap_ownership_test.go` — check CCCMO deployment for dynamic feature gate accessor env/args. Read-only |
| CSI / Routes / scale | **Out of scope** for initial implementation. Documented below |

#### Technical Concern Responses

1. **N-OP-07 (delete ConfigMap)** — Agreed, relabel as `mutating`. Move to a dedicated Context within `configmap_ownership_test.go` with its own `DeferCleanup` that recreates the CM from snapshot if the operator doesn't recreate it within timeout.

2. **Topology lifecycle negative dry-runs** — Agreed. Add dry-run negatives (remove FD with Machines present) as a preamble to the happy-path ordered shrink in `topology_lifecycle_test.go`.

3. **Stale vCenter #469** — Agreed. The read-only check only works if the cluster has already exercised add/remove. Add a mutating variant: add vCenter → remove vCenter → assert absent from cloud config YAML → restore.

4. **Dependency risk (machine-api-operator/pkg/apis)** — Agreed. Use `github.com/openshift/api/machine/v1beta1` and `github.com/openshift/api/machine/v1` (for CPMS) with `client-go` dynamic or generated clients. Drop `machine-api-operator` as a direct dependency.

5. **GetConfigMapOwner annotation** — Will investigate what annotation CCCMO/CCO actually set. If no consistent annotation exists, fall back to checking operator logs or resourceVersion change attribution via audit events. Document finding during implementation.

6. **`problem_detector_test.go`** — Adding as a new file. Initially read-only: check `ClusterOperator/vsphere-problem-detector` Available/not Degraded. Stub tests for #224 scenarios with `Skip("waiting for vsphere-problem-detector#224")`.

7. **Ratcheting** — Port key cases from `AAA_ungated.yaml` as dry-run tests. Document that full ratcheting coverage is delegated to openshift/api repo fixtures.

#### Updated File List

```
test/e2e/
  e2e_suite_test.go
  featuregate_test.go
  infrastructure_validation_test.go  # now includes ControllerConfig, N-SEQ-04, N-INF-12
  configmap_ownership_test.go        # steady-state single-writer, N-OP-07 as mutating
  configmap_content_test.go          # extended three-way parity with openshift-config source
  vap_test.go                        # includes dry-run spike result
  operator_health_test.go            # fixed: cluster-config-operator, not kube-controller-manager
  topology_lifecycle_test.go         # negative dry-runs before happy-path mutations
  problem_detector_test.go           # NEW: stub for #224, CO health read-only
```

#### Out of Scope for Ginkgo (Manual QA)

These scenarios require infrastructure not available in automated e2e:

| Scenario | IDs | Reason |
|---|---|---|
| Operator version skew during CVO upgrade | N-OP-01/02 | Requires controlled rollout ordering |
| Credential / cloud-credentials negative paths | N-CFG-04/05 | Requires real vCenter endpoints + secret setup |
| Gate disable after Day 2 migration (rollback) | N-OP-03 | TechPreview gate toggle not operationally supported |
| CCO startup before FG observed (degraded cluster) | N-OP-05 | Requires deliberately degraded API server |
| Environmental chaos (slow API, vCenter down, partition) | N-ENV-01–05 | Requires fault injection infrastructure |
| Exploratory wrong-order shrink without safety net | N-SEQ-06 | Concurrent admin edits — manual |
| Hypershift variant | Gap §3.2 | Different control plane topology |
| CSI volume lifecycle after topology change | Gap §2 | Requires real storage + vCenter endpoints |
| Routes / LoadBalancer after config migration | Gap §2 | Requires real CCM cloud provider |

#### Recommended Plan Edits — Status

- [x] Fix CO name: `cluster-config-operator` (not `kube-controller-manager` for CCO)
- [x] Add VAP dry-run spike / fallback strategy to Verification section
- [x] Split steady-state ownership tests from upgrade skew; re-home skew to manual QA
- [x] Add `problem_detector_test.go` (stub until vsphere-problem-detector#224 merges)
- [x] Add negative dry-runs: N-SEQ-04, N-INF-12, vCenter removal with FD still referencing server
- [x] Define CI tiers so `p0 && mutating` runs nightly on dedicated cluster
- [x] Relabel N-OP-07 as `mutating`
- [x] Extend parity check to source CM in `openshift-config` when present
- [x] Document gate-off coverage strategy (delegate to api fixtures with Skip reason)
- [x] Add **Out of scope for Ginkgo** section to main plan body
