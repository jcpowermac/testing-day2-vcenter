# FD-REM-01: Orphan tag detached after FD removal

**File:** `test/e2e/csi_operator_lifecycle_test.go`
**Labels:** `mutating`, `csi-operator`, `fd-lifecycle`, `p0`
**Component:** vmware-vsphere-csi-driver-operator
**PR:** `csi-op#348`

## Summary

Removes the second FD from Infrastructure (with restore), waits for the operator to detach the orphan tag from the datastore, verifies primary FD tags untouched and OrphanCleanupPending resolves. After restore, verifies tag is re-attached.

## Actions

1. Pre-check tag attached
2. Remove FD (skip if VAP denied)
3. Wait for tag detached
4. Check primary tags
5. Check condition resolves
6. After restore, verify re-attached
