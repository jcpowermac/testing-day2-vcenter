# N-MS-01: MachineSet workspace datacenter maps to known Infrastructure FD

**File:** `test/e2e/machineset_integration_test.go`
**Labels:** `readonly`, `integration`, `p0`
**Component:** machine-api-operator

## Summary

For each MachineSet with a vSphere providerSpec workspace, verifies the workspace datacenter matches the topology datacenter of a known failure domain in the Infrastructure CR.

## Actions

1. Skip if no MachineSets or FDs found
2. Build a map from datacenter to FD name
3. For each MachineSet, extract its workspace datacenter from providerSpec
4. Assert the datacenter matches a known FD

## Code

```go
It("should have providerSpec workspace matching an Infrastructure FD topology", func() {
    sets := listMachineSets()
    infra := currentInfrastructure()
    fds := framework.GetFailureDomains(infra)

    fdByDC := map[string]string{}
    for _, fd := range fds {
        fdByDC[fd.Topology.Datacenter] = fd.Name
    }

    for _, ms := range sets {
        providerSpec, err := framework.ExtractVSphereMachineSetProviderSpec(&ms)
        if err != nil { continue }
        if providerSpec.Workspace == nil { continue }

        dc := providerSpec.Workspace.Datacenter
        _, found := fdByDC[dc]
        Expect(found).To(BeTrue(),
            "MachineSet %s workspace datacenter %q does not match any Infrastructure FD topology",
            ms.Name, dc)
    }
})
```
