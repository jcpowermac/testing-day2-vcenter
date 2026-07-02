# N-GATE-02: IsFeatureGateEnabled matches BeforeSuite cached result

**File:** `test/e2e/featuregate_test.go`
**Labels:** `readonly`, `p0`, `operator`
**Component:** openshift/api

## Summary

Cross-checks that `framework.IsFeatureGateEnabled` returns the same enabled/disabled state that was cached during `BeforeSuite`. Guards against stale or inconsistent gate detection.

## Actions

1. Call `framework.IsFeatureGateEnabled` for the `VSphereMultiVCenterDay2` gate
2. Assert the result matches the `gateEnabled` suite variable

## Code

```go
It("should report gate enabled state consistently", func() {
    enabled, err := framework.IsFeatureGateEnabled(suiteCtx, clients.Config, framework.VSphereMultiVCenterDay2Gate)
    Expect(err).NotTo(HaveOccurred())
    Expect(enabled).To(Equal(gateEnabled))
})
```
