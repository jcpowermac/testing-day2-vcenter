# N-CSI-04: Provision and bind PVC in existing failure domain

**File:** `test/e2e/csi_storage_test.go`
**Labels:** `real-vcenter`, `mutating`, `storage`, `p1`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Creates a PVC and busybox pod in a test namespace, waits for the PVC to bind, then verifies the resulting PV's topology labels match an Infrastructure failure domain.

## Actions

1. Get default StorageClass and lab config
2. Create a test namespace (cleaned up via DeferCleanup)
3. Create a PVC and a busybox pod that mounts it
4. Wait for PVC to bind
5. Read the bound PV's topology labels
6. Assert the PV's region/zone match an Infrastructure FD

## Code

```go
It("should provision and bind a PVC in existing failure domain (N-CSI-04)", func() {
    sc := requireDefaultStorageClass()
    ns := createTestNamespaceWithCleanup(framework.TestNamespacePrefix)

    pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "csi-baseline-pvc",
        framework.TestPVCSize, sc.Name)
    Expect(err).NotTo(HaveOccurred())

    _, err = framework.CreateBusyboxPod(suiteCtx, clients.Kube, ns, "csi-baseline-pod", pvc.Name)
    Expect(err).NotTo(HaveOccurred())

    boundPVC, err := framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.LongTimeout)
    Expect(err).NotTo(HaveOccurred())

    pv, err := framework.GetPV(suiteCtx, clients.Kube, boundPVC.Spec.VolumeName)
    Expect(err).NotTo(HaveOccurred())

    pvRegion, pvZone, ok := framework.PVTopologyLabels(pv, topoKeys)
    if ok {
        infra := currentInfrastructure()
        fds := framework.GetFailureDomains(infra)
        fd := vsphere.FindFailureDomainByRegionZone(fds, pvRegion, pvZone)
        Expect(fd).NotTo(BeNil())
    }
})
```
