# N-TOPO-02: Add fake vCenter, wait for reconciliation, remove it, confirm no stale config

**File:** `test/e2e/topology_lifecycle_test.go`
**Labels:** `mutating`, `p0`
**Component:** cluster-config-operator

## Summary

End-to-end mutating sequence: adds a temporary fake vCenter to the Infrastructure spec, waits for the managed cloud config to include it, removes it, then waits for the cloud config to have no stale entries. Uses `withInfrastructureRestore` to guarantee the spec is restored on cleanup.

## Actions

1. Skip if lab config is present (use real vCenter workflow instead) or gate is disabled
2. Clone the current spec, add a fake vCenter (`temp-vcenter-e2e.example.com`)
3. Patch the Infrastructure spec (real, not dry-run)
4. Poll until managed cloud config includes all vCenters
5. Remove the fake vCenter and patch again
6. Poll until managed cloud config has no stale entries
7. Restore original Infrastructure spec via DeferCleanup

## Code

```go
It("should add and remove a temporary vCenter without leaving stale cloud config (#469)", func() {
    if labCfg != nil {
        Skip("lab config present; use make apply-lab / test-real / restore-lab")
    }
    requireGateEnabled()
    infra := currentInfrastructure()
    if len(framework.GetVCenters(infra)) >= 3 {
        Skip("cluster already has 3 vCenters")
    }

    withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
        extra := vsphere.CloneVCenter(spec.PlatformSpec.VSphere.VCenters[0])
        extra.Server = "temp-vcenter-e2e.example.com"
        extra.Datacenters = []string{"TEMP-DC"}
        spec.PlatformSpec.VSphere.VCenters = append(spec.PlatformSpec.VSphere.VCenters, extra)

        _, err := patchInfrastructureSpec(spec, false)
        Expect(err).NotTo(HaveOccurred())

        // Wait for cloud config to include new vCenter
        Eventually(func() error {
            cfg, err := vsphere.ParseCloudConfigYAML(managedCloudConfigYAML())
            if err != nil { return err }
            return vsphere.AssertInfrastructureVCentersPresent(currentInfrastructure(), cfg)
        }).WithTimeout(framework.DefaultTimeout).WithPolling(framework.DefaultPolling).Should(Succeed())

        // Remove fake vCenter
        current := currentInfrastructure()
        removeSpec := vsphere.CloneInfrastructureSpec(current.Spec)
        removeSpec.PlatformSpec.VSphere.VCenters = vsphere.RemoveVCenterByServer(
            removeSpec.PlatformSpec.VSphere.VCenters, extra.Server)
        _, err = patchInfrastructureSpec(&removeSpec, false)
        Expect(err).NotTo(HaveOccurred())

        // Wait for no stale entries
        Eventually(func() error {
            cfg, err := vsphere.ParseCloudConfigYAML(managedCloudConfigYAML())
            if err != nil { return err }
            return vsphere.AssertNoStaleVCenters(currentInfrastructure(), cfg)
        }).WithTimeout(framework.DefaultTimeout).WithPolling(framework.DefaultPolling).Should(Succeed())
    })
})
```
