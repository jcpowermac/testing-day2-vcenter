# N-RVC-07: CCM cloud config includes second vCenter server

**File:** `test/e2e/real_vcenter_test.go`
**Labels:** `real-vcenter`, `p0`, `mutating`
**Component:** cloud-controller-manager

## Summary

After Day 2 add, verifies the CCM's cloud config (`openshift-cloud-controller-manager/cloud-conf`) includes the second vCenter server.

## Actions

1. Read CCM cloud config; skip if not available
2. Parse the YAML and extract vCenter server names
3. Assert the second vCenter server from lab config is present

## Code

```go
It("should have CCM cloud config reflecting second vCenter", func() {
    raw := ccmCloudConfigYAML()
    if raw == "" {
        Skip("CCM cloud config not available")
    }
    cfg, err := vsphere.ParseCloudConfigYAML(raw)
    Expect(err).NotTo(HaveOccurred())
    servers := vsphere.VCenterServersFromConfig(cfg)
    Expect(servers).To(ContainElement(labCfg.SecondVCenter.Server))
})
```
