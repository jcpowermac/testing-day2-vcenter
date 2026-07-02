# N-SEQ-00: All 3 VAPs and bindings exist on cluster

**File:** `test/e2e/vap_test.go`
**Labels:** `readonly`, `admission`, `p0`
**Component:** cluster-config-operator

## Summary

Verifies that the three ValidatingAdmissionPolicies (machine, cpms, machineset) and their corresponding bindings are installed on the cluster when the feature gate is enabled.

## Actions

1. Skip if gate is disabled
2. For each VAP name (`VAPMachineFailureDomainName`, `VAPCPMSFailureDomainName`, `VAPMachineSetFailureDomainName`):
   - GET the ValidatingAdmissionPolicy and assert it exists with non-empty validations
   - GET the matching ValidatingAdmissionPolicyBinding (tries `<name>` then `<name>-binding`)

## Code

```go
It("should install vSphere failure domain VAP resources", func() {
    for _, name := range []string{
        framework.VAPMachineFailureDomainName,
        framework.VAPCPMSFailureDomainName,
        framework.VAPMachineSetFailureDomainName,
    } {
        vap, err := clients.Kube.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(suiteCtx, name, metav1.GetOptions{})
        Expect(err).NotTo(HaveOccurred(), "VAP %q should exist", name)
        Expect(vap.Spec.Validations).NotTo(BeEmpty())

        _, err = clients.Kube.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(suiteCtx, name, metav1.GetOptions{})
        if err != nil {
            _, err = clients.Kube.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(suiteCtx, name+"-binding", metav1.GetOptions{})
        }
        Expect(err).NotTo(HaveOccurred(), "VAP binding for %q should exist", name)
    }
})
```
