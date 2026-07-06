# PV-SAFE-03: Force cleanup annotation overrides PV safety

**File:** `test/e2e/csi_operator_lifecycle_test.go`
**Labels:** `mutating`, `csi-operator`, `pv-safety`, `p1`
**Component:** vmware-vsphere-csi-driver-operator
**PR:** `csi-op#348`

## Summary

With PVs present, sets `csi.vsphere.vmware.com/force-orphan-cleanup: "true"` on ClusterCSIDriver. Verifies tag is detached despite PVs and PVC remains Bound (only the tag is removed, not the data).

## Actions

1. Create PVC, remove FD
2. Wait for condition True
3. Annotate ClusterCSIDriver
4. Wait for tag detached, condition False
5. Assert PVC still Bound
