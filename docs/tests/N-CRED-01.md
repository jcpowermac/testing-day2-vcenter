# N-CRED-01: All 4 consumer secrets have key entries for every Infrastructure vCenter

**File:** `test/e2e/credentials_test.go`
**Labels:** `readonly`, `integration`, `p0`
**Component:** cloud-credential-operator

## Summary

Checks that all four credential consumer secrets contain key entries prefixed with each Infrastructure vCenter's server name. Warns but continues if a secret is missing.

## Actions

1. Read current Infrastructure CR and get the vCenters list
2. For each consumer secret (kube-system, machine-api, CCM, CSI):
   - GET the secret; warn and continue if not found
   - For each vCenter, assert `SecretHasKeyPrefix(secret, vc.Server)` is true

## Code

```go
It("should have credential secrets for all Infrastructure vCenters", func() {
    infra := currentInfrastructure()
    vcenters := framework.GetVCenters(infra)
    Expect(vcenters).NotTo(BeEmpty())

    for _, consumer := range consumers {
        secret, err := framework.GetSecret(suiteCtx, clients.Kube, consumer.namespace, consumer.name)
        if err != nil {
            GinkgoWriter.Printf("warning: secret %s/%s not found: %v\n",
                consumer.namespace, consumer.name, err)
            continue
        }
        for _, vc := range vcenters {
            hasKey := framework.SecretHasKeyPrefix(secret, vc.Server)
            Expect(hasKey).To(BeTrue(),
                "secret %s/%s should have key prefixed with %q",
                consumer.namespace, consumer.name, vc.Server)
        }
    }
})
```
