# N-INF-09: Gate-off — adding a vCenter triggers immutability rule

**File:** `test/e2e/infrastructure_validation_test.go`
**Labels:** `readonly`, `validation`, `p0`
**Component:** openshift/api

## Summary

When the feature gate is disabled, verifies that the CRD's immutability rule prevents adding a second vCenter to the Infrastructure spec.

## Actions

1. Skip if gate is enabled
2. Read current Infrastructure CR
3. Build a spec with a second vCenter added
4. Assert dry-run rejection with `"vcenters cannot be added or removed once set"`

## Code

```go
It("should reject adding a second vCenter (N-INF-09)", func() {
    requireGateDisabled()
    infra := currentInfrastructure()
    expectPatchRejected(addSecondVCenterSpec(infra), "vcenters cannot be added or removed once set")
})
```
