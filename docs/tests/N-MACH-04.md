# N-MACH-04: Machine workspace datacenter matches labeled FD topology

**File:** `test/e2e/machine_integration_test.go`
**Labels:** `readonly`, `integration`, `p0`
**Component:** machine-api-operator

## Summary

For each Machine with a vSphere providerSpec workspace, verifies the workspace datacenter matches the topology datacenter of the failure domain identified by the Machine's region/zone labels.

## Actions

1. Skip on single-vCenter clusters or if no FDs configured
2. List all Machines
3. For each non-deleting Machine:
   - Extract vSphere providerSpec
   - Look up the FD matching the Machine's labels
   - Assert `providerSpec.Workspace.Datacenter == fd.Topology.Datacenter`

## Code

```go
It("should have Machine providerSpec workspace matching Infrastructure topology", func() {
    requireMultiVCenter()
    infra := currentInfrastructure()
    fds := framework.GetFailureDomains(infra)

    machines := listMachines()
    for _, m := range machines {
        if m.DeletionTimestamp != nil {
            continue
        }
        providerSpec, err := framework.ExtractVSphereMachineProviderSpec(&m)
        if err != nil { continue }
        if providerSpec.Workspace == nil { continue }

        region, zone, ok := machineLabeledFailureDomain(m)
        if !ok { continue }
        fd := vsphere.FindFailureDomainByRegionZone(fds, region, zone)
        if fd == nil { continue }

        Expect(providerSpec.Workspace.Datacenter).To(Equal(fd.Topology.Datacenter),
            "Machine %s workspace datacenter should match FD %s topology", m.Name, fd.Name)
    }
})
```
