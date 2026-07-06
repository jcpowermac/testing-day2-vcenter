# OBS-02: TagOperationsTotal tracks detach operations

**File:** `test/e2e/csi_operator_lifecycle_test.go`
**Labels:** `mutating`, `csi-operator`, `observability`, `p1`
**Component:** vmware-vsphere-csi-driver-operator
**PR:** `csi-op#348`

## Summary

Scrapes `vsphere_csi_tag_operations_total{operation="detach",result="success"}` before and after FD removal, asserts it increased.

## Actions

1. Scrape with labels before
2. Remove FD, wait healthy
3. Scrape after
4. Assert increase
