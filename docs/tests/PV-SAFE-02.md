# PV-SAFE-02: Orphan cleanup proceeds after PVs deleted

**File:** `test/e2e/csi_operator_lifecycle_test.go`
**Labels:** `mutating`, `csi-operator`, `pv-safety`, `p0`
**Component:** vmware-vsphere-csi-driver-operator
**PR:** `csi-op#348`

## Summary

Creates a PVC in the second FD, removes the FD (blocked by PV safety), then deletes the PVC/PV and waits for the operator to complete orphan cleanup -- tag detached and condition resolved.

## Actions

1. Create PVC, remove FD
2. Wait for condition True
3. Delete pod+PVC+PV
4. Wait for tag detached
5. Wait for condition False
