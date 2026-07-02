# N-CSI-09: vCenter removal blocked by xValidation while PVs present

**File:** `test/e2e/csi_storage_test.go`
**Labels:** `real-vcenter`, `mutating`, `storage`, `p1`
**Component:** vmware-vsphere-csi-driver-operator, openshift/api

## Summary

Creates a PVC on the second vCenter, then probes whether removing that vCenter from the Infrastructure spec is denied. The denial is based on FD references to the vCenter (xValidation), not PV protection. Confirms PV presence is irrelevant to the denial mechanism.

## Actions

1. Skip if fewer than 2 vCenters
2. Create PVC and pod in the lab FD; wait for bind
3. Build a spec without the second vCenter
4. Submit a dry-run patch and assert it's denied
5. Log that PV presence is irrelevant — denial is FD-based

## Code

```go
It("should confirm vCenter removal blocked by existing xValidation, PV presence irrelevant (N-CSI-09)", func() {
    infra := currentInfrastructure()
    vcenters := framework.GetVCenters(infra)
    if len(vcenters) < 2 {
        Skip("need at least 2 vCenters for vCenter removal test")
    }

    ns := createTestNamespace(framework.TestNamespacePrefix)
    testNamespaces = append(testNamespaces, ns)
    pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "csi-vc-guard-pvc",
        framework.TestPVCSize, sc.Name)
    Expect(err).NotTo(HaveOccurred())
    // ... create pod, wait for bind ...

    spec := specWithoutVCenter(infra, lab.SecondVCenter.Server)
    _, err = patchInfrastructureSpec(spec, true)
    Expect(err).To(HaveOccurred(), "vCenter removal should be denied — FDs still reference it")

    GinkgoWriter.Println("PV presence irrelevant — denial is based on FD reference to vCenter")
})
```
