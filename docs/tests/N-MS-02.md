# N-MS-02: Region/zone labels on MachineSet templates match existing FDs

**File:** `test/e2e/machineset_integration_test.go`
**Labels:** `readonly`, `integration`, `p0`
**Component:** machine-api-operator

## Summary

For each MachineSet with region/zone template labels, verifies those labels match a failure domain in the Infrastructure CR.

## Actions

1. Skip if no MachineSets or FDs found
2. For each MachineSet, read template-level region/zone labels
3. Look up a matching FD by region/zone
4. Assert a match exists

## Code

```go
It("should have MachineSet template labels matching Infrastructure failure domains", func() {
    sets := listMachineSets()
    infra := currentInfrastructure()
    fds := framework.GetFailureDomains(infra)

    for _, ms := range sets {
        labels := ms.Spec.Template.Labels
        if labels == nil { continue }
        region := labels[framework.MachineRegionLabel]
        zone := labels[framework.MachineZoneLabel]
        if region == "" || zone == "" { continue }

        fd := vsphere.FindFailureDomainByRegionZone(fds, region, zone)
        Expect(fd).NotTo(BeNil(),
            "MachineSet %s template labels region=%s zone=%s don't match any Infrastructure FD",
            ms.Name, region, zone)
    }
})
```
