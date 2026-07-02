# N-CSI-02 (Storage): CSI driver pods healthy with current Infrastructure topology

**File:** `test/e2e/csi_storage_test.go`
**Labels:** `real-vcenter`, `mutating`, `storage`, `p1`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

After a topology change (real second vCenter added), verifies all CSI driver controller and node pods are Running with fewer than 5 restarts.

## Actions

1. List CSI controller pods by label; skip if not found
2. Assert each controller pod is Running with < 5 restarts
3. List CSI node pods by label
4. Assert each node pod is Running

## Code

```go
It("should have CSI driver pods healthy with current Infrastructure topology (N-CSI-02)", func() {
    controllerPods, err := framework.ListPodsByLabel(suiteCtx, clients.Kube,
        framework.CSIDriverNamespace, framework.CSIDriverControllerLabel)
    for i := range controllerPods {
        pod := &controllerPods[i]
        Expect(string(pod.Status.Phase)).To(Equal("Running"))
        Expect(framework.PodRestartCount(pod)).To(BeNumerically("<", 5))
    }

    nodePods, err := framework.ListPodsByLabel(suiteCtx, clients.Kube,
        framework.CSIDriverNamespace, framework.CSIDriverNodeLabel)
    if err == nil && len(nodePods) > 0 {
        for _, pod := range nodePods {
            Expect(string(pod.Status.Phase)).To(Equal("Running"))
        }
    }
})
```
