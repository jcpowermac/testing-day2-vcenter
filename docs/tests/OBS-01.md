# OBS-01: OrphanTagsDetectedTotal metric incremented on FD removal

**File:** `test/e2e/csi_operator_lifecycle_test.go`
**Labels:** `mutating`, `csi-operator`, `observability`, `p1`
**Component:** vmware-vsphere-csi-driver-operator
**PR:** `csi-op#348`

## Summary

Scrapes `vsphere_csi_orphan_tags_detected_total` before and after FD removal, asserts it increased.

## Actions

1. Scrape metric before
2. Remove FD, wait healthy
3. Scrape metric after
4. Assert increase
