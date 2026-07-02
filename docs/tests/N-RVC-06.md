# N-RVC-06: CCM, config-operator, machine-api are Available after Day 2 add

**File:** `test/e2e/real_vcenter_test.go`
**Labels:** `real-vcenter`, `p0`, `mutating`
**Component:** multiple

## Summary

After adding a real second vCenter, verifies the three core operators remain Available.

## Actions

1. For each operator (CCM, config-operator, machine-api):
   - GET the Available condition
   - Assert it is True

## Code

```go
It("should keep all operators healthy after Day 2 add", func() {
    for _, co := range []string{"cloud-controller-manager", "config-operator", "machine-api"} {
        available, err := framework.GetClusterOperatorCondition(suiteCtx, clients.Config,
            co, configv1.OperatorAvailable)
        Expect(err).NotTo(HaveOccurred())
        Expect(available.Status).To(Equal(configv1.ConditionTrue),
            "ClusterOperator %s should be Available after Day 2 add", co)
    }
})
```
