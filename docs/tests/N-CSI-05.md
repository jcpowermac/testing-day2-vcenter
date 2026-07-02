# N-CSI-05: Provision PV in new failure domain with correct topology labels (Expected failure)

**File:** `test/e2e/csi_storage_test.go`
**Labels:** `real-vcenter`, `mutating`, `storage`, `p1`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Creates a PVC and a busybox pod with a node selector targeting the lab's second-vCenter failure domain. Waits for PVC bind and validates the PV's topology labels match the lab FD. Also confirms the PV was provisioned on the second vCenter's server.

**Expected to fail** because the storage policy is not propagated to the second vCenter.

## Actions

1. Get default StorageClass and lab config with FD
2. Create test namespace, PVC, and busybox pod with node selector for lab FD
3. Wait for PVC to bind
4. Assert PV topology labels match the lab FD region/zone
5. Assert the matched FD's server is the second vCenter

## Code

```go
It("should provision a PV in new failure domain with correct topology labels (N-CSI-05)", func() {
    sc := requireDefaultStorageClass()
    ns := createTestNamespace(framework.TestNamespacePrefix)
    testNamespaces = append(testNamespaces, ns)

    pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "csi-newfd-pvc",
        framework.TestPVCSize, sc.Name)
    Expect(err).NotTo(HaveOccurred())

    nodeSelector := map[string]string{
        topoKeys.Region: lab.FailureDomain.Region,
        topoKeys.Zone:   lab.FailureDomain.Zone,
    }
    _, err = framework.CreateBusyboxPodWithNodeSelector(suiteCtx, clients.Kube, ns,
        "csi-newfd-pod", pvc.Name, nodeSelector)
    Expect(err).NotTo(HaveOccurred())

    boundPVC, err := framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.LongTimeout)
    Expect(err).NotTo(HaveOccurred())

    pv, err := framework.GetPV(suiteCtx, clients.Kube, boundPVC.Spec.VolumeName)
    Expect(err).NotTo(HaveOccurred())

    pvRegion, pvZone, ok := framework.PVTopologyLabels(pv, topoKeys)
    Expect(ok).To(BeTrue())
    Expect(pvRegion).To(Equal(lab.FailureDomain.Region))
    Expect(pvZone).To(Equal(lab.FailureDomain.Zone))

    matchedFD := vsphere.FindFailureDomainByRegionZone(
        framework.GetFailureDomains(currentInfrastructure()), pvRegion, pvZone)
    Expect(matchedFD).NotTo(BeNil())
    Expect(matchedFD.Server).To(Equal(lab.SecondVCenter.Server))
})
```
