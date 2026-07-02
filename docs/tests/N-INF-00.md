# N-INF-00: Dry-run adding a second vCenter is accepted

**File:** `test/e2e/infrastructure_validation_test.go`
**Labels:** `readonly`, `validation`, `p0`
**Component:** openshift/api

## Summary

Confirms that when the feature gate is enabled, the CRD allows adding a second vCenter to the Infrastructure spec via server-side dry-run. This is the positive-path counterpart to the rejection tests.

## Actions

1. Skip if gate is disabled
2. Read current Infrastructure CR
3. Build a spec with a cloned second vCenter entry (`vcenter2.example.com`)
4. Submit a dry-run patch and assert it succeeds

## Code

```go
It("should allow adding a second vCenter via dry-run", func() {
    infra := currentInfrastructure()
    spec := addSecondVCenterSpec(infra)
    expectPatchAllowedDryRun(spec)
})
```

### Helper: `addSecondVCenterSpec`

```go
func addSecondVCenterSpec(infra *configv1.Infrastructure) *configv1.InfrastructureSpec {
    spec := vsphere.CloneInfrastructureSpec(infra.Spec)
    extra := vsphere.CloneVCenter(spec.PlatformSpec.VSphere.VCenters[0])
    extra.Server = "vcenter2.example.com"
    extra.Datacenters = []string{"DC2"}
    spec.PlatformSpec.VSphere.VCenters = append(spec.PlatformSpec.VSphere.VCenters, extra)
    return &spec
}
```
