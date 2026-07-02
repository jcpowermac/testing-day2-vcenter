# N-MACH-02: Every Machine has non-empty region/zone labels

**File:** `test/e2e/machine_integration_test.go`
**Labels:** `readonly`, `integration`, `p0`
**Component:** cloud-controller-manager

## Summary

On multi-vCenter clusters, verifies that every non-deleting Machine has non-empty `machine.openshift.io/region` and `machine.openshift.io/zone` labels. These labels are set by CCM based on vCenter tags.

## Actions

1. Skip on single-vCenter clusters
2. List all Machines
3. For each non-deleting Machine, assert region and zone labels are present and non-empty

## Code

```go
It("should label every Machine with region and zone", func() {
    requireMultiVCenter()
    machines := listMachines()
    for _, m := range machines {
        if m.DeletionTimestamp != nil {
            continue
        }
        region, zone, ok := machineLabeledFailureDomain(m)
        Expect(ok).To(BeTrue(), "Machine %s missing region/zone labels", m.Name)
        Expect(region).NotTo(BeEmpty())
        Expect(zone).NotTo(BeEmpty())
    }
})
```
