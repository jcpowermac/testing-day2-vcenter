# N-RVC-03: Full lab.Verify workflow passes against live cluster

**File:** `test/e2e/real_vcenter_test.go`
**Labels:** `real-vcenter`, `p0`, `mutating`
**Component:** multiple

## Summary

Runs the complete `lab.Verify` workflow against the live cluster, which checks Infrastructure CR, cloud configs, credential secrets, and operator health in one call.

## Actions

1. Call `lab.Verify(suiteCtx, clients, labCfg)`
2. Assert it succeeds

## Code

```go
It("should pass lab verification helper", func() {
    Expect(lab.Verify(suiteCtx, clients, labCfg)).To(Succeed())
})
```
