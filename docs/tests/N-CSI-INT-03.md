# N-CSI-03 (Integration): Multi-vCenter cloud config includes all vCenters for CSI

**File:** `test/e2e/csi_integration_test.go`
**Labels:** `readonly`, `integration`, `p1`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

On multi-vCenter clusters, parses the managed cloud config and verifies all Infrastructure vCenters appear. Ensures the CSI driver has a complete vCenter view.

## Actions

1. Skip on single-vCenter clusters or if managed cloud config unavailable
2. Parse the managed cloud config YAML
3. Cross-check against Infrastructure vCenters using `AssertInfrastructureVCentersPresent`

## Code

```go
It("should have managed cloud config listing all vCenter datacenters for CSI", func() {
    infra := currentInfrastructure()
    vcenters := framework.GetVCenters(infra)
    if len(vcenters) < 2 {
        Skip("single vCenter cluster — CSI multi-vCenter parity not applicable")
    }

    raw := managedCloudConfigYAML()
    cfg, err := vsphere.ParseCloudConfigYAML(raw)
    Expect(err).NotTo(HaveOccurred())
    Expect(vsphere.AssertInfrastructureVCentersPresent(infra, cfg)).To(Succeed())
})
```
