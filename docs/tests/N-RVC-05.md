# N-RVC-05: All 4 credential secrets have entries for second vCenter

**File:** `test/e2e/real_vcenter_test.go`
**Labels:** `real-vcenter`, `p0`, `mutating`
**Component:** cloud-credential-operator

## Summary

After Day 2 add, verifies each of the four credential consumer secrets contains key entries for the second vCenter's server.

## Actions

1. For each consumer secret (kube-system, machine-api, CCM, CSI):
   - GET the secret; warn and continue if not found
   - Assert `SecretHasKeyPrefix(secret, labCfg.SecondVCenter.Server)` is true

## Code

```go
It("should have credential secrets updated with second vCenter entries", func() {
    consumers := []struct{ namespace, name string }{
        {framework.VSphereCredsNamespace, framework.VSphereCredsSecret},
        {framework.MachineAPINamespace, "vsphere-cloud-credentials"},
        {framework.CCMConfigNamespace, "vsphere-cloud-credentials"},
        {"openshift-cluster-csi-drivers", "vmware-vsphere-cloud-credentials"},
    }
    for _, c := range consumers {
        secret, err := framework.GetSecret(suiteCtx, clients.Kube, c.namespace, c.name)
        if err != nil {
            GinkgoWriter.Printf("warning: secret %s/%s not found: %v\n",
                c.namespace, c.name, err)
            continue
        }
        Expect(framework.SecretHasKeyPrefix(secret, labCfg.SecondVCenter.Server)).To(BeTrue(),
            "secret %s/%s missing credentials for second vCenter %s",
            c.namespace, c.name, labCfg.SecondVCenter.Server)
    }
})
```
