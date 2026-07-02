# N-SEQ-06: Dry-run probes each FD to find one the API accepts for removal

**File:** `test/e2e/vap_test.go`
**Labels:** `readonly`, `admission`, `p0`
**Component:** cluster-config-operator

## Summary

Iterates through all failure domains and attempts a dry-run removal of each one. Reports which FDs are referenced (denied by VAP) and which are unreferenced (allowed). Skips if all FDs are referenced.

## Actions

1. Skip if gate is disabled or cluster has < 2 vCenters
2. For each failure domain, build a spec without it and submit a dry-run patch
3. If any succeeds, log it as unreferenced and stop
4. If all are denied, skip the test

## Code

```go
It("should allow removing an unreferenced failure domain via dry-run", func() {
    requireMultiVCenter()
    infra := currentInfrastructure()
    fds := framework.GetFailureDomains(infra)

    var candidate *configv1.VSpherePlatformFailureDomainSpec
    for i := range fds {
        spec := specWithoutFailureDomain(infra, fds[i].Region, fds[i].Zone)
        _, err := patchInfrastructureSpec(spec, true)
        if err == nil {
            candidate = &fds[i]
            GinkgoWriter.Printf("FD %q (region=%s zone=%s) is unreferenced\n",
                fds[i].Name, fds[i].Region, fds[i].Zone)
            break
        }
        GinkgoWriter.Printf("FD %q is referenced: %s\n",
            fds[i].Name, framework.InfrastructurePatchError(err))
    }
    if candidate == nil {
        Skip("all failure domains are referenced by Machines, CPMS, or MachineSets")
    }
})
```
