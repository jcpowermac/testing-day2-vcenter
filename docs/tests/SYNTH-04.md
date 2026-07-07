# SYNTH-04: orphan tags detected metric increments

**File:** `test/e2e/csi_orphan_tag_test.go`
**Labels:** `mutating`, `csi-operator`, `csi-orphan`, `p1`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Scrapes `vsphere_csi_orphan_tags_detected_total` before and after attaching
and detaching the synthetic orphan tag, asserting it increased — the same
metric assertion as `OBS-01`, but driven by a synthetic orphan instead of FD
removal.

## Actions

1. Scrape the metric before
2. Attach the synthetic orphan tag, wait for detach
3. Scrape the metric after
4. Assert the value increased

## Code

```go
It("SYNTH-04: orphan tags detected metric increments", Label("p1"), func() {
    beforeMetrics, err := csiOperatorMetrics()
    if err != nil {
        Skip(fmt.Sprintf("cannot scrape operator metrics: %v", err))
    }
    beforeVal, _ := framework.ParseMetricValue(beforeMetrics, framework.OrphanTagsDetectedMetric, nil)

    Expect(vsphere.AttachTagToDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)).To(Succeed())
    DeferCleanup(func() {
        _ = vsphere.DetachTagFromDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)
    })

    waitForTagDetached(suiteCtx, orphanSess, orphanDatastore, orphanTagName, framework.OperatorSyncTimeout)

    afterMetrics, err := csiOperatorMetrics()
    Expect(err).NotTo(HaveOccurred())
    afterVal, err := framework.ParseMetricValue(afterMetrics, framework.OrphanTagsDetectedMetric, nil)
    Expect(err).NotTo(HaveOccurred())
    Expect(afterVal).To(BeNumerically(">", beforeVal),
        "orphan tags detected metric should increase after synthetic orphan tag detach")
})
```
