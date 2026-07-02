# N-INF-10: Gate-off — emptying vcenters list triggers immutability rule

**File:** `test/e2e/infrastructure_validation_test.go`
**Labels:** `readonly`, `validation`, `p0`
**Component:** openshift/api

## Summary

When the feature gate is disabled, verifies that the CRD's immutability rule prevents removing the only vCenter from the Infrastructure spec.

## Actions

1. Skip if gate is enabled or cluster doesn't have exactly one vCenter
2. Read current Infrastructure CR
3. Build a spec with an empty vcenters list
4. Assert dry-run rejection with `"vcenters cannot be added or removed once set"`

## Code

```go
It("should reject removing the only vCenter (N-INF-10)", func() {
    requireGateDisabled()
    infra := currentInfrastructure()
    if len(framework.GetVCenters(infra)) != 1 {
        Skip("cluster does not have exactly one vCenter")
    }
    expectPatchRejected(emptyVCentersSpec(infra), "vcenters cannot be added or removed once set")
})
```
