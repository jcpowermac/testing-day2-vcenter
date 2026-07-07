# SYNTH-09: orphan cleanup causes no side-effect damage

**File:** `test/e2e/csi_orphan_tag_test.go`
**Labels:** `mutating`, `csi-operator`, `csi-orphan`, `p0`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Verifies that cleaning up a synthetic orphan tag has no side effects beyond
the tag detach itself: the storage ClusterOperator stays healthy,
`OrphanCleanupPending` resolves to `False`, and the default StorageClass
retains its `StoragePolicyName` parameter. Because SPBM profile deletion only
triggers when a vCenter's failure domain count drops to zero (not the case
here — the second vCenter keeps its real FD), no SPBM transition is expected.

## Actions

1. Attach the synthetic orphan tag, wait for detach
2. Assert the storage ClusterOperator is healthy
3. Assert `OrphanCleanupPending` is `False`
4. Assert the default StorageClass still has `StoragePolicyName`

## Code

```go
It("SYNTH-09: orphan cleanup causes no side-effect damage", Label("p0"), func() {
    Expect(vsphere.AttachTagToDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)).To(Succeed())
    DeferCleanup(func() {
        _ = vsphere.DetachTagFromDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)
    })

    waitForTagDetached(suiteCtx, orphanSess, orphanDatastore, orphanTagName, framework.OperatorSyncTimeout)

    waitForStorageOperatorHealthy(framework.DefaultTimeout)
    waitForOrphanConditionFalse(framework.DefaultTimeout)

    sc := requireDefaultStorageClass()
    Expect(sc.Parameters).To(HaveKey("StoragePolicyName"),
        "default StorageClass should retain StoragePolicyName after synthetic orphan cleanup")
})
```
