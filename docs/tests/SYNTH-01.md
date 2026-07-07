# SYNTH-01: synthetic orphan tag is detected and detached without PVs

**File:** `test/e2e/csi_orphan_tag_test.go`
**Labels:** `mutating`, `csi-operator`, `csi-orphan`, `p0`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Manually attaches the cluster tag (`infra.Status.InfrastructureName`) to a
datastore that is **not** referenced by any Infrastructure failure domain
(a synthetic orphan, typically an ESXi host's local-disk datastore on the
second vCenter). `findOrphanedTags()` treats any tagged, non-FD
`datacenter/datastore` pair as an orphan regardless of how it got tagged, so
the operator detects and detaches it on its next sync — exercising the same
cleanup path as FD removal (`FD-REM-01`) without touching the Infrastructure
spec or its VAP-guarded fields. Because no PVs exist on the datastore,
`OrphanCleanupPending` should stay `False` throughout.

## Actions

1. Attach the cluster tag to the pre-validated non-FD datastore
2. Wait for the tag to be detached by the operator (within `OperatorSyncTimeout`)
3. Assert `OrphanCleanupPending` stayed `False`
4. Assert the storage ClusterOperator is healthy

## Code

```go
It("SYNTH-01: synthetic orphan tag is detected and detached without PVs", Label("p0"), func() {
    Expect(vsphere.AttachTagToDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)).To(Succeed())
    DeferCleanup(func() {
        _ = vsphere.DetachTagFromDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)
    })

    waitForTagDetached(suiteCtx, orphanSess, orphanDatastore, orphanTagName, framework.OperatorSyncTimeout)

    waitForOrphanConditionFalse(framework.DefaultTimeout)
    waitForStorageOperatorHealthy(framework.DefaultTimeout)
})
```
