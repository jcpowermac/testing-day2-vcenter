# N-INF-05: Swapping existing vCenter server triggers add-and-remove guard

**File:** `test/e2e/infrastructure_validation_test.go`
**Labels:** `readonly`, `validation`, `p0`
**Component:** openshift/api

## Summary

Validates the xValidation rule that prevents swapping an existing vCenter's server value in a single patch. The CRD treats this as simultaneously adding and removing a vCenter.

## Actions

1. Skip if gate is disabled or cluster has fewer than 2 vCenters
2. Read current Infrastructure CR
3. Build a spec that changes the second vCenter's server to `"vcenter-swapped.example.com"`
4. Assert dry-run rejection with `"Cannot add and remove vCenters at the same time"`

## Code

```go
It("should reject swapping an existing vCenter server (N-INF-05)", func() {
    requireMultiVCenter()
    infra := currentInfrastructure()
    expectPatchRejected(swapSecondVCenterServer(infra, "vcenter-swapped.example.com"),
        "Cannot add and remove vCenters at the same time")
})
```
