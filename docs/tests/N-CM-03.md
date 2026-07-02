# N-CM-03: CCM ConfigMap exists with cloud.conf data key

**File:** `test/e2e/configmap_ownership_test.go`
**Labels:** `readonly`, `config`, `operator`, `p0`
**Component:** cloud-controller-manager

## Summary

Verifies the CCM cloud config ConfigMap (`openshift-cloud-controller-manager/cloud-conf`) exists and contains the `cloud.conf` data key.

## Actions

1. GET the ConfigMap from `openshift-cloud-controller-manager/cloud-conf`
2. Assert it has the `cloud.conf` key in `.data`

## Code

```go
It("should expose cloud-conf for CCM consumption", func() {
    cm, err := framework.GetConfigMap(suiteCtx, clients.Kube,
        framework.CCMConfigNamespace, framework.CCMConfigName)
    Expect(err).NotTo(HaveOccurred())
    Expect(cm.Data).To(HaveKey(framework.CloudConfigDataKey))
})
```
