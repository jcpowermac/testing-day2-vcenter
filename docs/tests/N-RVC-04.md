# N-RVC-04: Lab FD exists in Infrastructure with correct server reference

**File:** `test/e2e/real_vcenter_test.go`
**Labels:** `real-vcenter`, `p0`, `mutating`
**Component:** openshift/api

## Summary

When the lab config has a `failureDomain`, verifies it exists in the Infrastructure CR with the expected region/zone and that its server field points to the second vCenter.

## Actions

1. Skip if lab config has no `failureDomain`
2. Look up the FD by region/zone in the Infrastructure CR
3. Assert the FD exists
4. Assert `fd.Server == labCfg.SecondVCenter.Server`

## Code

```go
It("should include failure domain when configured", func() {
    if labCfg.FailureDomain == nil {
        Skip("failureDomain not set in lab config")
    }
    infra := currentInfrastructure()
    fd := vsphere.FindFailureDomainByRegionZone(
        framework.GetFailureDomains(infra),
        labCfg.FailureDomain.Region,
        labCfg.FailureDomain.Zone,
    )
    Expect(fd).NotTo(BeNil())
    Expect(fd.Server).To(Equal(labCfg.SecondVCenter.Server))
})
```
