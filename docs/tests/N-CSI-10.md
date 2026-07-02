# N-CSI-10: Default StorageClass backed by vSphere CSI with WaitForFirstConsumer

**File:** `test/e2e/csi_storage_test.go`
**Labels:** `readonly`, `storage`, `p1`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Verifies the default StorageClass uses the vSphere CSI provisioner and has `WaitForFirstConsumer` volume binding mode.

## Actions

1. Get the default StorageClass; skip if none
2. Assert the provisioner is the vSphere CSI driver name
3. Assert the binding mode is `WaitForFirstConsumer`

## Code

```go
It("should have a default StorageClass backed by vSphere CSI (N-CSI-10)", Label("p1"), func() {
    sc := requireDefaultStorageClass()
    Expect(sc.Provisioner).To(Equal(framework.ClusterCSIDriverName))
    Expect(framework.StorageClassIsWaitForFirstConsumer(sc)).To(BeTrue())
})
```
