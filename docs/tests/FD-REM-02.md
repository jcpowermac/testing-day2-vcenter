# FD-REM-02: StorageClass and SPBM profile survive FD removal

**File:** `test/e2e/csi_operator_lifecycle_test.go`
**Labels:** `mutating`, `csi-operator`, `fd-lifecycle`, `p0`
**Component:** vmware-vsphere-csi-driver-operator
**PR:** `csi-op#348`

## Summary

After FD removal, verifies the default StorageClass still has storagePolicyName, SPBM profile survives on primary vCenter, and a quick PVC smoke test proves storage still works.

## Actions

1. Remove FD
2. Wait for operator healthy
3. Check StorageClass
4. Check SPBM on primary
5. Create PVC + pod, verify Running/Bound
