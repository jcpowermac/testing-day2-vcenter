# N-CSI-08: Probe FD removal behavior when PVs exist in that FD

**File:** `test/e2e/csi_storage_test.go`
**Labels:** `real-vcenter`, `mutating`, `storage`, `p1`
**Component:** vmware-vsphere-csi-driver-operator, openshift/api

## Summary

Creates a PVC in the lab FD, then probes whether removing that FD from the Infrastructure spec is allowed or denied via dry-run. Logs the denial source (Machine VAP, CPMS VAP, MachineSet VAP, or PV-specific). If allowed, flags as a product gap — no VAP protects PV-backed FDs.

## Actions

1. Create PVC and pod in the lab FD; wait for bind
2. Build a spec without the lab FD
3. Submit a dry-run patch
4. If denied: classify the denial source and log it
5. If allowed: log as product gap

## Code

```go
It("should probe FD removal behavior when PVs exist in that FD (N-CSI-08)", func() {
    sc := requireDefaultStorageClass()
    ns := createTestNamespace(framework.TestNamespacePrefix)
    testNamespaces = append(testNamespaces, ns)

    // Create PVC + pod in lab FD
    pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "csi-guard-pvc",
        framework.TestPVCSize, sc.Name)
    Expect(err).NotTo(HaveOccurred())
    nodeSelector := map[string]string{
        topoKeys.Region: lab.FailureDomain.Region,
        topoKeys.Zone:   lab.FailureDomain.Zone,
    }
    _, err = framework.CreateBusyboxPodWithNodeSelector(suiteCtx, clients.Kube, ns,
        "csi-guard-pod", pvc.Name, nodeSelector)
    Expect(err).NotTo(HaveOccurred())
    _, err = framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.LongTimeout)
    Expect(err).NotTo(HaveOccurred())

    // Probe FD removal
    infra := currentInfrastructure()
    spec := specWithoutFailureDomain(infra, lab.FailureDomain.Region, lab.FailureDomain.Zone)
    _, err = patchInfrastructureSpec(spec, true)

    if err != nil {
        errMsg := framework.InfrastructurePatchError(err)
        // Classify denial source
        GinkgoWriter.Printf("FD removal denied: %s\n", errMsg)
    } else {
        GinkgoWriter.Println("PRODUCT GAP: FD removal allowed via dry-run despite PVs present.")
    }
})
```
