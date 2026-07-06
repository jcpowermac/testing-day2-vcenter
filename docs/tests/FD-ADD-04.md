# FD-ADD-04: CSI driver config includes second vCenter

**File:** `test/e2e/csi_operator_lifecycle_test.go`
**Labels:** `mutating`, `csi-operator`, `fd-lifecycle`, `p0`
**Component:** vmware-vsphere-csi-driver-operator
**PR:** `csi-op#348`

## Summary

Reads the CSI driver config (ConfigMap or Secret) and verifies it references both the primary and second vCenter servers.

## Actions

1. GetCSIDriverConfig
2. Assert both vCenter hosts present
