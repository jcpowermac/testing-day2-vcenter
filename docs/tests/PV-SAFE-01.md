# PV-SAFE-01: Orphan tag blocked when PVs exist

**File:** `test/e2e/csi_operator_lifecycle_test.go`
**Labels:** `mutating`, `csi-operator`, `pv-safety`, `p0`
**Component:** vmware-vsphere-csi-driver-operator
**PR:** `csi-op#348`

## Summary

Creates a PVC+pod targeted to the second FD's zone, removes the FD, and verifies the operator sets OrphanCleanupPending=True but does NOT detach the tag (PV safety via `datastoreHasCnsVolumes()`). PVC remains Bound throughout.

## Actions

1. Create PVC+pod in second FD zone
2. Remove FD
3. Wait for OrphanCleanupPending=True
4. Assert tag still attached
5. Assert PVC still Bound
