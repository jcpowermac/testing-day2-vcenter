# FD-ADD-02: SPBM profile exists on second vCenter

**File:** `test/e2e/csi_operator_lifecycle_test.go`
**Labels:** `mutating`, `csi-operator`, `fd-lifecycle`, `p0`
**Component:** vmware-vsphere-csi-driver-operator
**PR:** `csi-op#348`

## Summary

Verifies the SPBM storage profile (`openshift-storage-policy-<infraID>`) exists on both the second and primary vCenters after FD addition.

## Actions

1. Connect to both vCenters
2. Check StorageProfileExists on each
