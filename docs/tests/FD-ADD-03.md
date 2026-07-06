# FD-ADD-03: Operator conditions healthy

**File:** `test/e2e/csi_operator_lifecycle_test.go`
**Labels:** `mutating`, `csi-operator`, `fd-lifecycle`, `p0`
**Component:** vmware-vsphere-csi-driver-operator
**PR:** `csi-op#348`

## Summary

Asserts storage ClusterOperator is Available/not Degraded, OrphanCleanupPending is not True, and operator pods have no more than 2 restarts.

## Actions

1. Wait for CO healthy
2. Check OrphanCleanupPending absent/False
3. List operator pods, check restarts
