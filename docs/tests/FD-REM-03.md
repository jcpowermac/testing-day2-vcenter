# FD-REM-03: Operator reconciles within backoff window

**File:** `test/e2e/csi_operator_lifecycle_test.go`
**Labels:** `mutating`, `csi-operator`, `fd-lifecycle`, `p1`
**Component:** vmware-vsphere-csi-driver-operator
**PR:** `csi-op#348`

## Summary

Times how long the operator takes to detach the orphan tag after FD removal. Asserts under 12 minutes (successCheckInterval 10min + jitter), validating the backoff is not stuck at the 30-minute cap.

## Actions

1. Remove FD
2. Time tag detach
3. Log elapsed, assert < 12 min
