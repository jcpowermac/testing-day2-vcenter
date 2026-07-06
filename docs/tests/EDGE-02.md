# EDGE-02: Backoff resets after successful sync

**File:** `test/e2e/csi_operator_lifecycle_test.go`
**Labels:** `mutating`, `csi-operator`, `p1`
**Component:** vmware-vsphere-csi-driver-operator
**PR:** `csi-op#348`

## Summary

Removes FD, waits for tag detach, restores, then verifies the tag is re-attached within 12 minutes -- proving the operator's backoff resets after successful cleanup rather than staying at the 30-minute cap.

## Actions

1. Remove FD, wait tag detached
2. Restore
3. Time tag re-attach, log elapsed
