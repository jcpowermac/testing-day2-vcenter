# N-INF-03: Empty vcenters array rejected by minItems=1

**File:** `test/e2e/infrastructure_validation_test.go`
**Labels:** `readonly`, `validation`, `p0`
**Component:** openshift/api

## Summary

Confirms the CRD's `minItems=1` constraint rejects setting the `vcenters` array to empty.

## Actions

1. Skip if gate is disabled
2. Send a raw JSON merge patch setting `vcenters: []`
3. Assert dry-run rejection with message containing `"at least 1 items"`

## Code

```go
It("should reject reducing vcenters to an empty array (N-INF-03)", func() {
    patch := []byte(`{"spec":{"platformSpec":{"vsphere":{"vcenters":[]}}}}`)
    expectRawPatchRejected(patch, "at least 1 items")
})
```
