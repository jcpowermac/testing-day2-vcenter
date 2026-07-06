# VC-REM-02: CSI driver config updated after vCenter removal

**File:** `test/e2e/csi_operator_lifecycle_test.go`
**Labels:** `mutating`, `csi-operator`, `vcenter-removal`, `p0`
**Component:** vmware-vsphere-csi-driver-operator
**PR:** `csi-op#348`

## Summary

After removing FDs then vCenter, reads CSI driver config and asserts removed vCenter is gone while primary vCenter remains.

## Actions

1. Remove FDs, wait healthy
2. Remove vCenter, wait healthy
3. Read CSI config, assert removed absent, primary present
