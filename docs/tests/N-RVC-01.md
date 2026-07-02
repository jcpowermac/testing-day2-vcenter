# N-RVC-01: Lab config's second vCenter appears in Infrastructure vcenters list

**File:** `test/e2e/real_vcenter_test.go`
**Labels:** `real-vcenter`, `p0`, `mutating`
**Component:** openshift/api

## Summary

After `make apply-lab`, verifies the second vCenter from the lab config appears in the Infrastructure CR's vcenters list.

## Actions

1. Skip if gate is disabled or lab config not loaded
2. Read current Infrastructure CR
3. Extract vCenter server names
4. Assert the lab config's second vCenter server is in the list

## Code

```go
It("should include configured vCenter in Infrastructure", func() {
    infra := currentInfrastructure()
    servers := vsphere.VCenterServers(framework.GetVCenters(infra))
    Expect(servers).To(ContainElement(labCfg.SecondVCenter.Server))
})
```
