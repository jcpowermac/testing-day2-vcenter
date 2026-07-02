# N-INF-12: Removing vCenter still referenced by FD (Expected failure)

**File:** `test/e2e/infrastructure_validation_test.go`
**Labels:** `readonly`, `validation`, `p0`
**Component:** openshift/api

## Summary

Probes whether the CRD rejects removing a vCenter that is still referenced by a failure domain's `server` field. **Expected to fail** — there is currently no xValidation rule enforcing that `failureDomain.server` must reference an existing vCenter entry (tracked in SPLAT-2827).

## Actions

1. Skip if gate is disabled or cluster has fewer than 2 vCenters
2. Read current Infrastructure CR
3. Build a spec that removes the last vCenter while its failure domains still reference it
4. Submit dry-run patch; if it succeeds, explicitly `Fail()` to flag the missing rule
5. If rejected, assert the error mentions "failure domain", "vCenter", or "ValidatingAdmissionPolicy"

## Code

```go
It("should reject removing a vCenter still referenced by a failure domain (N-INF-12)", func() {
    requireMultiVCenter()
    infra := currentInfrastructure()
    spec := fdReferencingRemovedVCenterSpec(infra)
    _, err := patchInfrastructureSpec(spec, true)
    if err == nil {
        Fail("CRD allows removing a vCenter that is still referenced by a failure domain — " +
            "no xValidation rule enforces FD.server must reference an existing vCenter entry")
    }
    Expect(framework.InfrastructurePatchError(err)).To(SatisfyAny(
        ContainSubstring("failure domain"),
        ContainSubstring("vCenter"),
        ContainSubstring("ValidatingAdmissionPolicy"),
    ))
})
```
