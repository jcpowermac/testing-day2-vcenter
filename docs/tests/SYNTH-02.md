# SYNTH-02: orphan cleanup latency is within OperatorSyncTimeout

**File:** `test/e2e/csi_orphan_tag_test.go`
**Labels:** `mutating`, `csi-operator`, `csi-orphan`, `p1`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Measures the wall-clock time between attaching the synthetic orphan tag and
the operator detaching it, logging the actual latency for observability and
asserting it stays under `OperatorSyncTimeout` (12 minutes) rather than the
30-minute backoff cap.

## Actions

1. Attach the synthetic orphan tag, record start time
2. Wait for the tag to detach
3. Log elapsed time and assert it's less than `OperatorSyncTimeout`

## Code

```go
It("SYNTH-02: orphan cleanup latency is within OperatorSyncTimeout", Label("p1"), func() {
    Expect(vsphere.AttachTagToDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)).To(Succeed())
    DeferCleanup(func() {
        _ = vsphere.DetachTagFromDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)
    })

    start := time.Now()
    waitForTagDetached(suiteCtx, orphanSess, orphanDatastore, orphanTagName, framework.OperatorSyncTimeout)
    elapsed := time.Since(start)
    Expect(elapsed).To(BeNumerically("<", framework.OperatorSyncTimeout))
})
```
