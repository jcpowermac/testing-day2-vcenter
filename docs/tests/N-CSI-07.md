# N-CSI-07: PV deleted when PVC deleted with reclaimPolicy Delete

**File:** `test/e2e/csi_storage_test.go`
**Labels:** `real-vcenter`, `mutating`, `storage`, `p2`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Creates a PVC + pod, waits for bind, then deletes the pod and PVC. Verifies the PV is also deleted (reclaimPolicy=Delete).

## Actions

1. Get default StorageClass; skip if reclaimPolicy is not Delete
2. Create test namespace, PVC, and busybox pod
3. Wait for PVC to bind
4. Delete the pod and PVC
5. Wait for the PV to be deleted within timeout

## Code

```go
It("should delete PV when PVC is deleted with reclaimPolicy Delete (N-CSI-07)", func() {
    sc := requireDefaultStorageClass()
    if sc.ReclaimPolicy != nil && *sc.ReclaimPolicy != corev1.PersistentVolumeReclaimDelete {
        Skip(fmt.Sprintf("default StorageClass reclaimPolicy is %s, not Delete", *sc.ReclaimPolicy))
    }

    ns := createTestNamespaceWithCleanup(framework.TestNamespacePrefix)
    pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "csi-cleanup-pvc",
        framework.TestPVCSize, sc.Name)
    Expect(err).NotTo(HaveOccurred())
    _, err = framework.CreateBusyboxPod(suiteCtx, clients.Kube, ns, "csi-cleanup-pod", pvc.Name)
    Expect(err).NotTo(HaveOccurred())

    boundPVC, err := framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.LongTimeout)
    Expect(err).NotTo(HaveOccurred())
    pvName := boundPVC.Spec.VolumeName

    Expect(framework.DeletePod(suiteCtx, clients.Kube, ns, "csi-cleanup-pod")).To(Succeed())
    Expect(framework.DeletePVC(suiteCtx, clients.Kube, ns, pvc.Name)).To(Succeed())

    err = framework.WaitForPVDeleted(suiteCtx, clients.Kube, pvName, framework.LongTimeout)
    Expect(err).NotTo(HaveOccurred())
})
```
