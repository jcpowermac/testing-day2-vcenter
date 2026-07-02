# N-OP-02: CCM pods have fewer than 5 restarts

**File:** `test/e2e/operator_health_test.go`
**Labels:** `readonly`, `operator`, `p1`
**Component:** cloud-controller-manager

## Summary

Lists CCM pods and asserts each has fewer than 5 container restarts, catching crash loops.

## Actions

1. List pods by label `k8s-app=vsphere-cloud-controller-manager` in CCM namespace
2. Skip if no pods found
3. For each pod, assert restart count < 5

## Code

```go
It("should not have CCM pods in a crash loop", Label("p1"), func() {
    pods, err := framework.ListPodsByLabel(suiteCtx, clients.Kube,
        framework.CCMConfigNamespace, "k8s-app=vsphere-cloud-controller-manager")
    if err != nil || len(pods) == 0 {
        Skip("CCM pods not found")
    }
    for _, pod := range pods {
        restarts := framework.PodRestartCount(&pod)
        Expect(restarts).To(BeNumerically("<", 5),
            "CCM pod %s has %d restarts, possible crash loop", pod.Name, restarts)
    }
})
```
