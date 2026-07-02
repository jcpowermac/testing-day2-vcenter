# N-MACH-03: Machine region/zone labels match an Infrastructure failure domain

**File:** `test/e2e/machine_integration_test.go`
**Labels:** `readonly`, `integration`, `p0`
**Component:** cloud-controller-manager

## Summary

For each Machine with region/zone labels, verifies those labels correspond to a failure domain defined in the Infrastructure CR. Catches label/FD drift.

## Actions

1. Skip on single-vCenter clusters or if no FDs configured
2. List all Machines
3. For each labeled Machine, look up the FD by region/zone
4. Assert a matching FD exists

## Code

```go
It("should map every Machine to a valid Infrastructure failure domain", func() {
    requireMultiVCenter()
    infra := currentInfrastructure()
    fds := framework.GetFailureDomains(infra)

    machines := listMachines()
    for _, m := range machines {
        if m.DeletionTimestamp != nil {
            continue
        }
        region, zone, ok := machineLabeledFailureDomain(m)
        if !ok {
            continue
        }
        fd := vsphere.FindFailureDomainByRegionZone(fds, region, zone)
        Expect(fd).NotTo(BeNil(),
            "Machine %s has labels region=%s zone=%s but no matching Infrastructure failure domain",
            m.Name, region, zone)
    }
})
```
