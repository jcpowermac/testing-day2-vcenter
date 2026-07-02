# N-SEQ-07: Gate-off — Machine VAP is absent

**File:** `test/e2e/vap_test.go`
**Labels:** `readonly`, `admission`, `p0`
**Component:** cluster-config-operator

## Summary

When the feature gate is disabled, confirms that the vSphere Machine failure domain VAP is not installed on the cluster.

## Actions

1. Skip if gate is enabled
2. Attempt to GET the Machine VAP by name
3. Assert the GET fails (resource not found)

## Code

```go
It("should not require vSphere VAP resources", func() {
    if gateEnabled {
        Skip("gate is enabled on this cluster")
    }
    _, err := clients.Kube.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(
        suiteCtx, framework.VAPMachineFailureDomainName, metav1.GetOptions{})
    Expect(err).To(HaveOccurred())
})
```
