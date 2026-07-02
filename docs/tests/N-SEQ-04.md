# N-SEQ-04: Removing vCenter while FD remains (Expected failure)

**File:** `test/e2e/topology_lifecycle_test.go`
**Labels:** `mutating`, `p0`
**Component:** openshift/api

## Summary

Probes whether removing a vCenter while its failure domains remain in the spec is rejected. **Expected to fail** — there is no xValidation rule enforcing that `failureDomain.server` must reference an existing vCenter (SPLAT-2827). Same logic as N-INF-12 but in the mutating lifecycle context.

## Actions

1. Skip if gate is disabled or cluster has < 2 vCenters
2. Build a spec that removes the last vCenter while its FDs remain
3. Submit dry-run patch; if it succeeds, explicitly `Fail()`
4. If rejected, assert error contains relevant strings

## Code

```go
It("should deny removing a vCenter referenced by a failure domain (N-SEQ-04)", func() {
    requireGateEnabled()
    requireMultiVCenter()
    infra := currentInfrastructure()
    spec := fdReferencingRemovedVCenterSpec(infra)
    _, err := patchInfrastructureSpec(spec, true)
    if err == nil {
        Fail("CRD allows removing a vCenter still referenced by a failure domain — " +
            "no xValidation rule enforces FD.server must reference an existing vCenter entry (see SPLAT-2827)")
    }
    Expect(framework.InfrastructurePatchError(err)).To(SatisfyAny(
        ContainSubstring("failure domain"),
        ContainSubstring("vCenter"),
        ContainSubstring("ValidatingAdmissionPolicy"),
    ))
})
```
