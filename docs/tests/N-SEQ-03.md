# N-SEQ-03: Create MachineSet, wait for Machine, FD removal denied by VAP

**File:** `test/e2e/vap_test.go`
**Labels:** `mutating`, `admission`, `p0`
**Component:** cluster-config-operator

## Summary

Creates a 1-replica MachineSet in a known failure domain, waits for the Machine to reach Running, then confirms the MachineSet VAP denies removing that FD from the Infrastructure spec. Cleans up by scaling to 0 and deleting the MachineSet.

## Actions

1. Skip if gate is disabled, cluster has < 2 vCenters, or no FDs/MachineSets exist
2. Clone an existing MachineSet with region/zone labels for the first FD
3. Create the MachineSet with 1 replica
4. Register `DeferCleanup` to scale down and delete
5. Wait for the MachineSet's Machine to be Running
6. Attempt to remove the FD and assert VAP denial

## Code

```go
It("should deny removing a failure domain referenced by a MachineSet (N-SEQ-03)", Label("mutating"), func() {
    requireMultiVCenter()
    infra := currentInfrastructure()
    fds := framework.GetFailureDomains(infra)
    sets := listMachineSets()

    fd := fds[0]
    msName := "e2e-vap-ms-n-seq-03"
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
