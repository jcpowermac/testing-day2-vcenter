# N-CSI-01 (Storage): ClusterCSIDriver Available and not Degraded

**File:** `test/e2e/csi_storage_test.go`
**Labels:** `readonly`, `storage`, `p0`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Reads the `ClusterCSIDriver` CR and asserts at least one `*Available` condition is True and no `*Degraded` conditions are True.

## Actions

1. GET `ClusterCSIDriver` by name; skip if CRD not available
2. Scan conditions: find any `*Available=True`, collect any `*Degraded=True`
3. Assert at least one Available, no Degraded

## Code

```go
It("should have ClusterCSIDriver Available and not Degraded (N-CSI-01)", func() {
    csi, err := clients.Operator.OperatorV1().ClusterCSIDrivers().Get(
        suiteCtx, framework.ClusterCSIDriverName, metav1.GetOptions{})
    if err != nil {
        if strings.Contains(err.Error(), "not found") {
            Skip("ClusterCSIDriver CRD not available on this cluster")
        }
        Expect(err).NotTo(HaveOccurred())
    }

    var anyAvailable bool
    var degradedConditions []string
    for _, cond := range csi.Status.Conditions {
        if strings.HasSuffix(cond.Type, "Available") && cond.Status == "True" {
            anyAvailable = true
        }
        if strings.HasSuffix(cond.Type, "Degraded") && cond.Status == "True" {
            degradedConditions = append(degradedConditions, cond.Type)
        }
    }
    Expect(anyAvailable).To(BeTrue())
    Expect(degradedConditions).To(BeEmpty())
})
```
