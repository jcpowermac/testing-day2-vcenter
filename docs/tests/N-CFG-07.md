# N-CFG-07: insecure-flag only set globally, not duplicated per-vCenter

**File:** `test/e2e/configmap_content_test.go`
**Labels:** `readonly`, `config`, `p0`
**Component:** cluster-config-operator

## Summary

Validates that the `insecure-flag` setting appears only in the global section of the cloud config, not duplicated into each per-vCenter section.

## Actions

1. Parse the managed cloud config YAML
2. Call `vsphere.GlobalInsecureOnly` to check flag placement
3. Assert it returns true (global-only)

## Code

```go
It("should keep insecure-flag out of per-vCenter entries when possible", func() {
    cfg, err := vsphere.ParseCloudConfigYAML(managedCloudConfigYAML())
    Expect(err).NotTo(HaveOccurred())
    Expect(vsphere.GlobalInsecureOnly(cfg)).To(BeTrue())
})
```
