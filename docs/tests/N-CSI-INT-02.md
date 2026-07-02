# N-CSI-02 (Integration): CSI driver controller pods are Running

**File:** `test/e2e/csi_integration_test.go`
**Labels:** `readonly`, `integration`, `p1`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Lists vSphere CSI driver controller pods and asserts each is in `Running` phase.

## Actions

1. List pods by label `app=vmware-vsphere-csi-driver-controller` in `openshift-cluster-csi-drivers`
2. Skip if no pods found
3. Assert each pod's phase is `Running`

## Code

```go
It("should have CSI driver pods running", func() {
    pods, err := framework.ListPodsByLabel(suiteCtx, clients.Kube,
        "openshift-cluster-csi-drivers", "app=vmware-vsphere-csi-driver-controller")
    if err != nil || len(pods) == 0 {
        Skip("vSphere CSI driver controller pods not found")
    }
    for _, pod := range pods {
        Expect(string(pod.Status.Phase)).To(Equal("Running"),
            "CSI driver pod %s phase is %s, expected Running", pod.Name, pod.Status.Phase)
    }
})
```
