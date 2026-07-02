# N-CFG-04: CCM cloud-conf parses as valid cloud config YAML

**File:** `test/e2e/configmap_content_test.go`
**Labels:** `readonly`, `config`, `p0`
**Component:** cloud-controller-manager

## Summary

Reads the CCM cloud config from `openshift-cloud-controller-manager/cloud-conf` and verifies it parses as valid cloud config YAML.

## Actions

1. Read the `cloud.conf` data key from the CCM ConfigMap
2. Assert the data is non-empty
3. Parse it with `vsphere.ParseCloudConfigYAML` and assert no error

## Code

```go
It("should parse CCM cloud-conf YAML", func() {
    data := ccmCloudConfigYAML()
    Expect(data).NotTo(BeEmpty())
    _, err := vsphere.ParseCloudConfigYAML(data)
    Expect(err).NotTo(HaveOccurred())
})
```
