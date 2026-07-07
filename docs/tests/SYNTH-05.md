# SYNTH-05: tag operations detach/success metric increments

**File:** `test/e2e/csi_orphan_tag_test.go`
**Labels:** `mutating`, `csi-operator`, `csi-orphan`, `p1`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Scrapes `vsphere_csi_tag_operations_total{operation="detach", result="success"}`
before and after the synthetic orphan cleanup, asserting it increased — the
synthetic-orphan counterpart to `OBS-02`.

## Actions

1. Scrape the metric before
2. Attach the synthetic orphan tag, wait for detach
3. Scrape the metric after
4. Assert the value increased

## Code

```go
It("SYNTH-05: tag operations detach/success metric increments", Label("p1"), func() {
    detachLabels := map[string]string{"operation": "detach", "result": "success"}

    beforeMetrics, err := csiOperatorMetrics()
    if err != nil {
        Skip(fmt.Sprintf("cannot scrape operator metrics: %v", err))
    }
    beforeVal, _ := framework.ParseMetricValue(beforeMetrics, framework.TagOperationsMetric, detachLabels)

    Expect(vsphere.AttachTagToDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)).To(Succeed())
    DeferCleanup(func() {
        _ = vsphere.DetachTagFromDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)
    })

    waitForTagDetached(suiteCtx, orphanSess, orphanDatastore, orphanTagName, framework.OperatorSyncTimeout)

    afterMetrics, err := csiOperatorMetrics()
    Expect(err).NotTo(HaveOccurred())
    afterVal, err := framework.ParseMetricValue(afterMetrics, framework.TagOperationsMetric, detachLabels)
    Expect(err).NotTo(HaveOccurred())
    Expect(afterVal).To(BeNumerically(">", beforeVal),
        "tag operations detach/success metric should increase")
})
```
