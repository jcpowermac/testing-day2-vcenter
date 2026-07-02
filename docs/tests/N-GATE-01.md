# N-GATE-01: Gate appears in FeatureGate/cluster status

**File:** `test/e2e/featuregate_test.go`
**Labels:** `readonly`, `p0`, `operator`
**Component:** openshift/api

## Summary

Verifies that the `VSphereMultiVCenterDay2` feature gate is listed in the `FeatureGate/cluster` status resource and has a non-empty version string. Confirms the gate's enabled/disabled state matches the suite-level cached value.

## Actions

1. Call `framework.GetFeatureGateAttributes` to query the FeatureGate CR for the `VSphereMultiVCenterDay2` gate
2. Assert the gate is found in the status
3. Assert the version string is non-empty
4. Assert the enabled state matches the `gateEnabled` value cached in `BeforeSuite`

## Code

```go
It("should expose VSphereMultiVCenterDay2 on FeatureGate/cluster", func() {
    enabled, version, found, err := framework.GetFeatureGateAttributes(suiteCtx, clients.Config, framework.VSphereMultiVCenterDay2Gate)
    Expect(err).NotTo(HaveOccurred())
    Expect(found).To(BeTrue(), "VSphereMultiVCenterDay2 must be listed in FeatureGate/cluster status")
    Expect(version).NotTo(BeEmpty())
    Expect(enabled).To(Equal(gateEnabled))
})
```
