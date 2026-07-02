# N-OP-03: MAO pods have fewer than 5 restarts

**File:** `test/e2e/operator_health_test.go`
**Labels:** `readonly`, `operator`, `p1`
**Component:** machine-api-operator

## Summary

Lists MAO pods and asserts each has fewer than 5 container restarts.

## Actions

1. List pods by label `k8s-app=machine-api-operator` in `openshift-machine-api`
2. Skip if no pods found
3. For each pod, assert restart count < 5

## Code

```go
It("should not have MAO pods in a crash loop", Label("p1"), func() {
    pods, err := framework.ListPodsByLabel(suiteCtx, clients.Kube,
        framework.MachineAPINamespace, "k8s-app=machine-api-operator")
    if err != nil || len(pods) == 0 {
        Skip("MAO pods not found")
    }
    for _, pod := range pods {
        restarts := framework.PodRestartCount(&pod)
        Expect(restarts).To(BeNumerically("<", 5))
    }
})
```
