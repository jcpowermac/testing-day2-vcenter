# N-INF-08: Ratcheting — identity patch accepted

**File:** `test/e2e/infrastructure_validation_test.go`
**Labels:** `readonly`, `validation`, `p0`
**Component:** openshift/api

## Summary

Confirms the CRD allows patching unrelated Infrastructure fields when the vSphere spec is unchanged (identity/no-op update). This validates CRD ratcheting behavior — existing valid specs should not be rejected on re-apply.

## Actions

1. Skip if gate is disabled
2. Read current Infrastructure CR
3. Clone the spec as-is (no changes)
4. Submit a dry-run patch and assert it succeeds

## Code

```go
It("should allow patching unrelated Infrastructure fields via dry-run (ratcheting)", func() {
    infra := currentInfrastructure()
    spec := vsphere.CloneInfrastructureSpec(infra.Spec)
    if spec.PlatformSpec.VSphere == nil {
        Skip("no vsphere platform spec")
    }
    expectPatchAllowedDryRun(&spec)
})
```
