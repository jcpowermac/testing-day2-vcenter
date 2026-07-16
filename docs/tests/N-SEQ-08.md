# N-SEQ-08: VAP resourceVersion stable across MAO sync cycles (SPLAT-2854)

**File:** `test/e2e/vap_test.go`
**Labels:** `readonly`, `admission`, `p0`, `p1`
**Component:** machine-api-operator

## Summary

Detects the SPLAT-2854 idempotency bug where MAO's `syncVSphereFailureDomainVAPs()` triggers spurious VAP updates every sync cycle (~5s). The VAP constructors omitted `MatchPolicy`, `NamespaceSelector`, and `ObjectSelector`; the API server defaults these on storage, and `resourceapply` sees nil vs. defaulted as a diff — causing UPDATE on every cycle even though nothing changed functionally.

The fix (MAO#1518) explicitly populates these fields so the desired spec matches the stored spec.

## Actions

1. Skip if gate is disabled
2. GET all 3 VAPs and record their `resourceVersion`
3. Sleep 30s (covers ~6 MAO sync cycles at 5s each)
4. Re-GET all 3 VAPs and assert `resourceVersion` has not changed

## Expected Behavior

| Condition | Result |
|---|---|
| Before fix (MAO#1518) | `resourceVersion` changes on every sync cycle — test fails |
| After fix | `resourceVersion` stable after initial creation — test passes |

## Code

```go
It("should not update VAPs on every sync cycle (SPLAT-2854)", Label("p1"), func() {
    vapNames := []string{
        framework.VAPMachineFailureDomainName,
        framework.VAPCPMSFailureDomainName,
        framework.VAPMachineSetFailureDomainName,
    }

    initialVersions := make(map[string]string, len(vapNames))
    for _, name := range vapNames {
        vap, err := clients.Kube.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(suiteCtx, name, metav1.GetOptions{})
        Expect(err).NotTo(HaveOccurred(), "VAP %q should exist", name)
        initialVersions[name] = vap.ResourceVersion
    }

    syncCycles := 6
    syncInterval := 5 * time.Second
    wait := time.Duration(syncCycles) * syncInterval
    time.Sleep(wait)

    for _, name := range vapNames {
        vap, err := clients.Kube.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(suiteCtx, name, metav1.GetOptions{})
        Expect(err).NotTo(HaveOccurred())
        Expect(vap.ResourceVersion).To(Equal(initialVersions[name]),
            fmt.Sprintf("VAP %s resourceVersion changed — spurious update detected (SPLAT-2854)",
                name, initialVersions[name], vap.ResourceVersion))
    }
})
```
