# N-RVC-02: Managed cloud config includes all Infrastructure vCenters, no stale entries

**File:** `test/e2e/real_vcenter_test.go`
**Labels:** `real-vcenter`, `p0`, `mutating`
**Component:** cluster-config-operator

## Summary

After Day 2 add, verifies the managed cloud config includes all Infrastructure vCenters and has no stale (leftover) entries.

## Actions

1. Read Infrastructure CR and parse managed cloud config
2. Assert all Infrastructure vCenters are present in config
3. Assert no stale vCenter entries exist

## Code

```go
It("should reflect configured vCenter in managed cloud config", func() {
    infra := currentInfrastructure()
    cfg, err := vsphere.ParseCloudConfigYAML(managedCloudConfigYAML())
    Expect(err).NotTo(HaveOccurred())
    Expect(vsphere.AssertInfrastructureVCentersPresent(infra, cfg)).To(Succeed())
    Expect(vsphere.AssertNoStaleVCenters(infra, cfg)).To(Succeed())
})
```
