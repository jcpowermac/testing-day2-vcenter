# OBS-03: TagOperationsTotal tracks PV-blocked skips

**File:** `test/e2e/csi_operator_lifecycle_test.go`
**Labels:** `mutating`, `csi-operator`, `observability`, `p1`
**Component:** vmware-vsphere-csi-driver-operator
**PR:** `csi-op#348`

## Summary

Creates a PVC in the second FD, removes the FD, then scrapes `vsphere_csi_tag_operations_total{operation="skip",result="pv_blocked"}` and `vsphere_csi_orphan_tags_detected_total`. Asserts both increased after OrphanCleanupPending goes True.

## Actions

1. Create PVC in second FD
2. Scrape before
3. Remove FD, wait for condition True
4. Scrape after
5. Assert both metrics increased
