# Guard Multi-vCenter Tests and Fix VAP Dry-Run

## Context

On a brand new single-vCenter cluster (before `make apply-lab`), `test-readonly` reports 4 failures from tests that construct specs requiring 2+ vCenters. A further ~10 admission/integration tests pass vacuously — they test multi-vCenter validation rules against a single-vCenter spec where the rules don't apply. This gives misleading results: failures that aren't real bugs, and passes that didn't validate anything.

Separately, the VAP tests use `vapDryRunWorks` to probe whether dry-run triggers VAP denials. If not, they fall through to a real (non-dry-run) patch inside a `readonly`-labeled test. This is both fragile and incorrect: `readonly` should mean zero write attempts.

## Change 1: Add `requireMultiVCenter()` guard

### File: `test/e2e/helpers_test.go`

Add helper:
```go
func requireMultiVCenter() {
    infra := currentInfrastructure()
    if len(framework.GetVCenters(infra)) < 2 {
        Skip("cluster has fewer than 2 vCenters — run make apply-lab first")
    }
}
```

### Tests to guard

**`infrastructure_validation_test.go`:**
- N-INF-05 — replace inline `< 2` skip with `requireMultiVCenter()` for consistency
- N-INF-06/07 — replace inline `< 2` skip with `requireMultiVCenter()` for consistency
- N-INF-12 — add `requireMultiVCenter()`. With 1 vCenter, `fdReferencingRemovedVCenterSpec` removes the sole vCenter, which is a different scenario than intended.

**`vap_test.go`:**
- N-SEQ-01, N-SEQ-02, N-SEQ-03, N-SEQ-06 — add `requireMultiVCenter()`

**`topology_lifecycle_test.go`:**
- N-SEQ-05, N-SEQ-04, N-TOPO-01 — add `requireMultiVCenter()`

**`machine_integration_test.go`:**
- N-MACH-02, N-MACH-03, N-MACH-04 — add `requireMultiVCenter()`

**No guard needed** (valid on single-vCenter):
- N-INF-00, N-INF-01/02, N-INF-03/04, N-INF-08, N-INF-09/10, N-INF-11
- All configmap, operator, credentials, CSI integration, problem detector, featuregate tests
- `real_vcenter_test.go` — already guarded by `requireLabConfig()`
- `csi_storage_test.go` — already guarded by `requireLabConfigWithFD()`

## Change 2: Fix VAP tests — drop dry-run, use real patches, relabel

VAP denial tests should use real patches. A denied patch doesn't mutate. If the VAP is broken and the patch goes through, that's a product bug worth discovering.

**But**: real patches violate the `readonly` label contract. N-SEQ-01 and N-SEQ-02 must be relabeled as `mutating`.

### Dry-run decision rule

| What's being tested | Mechanism | Use dry-run? |
|---|---|---|
| xValidation CEL rules (N-INF-*) | CRD webhook | Yes — CEL evaluates identically under dry-run |
| VAP denial (N-SEQ-01/02/03/05, N-TOPO-01) | ValidatingAdmissionPolicy | No — use real patch, label `mutating` |
| VAP "allowed" probe (N-SEQ-06) | ValidatingAdmissionPolicy | Yes — we don't want to actually remove the FD |
| Informational probe (N-CSI-08) | Mixed | Yes — logs outcome, doesn't assert |

### File: `test/e2e/helpers_test.go`

**Replace `expectFailureDomainRemovalDenied`:**
```go
func expectFailureDomainRemovalDenied(infra *configv1.Infrastructure, region, zone string) {
    spec := specWithoutFailureDomain(infra, region, zone)
    _, err := patchInfrastructureSpec(spec, false)
    Expect(err).To(HaveOccurred(),
        "removing FD region=%s zone=%s should be denied by VAP", region, zone)
    Expect(framework.InfrastructurePatchError(err)).To(SatisfyAny(
        ContainSubstring("failure domain"),
        ContainSubstring("still in use"),
    ))
}
```

**Delete unused code:**
- `vapDryRunWorks` variable
- `vapDryRunWorks = probeVAPDryRun()` call in `BeforeSuite`
- `probeVAPDryRun()` function
- `isVAPDenial()` function
- `findMachineSetBackedFailureDomain()` function
- `marshalPatchFromSpec()` function
- `removeVCentersFieldSpec()` function

**Keep in `BeforeSuite`:** a log line showing vCenter count for cluster characterization:
```go
GinkgoWriter.Printf("VSphereMultiVCenterDay2 enabled=%v vCenters=%d\n", gateEnabled, len(framework.GetVCenters(infra)))
```

### File: `test/e2e/vap_test.go`

- **Relabel N-SEQ-01 and N-SEQ-02** — add `Label("mutating")` since `expectFailureDomainRemovalDenied` now sends real patches
- **Remove** the "VAP dry-run probe" Describe block (lines 123-134)
- **N-SEQ-06** — keep dry-run (probing "allowed", don't want actual removal)

### File: `test/e2e/topology_lifecycle_test.go`

- N-SEQ-04 — keep dry-run (tests xValidation CEL, not VAP)
- N-SEQ-05 — already in `mutating` context, uses `expectFailureDomainRemovalDenied` which now sends real patch. No label change needed.

### File: `test/e2e/csi_storage_test.go`

- N-CSI-08 — keep dry-run (informational probe)

## Change 3: Update docs

### `docs/tests.md`
- Add header note: multi-vCenter tests skip on single-vCenter clusters; run `make apply-lab` first for full coverage
- Remove N-SEQ-PROBE from catalog (deleted test)
- Update N-SEQ-01, N-SEQ-02 labels to include `mutating`

### `CLAUDE.md`
- Update `test-readonly` description: "safe, no cluster mutation" remains true — VAP denial tests moved to `mutating`
- Remove VAP dry-run known issue (no longer relevant)

## Verification

1. `make build && make vet` — compiles, no unused code
2. `make test-dry-run` — confirm spec count and label assignments
3. Single-vCenter cluster (before apply-lab):
   - `make test-readonly` — multi-vCenter tests skip, no failures from missing vCenters
   - `make test-mutating` — VAP denial tests skip (need 2+ vCenters)
4. After `make apply-lab`:
   - `make test-readonly` — multi-vCenter readonly tests now run
   - `make test-mutating` — VAP denial tests use real patches, denied correctly
5. Confirm the original 4 failures are now clean skips on single-vCenter
