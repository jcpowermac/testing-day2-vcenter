# N-SEQ-01: Removing FD matching Machine region/zone denied by VAP

**File:** `test/e2e/vap_test.go`
**Labels:** `mutating`, `admission`, `p0`
**Component:** cluster-config-operator

## Summary

Verifies that the Machine VAP denies removing a failure domain from the Infrastructure spec when a Machine has region/zone labels matching that FD. This is a real (non-dry-run) patch that the VAP should block.

## Actions

1. Skip if gate is disabled, cluster has < 2 vCenters, or no Machine-backed FD exists
2. Find a failure domain whose region/zone matches a running Machine's labels
3. Build a spec without that FD and submit a real patch
4. Assert the patch is denied with error mentioning "failure domain" or "still in use"

## Code

```go
It("should deny removing a failure domain referenced by a Machine (N-SEQ-01)", Label("mutating"), func() {
    requireMultiVCenter()
    infra := currentInfrastructure()
    region, zone, ok := findMachineBackedFailureDomain(infra)
    if !ok {
        Skip("no Machine-backed failure domain found")
    }
    expectFailureDomainRemovalDenied(infra, region, zone)
})
```

### Helper: `expectFailureDomainRemovalDenied`

```go
func expectFailureDomainRemovalDenied(infra *configv1.Infrastructure, region, zone string) {
    spec := specWithoutFailureDomain(infra, region, zone)
    _, err := patchInfrastructureSpec(spec, false)
    Expect(err).To(HaveOccurred(),
        "removing FD region=%s zone=%s should be denied by VAP", region, zone)
    Expect(framework.InfrastructurePatchError(err)).To(SatisfyAny(
        ContainSubstring("failure domain"),
        ContainSubstring("still in use"),
    ))
}
```
