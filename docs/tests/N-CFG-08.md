# N-CFG-08: Validate GetVCenter after FD removal (skipped)

**File:** `test/e2e/problem_detector_test.go`
**Labels:** `readonly`, `operator`, `p1`
**Component:** vsphere-problem-detector

## Summary

Placeholder test for validating vsphere-problem-detector's `GetVCenter` behavior after a failure domain is removed. Currently skipped pending the merge of vsphere-problem-detector#224.

## Actions

1. Immediately skip with a message about waiting for upstream merge

## Code

```go
It("should validate GetVCenter behavior after failure domain removal when #224 merges", func() {
    Skip("waiting for vsphere-problem-detector#224 merge and lab scenarios (N-CFG-08)")
})
```
