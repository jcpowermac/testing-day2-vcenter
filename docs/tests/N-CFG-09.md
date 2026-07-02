# N-CFG-09: Managed cloud config semantically matches Infrastructure CR and source ConfigMap

**File:** `test/e2e/configmap_content_test.go`
**Labels:** `readonly`, `config`, `p0`
**Component:** cluster-config-operator

## Summary

Three-way parity check: reads the source cloud config from `openshift-config/cloud-provider-config`, the managed config, and the Infrastructure CR, then verifies they agree semantically on vCenter entries.

## Actions

1. Read source cloud config from `openshift-config/cloud-provider-config`; skip if not present
2. Read managed cloud config; assert non-empty
3. Parse managed config and cross-check vCenters against Infrastructure CR

## Code

```go
It("should include source openshift-config cloud config when present (three-way parity)", func() {
    source, ok := sourceCloudConfigYAML()
    if !ok {
        Skip("openshift-config/cloud-provider-config not present")
    }
    Expect(source).NotTo(BeEmpty())

    managed := managedCloudConfigYAML()
    Expect(managed).NotTo(BeEmpty())
    managedCfg, err := vsphere.ParseCloudConfigYAML(managed)
    Expect(err).NotTo(HaveOccurred())
    Expect(vsphere.AssertInfrastructureVCentersPresent(currentInfrastructure(), managedCfg)).To(Succeed())
})
```
