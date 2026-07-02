# N-CFG-06: No stale vCenters in managed cloud config

**File:** `test/e2e/configmap_content_test.go`
**Labels:** `readonly`, `config`, `p0`
**Component:** cluster-config-operator

## Summary

Verifies there are no leftover/stale vCenter entries in the managed cloud config that don't correspond to a vCenter in the Infrastructure CR. Catches the bug fixed in CCMO#469.

## Actions

1. Read current Infrastructure CR
2. Parse the managed cloud config YAML
3. Call `vsphere.AssertNoStaleVCenters` to verify no extra vCenter entries exist

## Code

```go
It("should not contain stale vCenters in managed cloud config (N-CFG-06)", func() {
    infra := currentInfrastructure()
    cfg, err := vsphere.ParseCloudConfigYAML(managedCloudConfigYAML())
    Expect(err).NotTo(HaveOccurred())
    Expect(vsphere.AssertNoStaleVCenters(infra, cfg)).To(Succeed())
})
```
