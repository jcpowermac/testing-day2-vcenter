# N-OP-01: CCM, config-operator, machine-api are Available=True, Degraded=False

**File:** `test/e2e/operator_health_test.go`
**Labels:** `readonly`, `operator`, `p0`
**Component:** multiple

## Summary

Checks the three core ClusterOperators (`cloud-controller-manager`, `config-operator`, `machine-api`) for healthy conditions: Available=True and Degraded=False.

## Actions

1. For each operator name:
   - GET the ClusterOperator's `Available` condition and assert True
   - GET the ClusterOperator's `Degraded` condition and assert False

## Code

```go
operators := []string{
    "cloud-controller-manager",
    "config-operator",
    "machine-api",
}

for _, name := range operators {
    It("should keep ClusterOperator/"+name+" Available and not Degraded", func() {
        available, err := framework.GetClusterOperatorCondition(suiteCtx, clients.Config,
            name, configv1.OperatorAvailable)
        Expect(err).NotTo(HaveOccurred())
        Expect(available.Status).To(Equal(configv1.ConditionTrue))

        degraded, err := framework.GetClusterOperatorCondition(suiteCtx, clients.Config,
            name, configv1.OperatorDegraded)
        Expect(err).NotTo(HaveOccurred())
        Expect(degraded.Status).To(Equal(configv1.ConditionFalse))
    })
}
```
