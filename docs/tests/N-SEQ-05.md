# N-SEQ-05: Dry-run removing Machine-backed FD denied by VAP (precheck)

**File:** `test/e2e/topology_lifecycle_test.go`
**Labels:** `mutating`, `p0`
**Component:** cluster-config-operator

## Summary

Precheck variant: uses the same logic as N-SEQ-01 but in the topology lifecycle context. Finds a failure domain backed by a running Machine and confirms the VAP denies its removal.

## Actions

1. Skip if gate is disabled, cluster has < 2 vCenters, or no Machine-backed FD
2. Find a FD with matching Machine region/zone labels
3. Attempt removal and assert denial

## Code

```go
It("should deny removing a failure domain that still has Machines (N-SEQ-05 precheck)", func() {
    requireGateEnabled()
    requireMultiVCenter()
    infra := currentInfrastructure()
    region, zone, ok := findMachineBackedFailureDomain(infra)
    if !ok {
        Skip("no Machine-backed failure domain found")
    }
    expectFailureDomainRemovalDenied(infra, region, zone)
})
```
