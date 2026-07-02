# N-CM-02: Managed ConfigMap stable over 60s observation window

**File:** `test/e2e/configmap_ownership_test.go`
**Labels:** `readonly`, `config`, `operator`, `p0`
**Component:** cluster-config-operator

## Summary

Watches the managed cloud config ConfigMap for 60 seconds and asserts it is not modified during that window. Catches dual-writer issues where multiple controllers fight over the same ConfigMap.

## Actions

1. Call `framework.WaitForConfigMapStable` with a 60-second observation window
2. The helper watches the ConfigMap's resourceVersion and fails if it changes

## Code

```go
It("should keep managed ConfigMap stable over observation window (steady-state single writer)", func() {
    err := framework.WaitForConfigMapStable(suiteCtx, clients.Kube,
        framework.ManagedConfigNamespace, framework.ManagedConfigName, 60*time.Second)
    Expect(err).NotTo(HaveOccurred())
})
```
