# VC-REM-03: Credential secrets after vCenter removal [observational]

**File:** `test/e2e/csi_operator_lifecycle_test.go`
**Labels:** `mutating`, `csi-operator`, `vcenter-removal`, `p2`
**Component:** vmware-vsphere-csi-driver-operator
**PR:** `csi-op#348`

## Summary

Observational test -- logs whether vsphere-creds still contains username/password keys for the removed vCenter. Does not assert, since credential cleanup is CCO's responsibility.

## Actions

1. Read vsphere-creds
2. Log presence/absence of second vCenter keys
