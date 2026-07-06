# FD-ADD-01: Operator tags new FD's datastore

**File:** `test/e2e/csi_operator_lifecycle_test.go`
**Labels:** `mutating`, `csi-operator`, `fd-lifecycle`, `p0`
**Component:** vmware-vsphere-csi-driver-operator
**PR:** `csi-op#348`

## Summary

Verifies the CSI operator's tag category (`openshift-<infraID>`) and tag (`<infraID>`) exist on the second vCenter, and the new FD's datastore is tagged. Also checks that the default StorageClass has a `storagePolicyName` parameter.

## Actions

1. Connect to second vCenter
2. Find tag category by name
3. Check datastore tagged
4. Verify StorageClass has storagePolicyName
