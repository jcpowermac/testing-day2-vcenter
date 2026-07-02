# N-CSI-03 (Storage): CSI credential secret reflecting all vCenters

**File:** `test/e2e/csi_storage_test.go`
**Labels:** `real-vcenter`, `readonly`, `storage`, `p1`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

On multi-vCenter clusters with gate enabled, verifies the CSI credential secret has key entries for every Infrastructure vCenter.

## Actions

1. Skip on single-vCenter clusters or gate-disabled
2. GET the CSI credential secret; skip if not found
3. For each vCenter, assert a key prefix match exists

## Code

```go
It("should have CSI credential secret reflecting all vCenters (N-CSI-03)", func() {
    infra := currentInfrastructure()
    vcenters := framework.GetVCenters(infra)
    if len(vcenters) < 2 {
        Skip("single vCenter cluster")
    }
    requireGateEnabled()

    secret, err := framework.GetSecret(suiteCtx, clients.Kube,
        framework.CSIDriverNamespace, framework.CSICredentialSecretName)
    if err != nil {
        Skip(fmt.Sprintf("CSI credential secret not found: %v", err))
    }

    for _, vc := range vcenters {
        Expect(framework.SecretHasKeyPrefix(secret, vc.Server)).To(BeTrue(),
            "CSI credential secret missing key for vCenter %s", vc.Server)
    }
})
```
