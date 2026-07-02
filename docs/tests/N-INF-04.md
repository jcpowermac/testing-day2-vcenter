# N-INF-04: Null vcenters field rejected

**File:** `test/e2e/infrastructure_validation_test.go`
**Labels:** `readonly`, `validation`, `p0`
**Component:** openshift/api

## Summary

Confirms the CRD rejects setting the `vcenters` field to `null` once it has been set.

## Actions

1. Skip if gate is disabled
2. Send a raw JSON merge patch setting `vcenters: null`
3. Assert dry-run rejection

## Code

```go
It("should reject removing the vcenters field once set (N-INF-04)", func() {
    patch := []byte(`{"spec":{"platformSpec":{"vsphere":{"vcenters":null}}}}`)
    expectRawPatchRejected(patch, "vcenters")
})
```
