# N-CRED-02: Per-secret check for each consumer

**File:** `test/e2e/credentials_test.go`
**Labels:** `readonly`, `integration`, `p0`
**Component:** cloud-credential-operator

## Summary

Generates a separate `It` spec per consumer secret (4 total), so each secret's vCenter coverage is reported individually. Skips if the specific secret is missing.

## Actions

1. For each consumer secret (`kube-system/vsphere-creds`, `openshift-machine-api/vsphere-cloud-credentials`, `openshift-cloud-controller-manager/vsphere-cloud-credentials`, `openshift-cluster-csi-drivers/vmware-vsphere-cloud-credentials`):
   - GET the secret; skip if not found
   - Log the secret's data keys
   - For each Infrastructure vCenter, assert a key prefix match

## Code

```go
for _, consumer := range consumers {
    It(fmt.Sprintf("should have %s/%s with entries for every vCenter",
        consumer.namespace, consumer.name), func() {
        infra := currentInfrastructure()
        vcenters := framework.GetVCenters(infra)

        secret, err := framework.GetSecret(suiteCtx, clients.Kube,
            consumer.namespace, consumer.name)
        if err != nil {
            Skip(fmt.Sprintf("secret %s/%s not found: %v",
                consumer.namespace, consumer.name, err))
        }

        keys := framework.SecretDataKeys(secret)
        GinkgoWriter.Printf("secret %s/%s keys: %v\n",
            consumer.namespace, consumer.name, keys)

        for _, vc := range vcenters {
            Expect(framework.SecretHasKeyPrefix(secret, vc.Server)).To(BeTrue(),
                "missing credential key for vCenter %s in %s/%s",
                vc.Server, consumer.namespace, consumer.name)
        }
    })
}
```
