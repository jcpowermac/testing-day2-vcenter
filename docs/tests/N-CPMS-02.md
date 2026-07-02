# N-CPMS-02: Logs Infrastructure FDs not referenced by CPMS (informational)

**File:** `test/e2e/cpms_integration_test.go`
**Labels:** `readonly`, `integration`, `p0`
**Component:** machine-api-operator

## Summary

Informational test that logs which Infrastructure failure domains are not referenced by any CPMS. These may be worker-only FDs. Does not fail — purely diagnostic.

## Actions

1. Skip if no CPMS or FDs found
2. Build a set of CPMS-referenced FD names
3. For each Infrastructure FD, log if it's not referenced by any CPMS

## Code

```go
It("should have CPMS failure domains covering all Infrastructure FDs", func() {
    cpmsList := listCPMS()
    infra := currentInfrastructure()
    fds := framework.GetFailureDomains(infra)

    for _, cpms := range cpmsList {
        cpmsNames := map[string]bool{}
        for _, name := range framework.CPMSVSphereFailureDomainNames(&cpms) {
            cpmsNames[name] = true
        }
        for _, fd := range fds {
            if !cpmsNames[fd.Name] {
                GinkgoWriter.Printf(
                    "note: Infrastructure FD %q not referenced by CPMS %s (may be worker-only)\n",
                    fd.Name, cpms.Name)
            }
        }
    }
})
```
