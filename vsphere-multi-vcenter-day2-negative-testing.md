# Negative Testing Plan: vSphere Multi-vCenter Day 2

**Feature gate**: `VSphereMultiVCenterDay2`  
**Related docs**: [risk assessment](./vsphere-multi-vcenter-day2-risk-assessment.md), [gap analysis](./vsphere-multi-vcenter-day2-gap-analysis.md)  
**Approach**: Fused [QA + QE](https://www.testlio.com/blog/qa-vs-qe-approach) with structured [negative testing](https://medium.com/@abdulrehmanrizwan81/the-importance-of-negative-testing-in-qa-and-how-to-approach-it-effectively-77b0366e3465)

---

## 1. Why negative testing matters here

Positive testing proves Day 2 vCenter add/remove works on a healthy cluster. **Negative testing** proves the platform **fails safely** when admins, operators, or the environment do the wrong thing:

- Invalid `Infrastructure` patches are **rejected** with actionable errors (not partial apply / silent corruption).
- Topology shrink is **blocked** while Machines, CPMS, or MachineSets still reference a failure domain.
- Operators do **not** fight over `kube-cloud-config` or emit stale multi-vCenter config after removal.
- Cluster health (CCM, CSI, problem detector) **degrades predictably** rather than intermittently.

For this feature, negative paths are as important as happy paths because mistakes touch **cluster-wide infrastructure state**.

---

## 2. QA vs QE: fused approach for this feature

Per Testlio’s model, QA and QE are complementary—not either/or.

| Dimension | **QE (prevent defects early)** | **QA (find defects before release)** |
|---|---|---|
| **Focus** | Shift-left guardrails, contract tests, CI signal | Real vSphere clusters, exploratory abuse, release sign-off |
| **When** | PR merge → nightly → pre-release payload | TechPreview builds, customer-like topology, upgrade paths |
| **Negative testing role** | Automate API/xValidation matrix; unit-level operator tests; lint CRD fixtures | Manual invalid patches, operator skew, credential gaps, multi-step shrink in wrong order |
| **Strength** | Repeatable, fast regression on known rules | Edge cases automation misses (timing, vCenter env, human error sequences) |
| **This feature** | Port `VSphereMultiVCenterDay2.yaml` cases; VAP denial e2e in CI if feasible | vCenter connectivity, MCO/CCM behavior after rejected vs accepted patches |

### Division of labor

```
┌─────────────────────────────────────────────────────────────────┐
│  QE (shift-left)                                                │
│  • CRD/xValidation contract tests (openshift/api fixtures)      │
│  • MAO VAP unit/sync tests                                      │
│  • CCCMO transformer error cases (empty/malformed cloud.conf)   │
│  • CCO “skip when gate on” unit tests                           │
│  • Optional: e2e template patches against fake/apiservers       │
└───────────────────────────┬─────────────────────────────────────┘
                            │ failures → fix before QA cycle
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  QA (shift-right / release)                                   │
│  • Live cluster invalid Infrastructure edits                    │
│  • Wrong-order topology reduction (exploratory)                 │
│  • Operator version skew + feature gate flip                    │
│  • Missing credentials / bad vCenter endpoints                  │
│  • Observability: alerts, CO degraded, config map contents      │
└─────────────────────────────────────────────────────────────────┘
```

**Rule**: QE owns *known invalid inputs* encoded in code/CRDs. QA owns *invalid operations in realistic clusters* and *ambiguous failure modes* called out in the gap analysis.

---

## 3. System boundaries (understand before designing cases)

Negative tests must respect what the system **allows vs forbids** when `VSphereMultiVCenterDay2` is **enabled**.

### 3.1 Infrastructure / vCenters (CRD xValidation)

| Boundary | Gate ON behavior | Gate OFF behavior (legacy) |
|---|---|---|
| vCenters in spec | Required once set; cannot remove field entirely | Cannot add/remove after initial set (max 1 post-install) |
| vCenter count | Up to 3; cannot reduce to 0 | minItems ratcheting allows empty *persisted* legacy state |
| Same operation | Cannot add **and** remove in one update | N/A |
| Server identity | Cannot swap server URL for existing entry | N/A |
| Uniqueness | Duplicate `server` values rejected | Same |

**Authoritative messages** (from [openshift/api#2784](https://github.com/openshift/api/pull/2784) tests):

- `vcenters must have unique server values`
- `vcenters is required once set and cannot be removed`
- `Cannot add and remove vCenters at the same time`
- `spec.platformSpec.vsphere.vcenters in body should have at least 1 items`

### 3.2 Failure domains (VAP — MAO #1510)

| Boundary | Enforcement |
|---|---|
| Remove FD referenced by Machine (region+zone labels) | **Deny** — VAP `vsphere-failure-domain-in-use-by-machine` |
| Remove FD referenced by CPMS template | **Deny** — VAP `vsphere-failure-domain-in-use-by-cpms` |
| Remove FD referenced by MachineSet template (incl. 0 replicas) | **Deny** — VAP `vsphere-failure-domain-in-use-by-machineset` |
| Machine without region/zone labels | **Allow** FD removal (VAP passes) |
| No param objects (no Machines) | **Allow** (`ParamNotFoundAction: Allow`) — QA must validate this is intentional |

### 3.3 Operators / ConfigMaps

| Boundary | Expected negative outcome |
|---|---|
| Gate ON, vSphere | CCO **must not** update `openshift-config-managed/kube-cloud-config` |
| Gate OFF, vSphere | CCCMO **must not** own managed ConfigMap path |
| Both writers active (skew/race) | **No** thrashing; single writer — cluster must not oscillate |
| Removed vCenter in Infrastructure | **Must not** appear in CCCMO-generated YAML (#469) |

### 3.4 External dependencies

| Invalid condition | Expected behavior |
|---|---|
| New vCenter in Infrastructure, no matching credential secret | Provision/connect failures; clear errors (QA) |
| Malformed INI/YAML in source ConfigMap | CCCMO sync error; CO not silently Available |
| vCenter unreachable after valid patch | Infrastructure accepted; CCM/CSI/detector report failure (not API rejection) |

---

## 4. Negative scenario catalog

Scenarios grouped by **invalid input**, **invalid sequence**, **environment/timing**, and **security/abuse**.

### 4.1 Invalid Infrastructure input (API layer)

| ID | Scenario | Pri | QE | QA |
|---|---|---|---|---|
| N-INF-01 | Duplicate vCenter `server` on create | P0 | ✓ fixture | Smoke on cluster |
| N-INF-02 | Duplicate `server` when adding 2nd vCenter | P0 | ✓ fixture | ✓ |
| N-INF-03 | Reduce `vcenters` to `[]` | P0 | ✓ fixture | ✓ |
| N-INF-04 | Omit `vcenters` field after it was set | P0 | ✓ fixture | ✓ |
| N-INF-05 | Swap vcenter2 → vcenter3 (same count) | P0 | ✓ fixture | ✓ |
| N-INF-06 | Add vcenter3 + remove vcenter2 (2→3) | P0 | ✓ fixture | ✓ |
| N-INF-07 | Add vcenter4 + remove vcenter2/3 (3→2) | P0 | ✓ fixture | ✓ |
| N-INF-08 | Add vSphere platform post-install with 0 vCenters | P0 | ✓ fixture | — |
| N-INF-09 | Gate **OFF**: add 2nd vCenter Day 2 | P0 | ✓ ungated fixture | ✓ |
| N-INF-10 | Gate **OFF**: remove only vCenter | P0 | ✓ ungated fixture | ✓ |
| N-INF-11 | >3 vCenters | P1 | schema max | ✓ |
| N-INF-12 | FD references removed vCenter server | P0 | — | ✓ (API or runtime) |
| N-INF-13 | Malformed topology (empty datacenter, bad paths) | P1 | partial | ✓ |
| N-INF-14 | Invalid port / negative port | P2 | schema | ✓ |

### 4.2 Invalid topology shrink sequence (admission + exploratory)

Correct order: **scale/drain Machines → remove FD → remove vCenter**. Negative tests **deliberately violate** order.

| ID | Scenario | Pri | QE | QA |
|---|---|---|---|---|
| N-SEQ-01 | Remove FD while Machine exists in that region/zone | P0 | VAP unit | ✓ live deny |
| N-SEQ-02 | Remove FD while CPMS still lists it | P0 | VAP unit | ✓ |
| N-SEQ-03 | Remove FD while 0-replica MachineSet template references it | P0 | VAP unit | ✓ |
| N-SEQ-04 | Remove vCenter while FD still points at its server | P0 | — | ✓ |
| N-SEQ-05 | Remove vCenter before draining Machines on that FD | P0 | — | ✓ exploratory |
| N-SEQ-06 | Concurrent `kubectl edit` Infrastructure (two admins) | P2 | — | ✓ |

**Expected VAP denial (Machine)** — substring match:

```text
Infrastructure update would remove vSphere failure domain (region="<r>", zone="<z>") that is still in use by Machine '<name>'
```

### 4.3 Operator / feature-gate / timing negatives

| ID | Scenario | Pri | QE | QA |
|---|---|---|---|---|
| N-OP-01 | Enable gate: CCO upgraded, CCCMO not yet | P0 | — | ✓ |
| N-OP-02 | Enable gate: CCCMO upgraded, CCO not yet | P0 | — | ✓ |
| N-OP-03 | Toggle gate OFF after Day 2 edits | P1 | — | ✓ |
| N-OP-04 | Toggle gate ON/OFF rapidly | P2 | — | ✓ |
| N-OP-05 | CCO restart before FG observed (5m window) | P1 | unit | ✓ degraded API |
| N-OP-06 | Manual edit of `kube-cloud-config` (human bypass) | P1 | — | ✓ reconciled or error |
| N-OP-07 | Delete managed ConfigMap while gate ON | P1 | — | ✓ recreated |
| N-OP-08 | Gate ON on AWS cluster (should ignore vSphere gate) | P1 | ✓ CCO unit | ✓ |

### 4.4 Config / credential / consumer negatives

| ID | Scenario | Pri | QE | QA |
|---|---|---|---|---|
| N-CFG-01 | Empty `cloud-provider-config` in openshift-config | P0 | ✓ transformer unit | ✓ |
| N-CFG-02 | Malformed INI in source ConfigMap | P0 | ✓ unit | ✓ |
| N-CFG-03 | Malformed YAML in source ConfigMap | P0 | ✓ unit | ✓ |
| N-CFG-04 | Add vCenter without cloud-credentials entry | P0 | — | ✓ |
| N-CFG-05 | Wrong password for new vCenter | P1 | — | ✓ |
| N-CFG-06 | After vCenter removal, old entry in `cloud-conf` | P0 | ✓ #469 unit | ✓ |
| N-CFG-07 | CSI PVC in removed FD topology | P1 | — | ✓ |
| N-CFG-08 | Problem detector on node after FD removed (OCPBUGS-87906) | P0 | unit (#224) | ✓ |

### 4.5 Environmental / resilience negatives

| ID | Scenario | Pri | QE | QA |
|---|---|---|---|---|
| N-ENV-01 | API server slow during CCO startup (FG wait) | P1 | — | ✓ |
| N-ENV-02 | etcd brief unavailable during Infrastructure patch | P2 | — | ✓ |
| N-ENV-03 | vCenter down during valid Infrastructure add | P1 | — | ✓ |
| N-ENV-04 | Network partition between cluster and one vCenter | P1 | — | ✓ |
| N-ENV-05 | 100+ Machines: invalid FD removal latency/timeout | P2 | — | ✓ |

---

## 5. Test case template (use for QA execution)

Each negative case should document:

| Field | Content |
|---|---|
| **ID** | e.g. N-INF-05 |
| **Owner** | QE / QA / Both |
| **Preconditions** | Gate state, cluster topology, Machine count |
| **Invalid action** | Exact patch or sequence |
| **Steps** | Numbered reproduction |
| **Expected result** | Request rejected **or** controlled degradation |
| **Expected error** | Exact API message, VAP reason, or CO condition |
| **Must NOT happen** | Stale vCenter in CM, dual writers, silent success |
| **Evidence** | `kubectl`, must-gather, CM diff, events |

### Example: N-INF-05 (swap vCenter server)

**Preconditions**: Gate ON; 2 vCenters (`vc1`, `vc2`); FDs intact.

**Invalid action**: Replace `vc2` server with `vc3.example.com` without remove+add flow.

**Steps**:
1. `kubectl patch infrastructure cluster --type=merge ...` (swap server on second entry).
2. Observe API response.

**Expected**: HTTP 422; body contains `Cannot add and remove vCenters at the same time`.

**Must NOT happen**: Partial spec update; ConfigMap regenerated.

**Owner**: QE (fixture) + QA (live cluster smoke).

---

## 6. Automation strategy (QE)

Align with Testlio: automate **repeatable, rule-based** negatives; leave **exploratory sequences** to QA.

| Layer | Automate | Tool / source |
|---|---|---|
| CRD xValidation | All `Should not` rows in `VSphereMultiVCenterDay2.yaml` | openshift/api CRD tests (already in repo) |
| Gate-off regressions | Ungated vCenter immutability cases | `AAA_ungated.yaml` |
| VAP CEL | Deny/allow matrix | MAO `pkg/webhooks/vap_test.go` |
| CCCMO transformer | Empty/malformed config | `vsphere_config_transformer_test.go` |
| CCO skip logic | Gate on/off × platform | `controller_test.go` |
| Live cluster | Subset: N-INF-02, N-SEQ-01, N-CFG-06 | openshift-tests / custom CI job on vSphere |

**Do not over-automate**: N-OP-01/02 (operator skew), N-SEQ-05 (wrong drain order), N-ENV-* — QA exploratory unless stable lab pipeline exists.

---

## 7. Exploratory negative charters (QA)

Time-boxed sessions (~90 min) with mission statements:

1. **“Break the shrink”** — Remove as much topology as possible without following runbook; document every error message and any silent corruption.
2. **“Confuse the operators”** — Feature gate flips, pod deletes, ConfigMap hand-edits during active Day 2 change window.
3. **“Credential chaos”** — Valid Infrastructure, invalid secrets; document which component fails first (MAPI vs CCM vs detector).
4. **“Gate profile matrix”** — Same negative patch on DevPreview vs TechPreview vs CustomNoUpgrade; gate absent vs present.

Record: surprises, unclear errors, components not listed in this doc.

---

## 8. Error-handling quality bar

Negative tests fail **QA sign-off** if:

| Criterion | Pass | Fail |
|---|---|---|
| **Rejection clarity** | Admin sees CRD/VAP message explaining what is still in use | Generic `Internal Error` or silent no-op |
| **Data integrity** | No change to spec/CMs on rejected patch | Partial spec or stale CM |
| **Stability** | CO returns healthy; no crash loops | Operator panic/restart storm |
| **Security** | Errors do not leak credential material | Secrets in logs/events |
| **Observability** | Events or alerts point to blocking resource | Must-gather required to guess cause |

---

## 9. Priority summary

| Priority | Count focus | Release blocker? |
|---|---|---|
| **P0** | N-INF-01–10, N-SEQ-01–05, N-OP-01–02, N-CFG-01–04,06, N-CFG-08 | Yes |
| **P1** | Gate rollback, credentials, degraded startup, ControllerConfig | Strong recommend |
| **P2** | Performance, concurrency, env chaos | Best effort |

---

## 10. Reporting template

```markdown
### Negative test: <ID>
- **Build**: 
- **Gate**: ON / OFF
- **Result**: PASS / FAIL
- **Action attempted**: 
- **Observed**: 
- **Expected**: 
- **Severity** (if fail): blocker / major / minor
- **Artifacts**: must-gather link, patch yaml, screenshot
- **QE follow-up**: new fixture? / automation gap?
```

---

## 11. References

- [Risk assessment](./vsphere-multi-vcenter-day2-risk-assessment.md)
- [Gap analysis](./vsphere-multi-vcenter-day2-gap-analysis.md)
- [openshift/api#2784](https://github.com/openshift/api/pull/2784) — xValidation + fixture tests
- [machine-api-operator#1510](https://github.com/openshift/machine-api-operator/pull/1510) — VAP
- [Negative testing in QA (Medium)](https://medium.com/@abdulrehmanrizwan81/the-importance-of-negative-testing-in-qa-and-how-to-approach-it-effectively-77b0366e3465)
- [QA vs QE approach (Testlio)](https://www.testlio.com/blog/qa-vs-qe-approach)
