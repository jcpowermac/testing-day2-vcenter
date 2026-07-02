# N-SEQ-02: Removing FD matching CPMS denied by VAP

**File:** `test/e2e/vap_test.go`
**Labels:** `mutating`, `admission`, `p0`
**Component:** cluster-config-operator

## Summary

Verifies that the CPMS VAP denies removing a failure domain from the Infrastructure spec when a ControlPlaneMachineSet references that FD by name.

## Actions

1. Skip if gate is disabled, cluster has < 2 vCenters, or no CPMS-backed FD exists
2. Find a failure domain referenced by a CPMS's vSphere failure domain name list
3. Build a spec without that FD and submit a real patch
4. Assert the patch is denied

## Code

```go
It("should deny removing a failure domain referenced by a CPMS (N-SEQ-02)", Label("mutating"), func() {
    requireMultiVCenter()
    infra := currentInfrastructure()
    region, zone, ok := findCPMSBackedFailureDomain(infra)
    if !ok {
        Skip("no CPMS-backed failure domain found")
    }
    expectFailureDomainRemovalDenied(infra, region, zone)
})
```
