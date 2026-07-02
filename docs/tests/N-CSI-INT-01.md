# N-CSI-01 (Integration): CSI credential secret has keys for every vCenter

**File:** `test/e2e/csi_integration_test.go`
**Labels:** `readonly`, `integration`, `p1`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Verifies the CSI driver credential secret (`openshift-cluster-csi-drivers/vmware-vsphere-cloud-credentials`) has key entries prefixed with each Infrastructure vCenter's server name.

## Actions

1. Read current Infrastructure CR and get the vCenters list
2. GET the CSI credential secret; skip if not found
3. Log all secret keys
4. For each vCenter, assert a key prefix match exists

## Code

```go
It("should have CSI driver credential secret with entries for all vCenters", func() {
    infra := currentInfrastructure()
    vcenters := framework.GetVCenters(infra)

    secret, err := framework.GetSecret(suiteCtx, clients.Kube,
        "openshift-cluster-csi-drivers", "vmware-vsphere-cloud-credentials")
    if err != nil {
        Skip(fmt.Sprintf("CSI credential secret not found: %v", err))
    }

    for _, vc := range vcenters {
        Expect(framework.SecretHasKeyPrefix(secret, vc.Server)).To(BeTrue(),
            "CSI credential secret missing key for vCenter %s", vc.Server)
    }
})
```
