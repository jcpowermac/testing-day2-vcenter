# N-INF-11: Exceeding 3 vCenters rejected by maxItems=3

**File:** `test/e2e/infrastructure_validation_test.go`
**Labels:** `readonly`, `validation`, `p0`
**Component:** openshift/api

## Summary

Validates the CRD's `maxItems=3` constraint on the vcenters array. Builds a spec with 4 vCenters and confirms rejection.

## Actions

1. Skip if gate is disabled
2. Read current Infrastructure CR
3. Build a spec with 4 vCenter entries
4. Assert dry-run rejection with `"must have at most 3 items"`

## Code

```go
It("should reject more than 3 vCenters (N-INF-11)", func() {
    infra := currentInfrastructure()
    expectPatchRejected(tooManyVCentersSpec(infra), "must have at most 3 items")
})
```

### Helper: `tooManyVCentersSpec`

```go
func tooManyVCentersSpec(infra *configv1.Infrastructure) *configv1.InfrastructureSpec {
    spec := vsphere.CloneInfrastructureSpec(infra.Spec)
    base := spec.PlatformSpec.VSphere.VCenters
    for i := 0; len(spec.PlatformSpec.VSphere.VCenters) < 4; i++ {
        clone := vsphere.CloneVCenter(base[0])
        clone.Server = fmt.Sprintf("vcenter-extra-%d.example.com", i)
        spec.PlatformSpec.VSphere.VCenters = append(spec.PlatformSpec.VSphere.VCenters, clone)
    }
    return &spec
}
```
