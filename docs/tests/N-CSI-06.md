# N-CSI-06: Provision PVC with explicit topology constraint in correct FD (Expected failure)

**File:** `test/e2e/csi_storage_test.go`
**Labels:** `real-vcenter`, `mutating`, `storage`, `p2`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Creates a custom StorageClass with `allowedTopologies` pinned to the lab FD, provisions a PVC and pod using that class, and verifies the PV lands in the expected topology.

**Expected to fail** due to the storage policy gap on the second vCenter.

## Actions

1. Clone the default StorageClass with topology constraints for the lab FD
2. Register DeferCleanup to delete the custom StorageClass
3. Create a test namespace, PVC (using the custom class), and busybox pod
4. Wait for PVC to bind
5. Assert PV topology labels match the lab FD

## Code

```go
It("should provision PVC with explicit topology constraint in correct FD (N-CSI-06)", func() {
    defaultSC := requireDefaultStorageClass()
    ns := createTestNamespace(framework.TestNamespacePrefix)
    testNamespaces = append(testNamespaces, ns)

    topologyTerms := []corev1.TopologySelectorTerm{
        {
            MatchLabelExpressions: []corev1.TopologySelectorLabelRequirement{
                {Key: topoKeys.Region, Values: []string{lab.FailureDomain.Region}},
                {Key: topoKeys.Zone, Values: []string{lab.FailureDomain.Zone}},
            },
        },
    }
    scName := "csi-test-topo-" + lab.FailureDomain.Name
    _, err := framework.CloneStorageClassWithTopology(suiteCtx, clients.Kube,
        defaultSC, scName, topologyTerms)
    Expect(err).NotTo(HaveOccurred())
    DeferCleanup(func() { _ = framework.DeleteStorageClass(suiteCtx, clients.Kube, scName) })

    pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "csi-topo-pvc",
        framework.TestPVCSize, scName)
    Expect(err).NotTo(HaveOccurred())
    _, err = framework.CreateBusyboxPod(suiteCtx, clients.Kube, ns, "csi-topo-pod", pvc.Name)
    Expect(err).NotTo(HaveOccurred())

    boundPVC, err := framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.LongTimeout)
    Expect(err).NotTo(HaveOccurred())
    pv, err := framework.GetPV(suiteCtx, clients.Kube, boundPVC.Spec.VolumeName)
    Expect(err).NotTo(HaveOccurred())

    pvRegion, pvZone, ok := framework.PVTopologyLabels(pv, topoKeys)
    Expect(ok).To(BeTrue())
    Expect(pvRegion).To(Equal(lab.FailureDomain.Region))
    Expect(pvZone).To(Equal(lab.FailureDomain.Zone))
})
```
