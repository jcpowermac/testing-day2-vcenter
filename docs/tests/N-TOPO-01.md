# N-TOPO-01: Create MachineSet, FD removal denied by VAP, scale down and cleanup

**File:** `test/e2e/topology_lifecycle_test.go`
**Labels:** `mutating`, `p1`
**Component:** cluster-config-operator

## Summary

Full lifecycle test: creates a 1-replica MachineSet in a known FD, waits for the Machine to be Running, confirms the VAP denies removing that FD, then scales down and deletes the MachineSet.

## Actions

1. Skip if gate is disabled, cluster has < 2 vCenters, or no FDs/MachineSets exist
2. Clone an existing MachineSet for the first FD with 1 replica
3. Create MachineSet; register DeferCleanup for scale-down + delete
4. Wait for Machine to reach Running
5. Attempt FD removal and assert VAP denial

## Code

```go
It("should deny removing an FD referenced by a scaled MachineSet", func() {
    requireGateEnabled()
    requireMultiVCenter()
    infra := currentInfrastructure()
    fds := framework.GetFailureDomains(infra)
    sets := listMachineSets()

    fd := fds[0]
    msName := "e2e-vap-probe-ms"
    ms := framework.CloneMachineSetForVAP(sets[0], msName, fd.Region, fd.Zone, 1)

    created, err := framework.CreateMachineSet(suiteCtx, clients.Machine, ms)
    Expect(err).NotTo(HaveOccurred())
    DeferCleanup(func() {
        _ = framework.ScaleMachineSet(suiteCtx, clients.Machine, created.Name, 0)
        Eventually(func() error {
            return framework.WaitForMachineSetDrained(suiteCtx, clients.Machine, created.Name)
        }).WithTimeout(framework.LongTimeout).WithPolling(framework.DefaultPolling).Should(Succeed())
        _ = framework.DeleteMachineSet(suiteCtx, clients.Machine, created.Name)
    })

    Eventually(func() error {
        return framework.WaitForMachineSetMachines(suiteCtx, clients.Machine, msName, 1)
    }).WithTimeout(framework.LongTimeout).WithPolling(framework.DefaultPolling).Should(Succeed())

    expectFailureDomainRemovalDenied(infra, fd.Region, fd.Zone)
})
```
