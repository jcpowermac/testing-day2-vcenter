# SYNTH-10: operator handles repeated orphans without getting stuck

**File:** `test/e2e/csi_orphan_tag_test.go`
**Labels:** `mutating`, `csi-operator`, `csi-orphan`, `p1`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Attaches and detaches the synthetic orphan tag twice in a row, asserting the
second detach also completes within `OperatorSyncTimeout`. This is not
equivalent to `EDGE-02` (which tests backoff reset after an Infrastructure
FD restore) — it verifies the operator doesn't accumulate backoff or get
stuck handling the same datastore going in and out of orphan state
repeatedly.

## Actions

1. Attach the synthetic orphan tag, wait for detach (first cycle)
2. Attach it again, record start time
3. Wait for detach (second cycle)
4. Assert the second detach also completed within `OperatorSyncTimeout`

## Code

```go
It("SYNTH-10: operator handles repeated orphans without getting stuck", Label("p1"), func() {
    Expect(vsphere.AttachTagToDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)).To(Succeed())
    DeferCleanup(func() {
        _ = vsphere.DetachTagFromDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)
    })
    waitForTagDetached(suiteCtx, orphanSess, orphanDatastore, orphanTagName, framework.OperatorSyncTimeout)

    Expect(vsphere.AttachTagToDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)).To(Succeed())

    start := time.Now()
    waitForTagDetached(suiteCtx, orphanSess, orphanDatastore, orphanTagName, framework.OperatorSyncTimeout)
    elapsed := time.Since(start)
    Expect(elapsed).To(BeNumerically("<", framework.OperatorSyncTimeout),
        "repeated orphan should be cleaned up within OperatorSyncTimeout, not stuck at backoff cap")
})
```
