# N-CPMS-01: Every CPMS FD name exists in Infrastructure CR

**File:** `test/e2e/cpms_integration_test.go`
**Labels:** `readonly`, `integration`, `p0`
**Component:** machine-api-operator

## Summary

Verifies that every failure domain name referenced by a ControlPlaneMachineSet exists as a named failure domain in the Infrastructure CR. Catches CPMS/Infrastructure drift.

## Actions

1. Skip if no CPMS or FDs found
2. Build a set of Infrastructure FD names
3. For each CPMS, extract its vSphere FD name references
4. Assert each referenced name exists in the Infrastructure FD set

## Code

```go
It("should reference failure domain names that exist in Infrastructure", func() {
    cpmsList := listCPMS()
    infra := currentInfrastructure()
    fds := framework.GetFailureDomains(infra)

    fdNames := map[string]bool{}
    for _, fd := range fds {
        fdNames[fd.Name] = true
    }

    for _, cpms := range cpmsList {
        names := framework.CPMSVSphereFailureDomainNames(&cpms)
        for _, name := range names {
            Expect(fdNames).To(HaveKey(name),
                "CPMS %s references failure domain %q which does not exist in Infrastructure",
                cpms.Name, name)
        }
    }
})
```
