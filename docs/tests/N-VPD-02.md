# N-VPD-02: VPD pods have fewer than 5 restarts

**File:** `test/e2e/problem_detector_test.go`
**Labels:** `readonly`, `operator`, `p1`
**Component:** vsphere-problem-detector

## Summary

Lists vsphere-problem-detector pods and asserts each has fewer than 5 container restarts.

## Actions

1. List pods by label `name=vsphere-problem-detector-operator` in `openshift-cluster-storage-operator`
2. Skip if no pods found
3. For each pod, assert restart count < 5

## Code

```go
It("should not have vsphere-problem-detector pods in a crash loop", func() {
    pods, err := framework.ListPodsByLabel(suiteCtx, clients.Kube,
        vpdNamespace, "name=vsphere-problem-detector-operator")
    if err != nil || len(pods) == 0 {
        Skip("vsphere-problem-detector pods not found")
    }
    for _, pod := range pods {
        restarts := framework.PodRestartCount(&pod)
        Expect(restarts).To(BeNumerically("<", 5))
    }
})
```
