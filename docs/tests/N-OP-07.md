# N-OP-07: Deleting managed ConfigMap triggers config-operator recreation

**File:** `test/e2e/configmap_ownership_test.go`
**Labels:** `mutating`, `config`, `operator`, `p1`
**Component:** cluster-config-operator

## Summary

Deletes the managed cloud config ConfigMap and asserts that the config-operator recreates it within the default timeout. Uses snapshot/restore to ensure the ConfigMap is returned to its original state even if the test fails.

## Actions

1. Skip if gate is disabled
2. Snapshot the ConfigMap (for restore in DeferCleanup)
3. Delete the managed ConfigMap
4. Poll with `Eventually` until the ConfigMap reappears
5. On cleanup, restore the ConfigMap from snapshot

## Code

```go
It("should recreate kube-cloud-config if deleted when gate is enabled (N-OP-07)", func() {
    requireGateEnabled()

    snapshot, err := framework.SnapshotConfigMap(suiteCtx, clients.Kube,
        framework.ManagedConfigNamespace, framework.ManagedConfigName)
    Expect(err).NotTo(HaveOccurred())

    DeferCleanup(func() {
        _ = framework.RestoreConfigMapFromSnapshot(suiteCtx, clients.Kube, snapshot)
    })

    err = clients.Kube.CoreV1().ConfigMaps(framework.ManagedConfigNamespace).Delete(
        suiteCtx, framework.ManagedConfigName, metav1.DeleteOptions{})
    Expect(err).NotTo(HaveOccurred())

    Eventually(func() error {
        _, err := framework.GetConfigMap(suiteCtx, clients.Kube,
            framework.ManagedConfigNamespace, framework.ManagedConfigName)
        return err
    }).WithTimeout(framework.DefaultTimeout).WithPolling(framework.DefaultPolling).Should(Succeed())
})
```
