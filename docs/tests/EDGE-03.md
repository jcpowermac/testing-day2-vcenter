# EDGE-03: Topology transition 2 FDs -> 1 FD -> 2 FDs

**File:** `test/e2e/csi_operator_lifecycle_test.go`
**Labels:** `mutating`, `csi-operator`, `p1`
**Component:** vmware-vsphere-csi-driver-operator
**PR:** `csi-op#348`

## Summary

Round-trip topology transition: removes FD (2->1), verifies StorageClass survives, restores (1->2), verifies tag re-attached and SPBM profile re-created on second vCenter.

## Actions

1. Remove FD, wait healthy, check StorageClass
2. Restore
3. Wait for tag re-attached
4. Wait for SPBM profile re-created
