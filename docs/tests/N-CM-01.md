# N-CM-01: Managed ConfigMap exists with cloud.conf data key

**File:** `test/e2e/configmap_ownership_test.go`
**Labels:** `readonly`, `config`, `operator`, `p0`
**Component:** cluster-config-operator

## Summary

Verifies the managed cloud config ConfigMap (`openshift-config-managed/kube-cloud-config`) exists and contains the expected `cloud.conf` data key.

## Actions

1. GET the ConfigMap from `openshift-config-managed/kube-cloud-config`
2. Assert it has the `cloud.conf` key in `.data`

## Code

```go
It("should expose kube-cloud-config in openshift-config-managed", func() {
    cm, err := framework.GetConfigMap(suiteCtx, clients.Kube,
        framework.ManagedConfigNamespace, framework.ManagedConfigName)
    Expect(err).NotTo(HaveOccurred())
    Expect(cm.Data).To(HaveKey(framework.CloudConfigDataKey))
})
```
