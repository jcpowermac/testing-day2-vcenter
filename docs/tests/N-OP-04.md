# N-OP-04: CSI driver controller pods have fewer than 5 restarts

**File:** `test/e2e/operator_health_test.go`
**Labels:** `readonly`, `operator`, `p1`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Lists vSphere CSI driver controller pods and asserts each has fewer than 5 container restarts.

## Actions

1. List pods by label `app=vmware-vsphere-csi-driver-controller` in `openshift-cluster-csi-drivers`
2. Skip if no pods found
3. For each pod, assert restart count < 5

## Code

```go
It("should not have CSI driver pods in a crash loop", Label("p1"), func() {
    pods, err := framework.ListPodsByLabel(suiteCtx, clients.Kube,
        "openshift-cluster-csi-drivers", "app=vmware-vsphere-csi-driver-controller")
    if err != nil || len(pods) == 0 {
        Skip("vSphere CSI driver pods not found")
    }
    for _, pod := range pods {
        restarts := framework.PodRestartCount(&pod)
        Expect(restarts).To(BeNumerically("<", 5))
    }
})
```
