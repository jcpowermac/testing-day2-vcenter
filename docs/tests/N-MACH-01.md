# N-MACH-01: Every non-deleting Machine is Running or Provisioned

**File:** `test/e2e/machine_integration_test.go`
**Labels:** `readonly`, `integration`, `p0`
**Component:** machine-api-operator

## Summary

Lists all Machines and verifies that every Machine not being deleted is in `Running` or `Provisioned` phase. Catches stuck or failed Machines.

## Actions

1. List all Machines in the machine-api namespace
2. Skip Machines with a deletion timestamp
3. Assert each Machine's phase is `Running` or `Provisioned`

## Code

```go
It("should have all worker Machines in a healthy phase", func() {
    machines := listMachines()
    Expect(machines).NotTo(BeEmpty())

    for _, m := range machines {
        if m.DeletionTimestamp != nil {
            continue
        }
        phase := ""
        if m.Status.Phase != nil {
            phase = *m.Status.Phase
        }
        Expect(phase).To(SatisfyAny(
            Equal("Running"),
            Equal("Provisioned"),
        ), "Machine %s has unexpected phase %q", m.Name, phase)
    }
})
```
