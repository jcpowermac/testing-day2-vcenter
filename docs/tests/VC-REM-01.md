# VC-REM-01: Complete vCenter removal lifecycle

**File:** `test/e2e/csi_operator_lifecycle_test.go`
**Labels:** `mutating`, `csi-operator`, `vcenter-removal`, `p0`
**Component:** vmware-vsphere-csi-driver-operator
**PR:** `csi-op#348`

## Summary

Full lifecycle: remove FDs, wait for orphan cleanup, verify SPBM profile deleted from second vCenter, remove vCenter entry, verify CSI config updated, StorageClass intact, primary SPBM profile untouched.

## Actions

1. Remove FDs
2. Wait for tag detach + condition clear
3. Check SPBM deleted
4. Remove vCenter entry
5. Verify config, StorageClass, primary SPBM
