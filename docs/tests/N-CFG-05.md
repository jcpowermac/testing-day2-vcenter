# N-CFG-05: Every Infrastructure vCenter has a corresponding managed cloud config entry

**File:** `test/e2e/configmap_content_test.go`
**Labels:** `readonly`, `config`, `p0`
**Component:** cluster-config-operator

## Summary

Cross-references the Infrastructure CR's vCenters list against the managed cloud config to ensure every vCenter is represented. Detects missing entries that would prevent the cloud provider from connecting to a vCenter.

## Actions

1. Read current Infrastructure CR to get the vCenters list
2. Parse the managed cloud config YAML
3. Call `vsphere.AssertInfrastructureVCentersPresent` to verify all vCenters appear

## Code

```go
It("should include all Infrastructure vCenters in managed cloud config", func() {
    infra := currentInfrastructure()
    cfg, err := vsphere.ParseCloudConfigYAML(managedCloudConfigYAML())
    Expect(err).NotTo(HaveOccurred())
    Expect(vsphere.AssertInfrastructureVCentersPresent(infra, cfg)).To(Succeed())
})
```
