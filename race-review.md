# Race Review Notes

Reviewed against `HEAD` commit `44de28a` (`Fix MachineSet cleanup race: wait for Running, not Provisioned`).

## Summary

The last commit fixed the most obvious readiness bug: `WaitForMachineSetMachines()` no longer treats `Provisioned` as good enough. That closes the window where cleanup could begin before the kubelet registered the node.

I found two more likely race/stale-state issues in the same area:

1. `WaitForMachineSetMachines()` still counts deleting `Running` Machines as ready.
2. The CSI suite setup and readiness checks can succeed against stale nodes from a previous run instead of the newly requested replica.

## Finding 1: deleting Machines still count as ready

`pkg/framework/machine.go`:

```go
// WaitForMachineSetMachines waits until the MachineSet has at least `count`
// Machines in Running phase (node linked).
func WaitForMachineSetMachines(ctx context.Context, client machineclient.Interface, msName string, count int) error {
	machines, err := client.MachineV1beta1().Machines(MachineAPINamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "machine.openshift.io/cluster-api-machineset=" + msName,
	})
	if err != nil {
		return fmt.Errorf("list machines for machineset %s: %w", msName, err)
	}
	ready := 0
	for _, m := range machines.Items {
		if m.Status.Phase != nil && *m.Status.Phase == "Running" {
			ready++
		}
	}
	if ready < count {
		return fmt.Errorf("machineset %s has %d/%d running machines", msName, ready, count)
	}
	return nil
}
```

### Why this is still racy

If a prior test run scaled a MachineSet down, a Machine can remain visible briefly with:

- `Status.Phase == "Running"`
- a non-nil `DeletionTimestamp`

That object is no longer a valid "ready" target for a new scale-up, but the helper still counts it. A caller can therefore observe `1/1 running machines` before the new replica exists.

### Impact

This is most likely to affect reuse paths where a pre-existing MachineSet is scaled from `0 -> 1`, especially if the previous teardown has drained the Machine objects only recently.

### Recommended fix

Skip Machines with `DeletionTimestamp != nil` in `WaitForMachineSetMachines()`.

## Finding 2: CSI setup can pass on stale nodes

### Stale-node skip logic

`test/e2e/csi_storage_test.go`:

```go
nodes, err := clients.Kube.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
Expect(err).NotTo(HaveOccurred())
for _, node := range nodes.Items {
	if node.Labels[topoKeys.Region] == lab.FailureDomain.Region &&
		node.Labels[topoKeys.Zone] == lab.FailureDomain.Zone {
		GinkgoWriter.Printf("found existing node %s in lab FD, skipping MachineSet creation\n", node.Name)
		return
	}
}
```

The same pattern exists in `test/e2e/csi_operator_lifecycle_test.go`.

### Why this is still racy

This treats any node with the target topology labels as sufficient setup state, but it does not verify that the node is:

- `Ready`
- not in the middle of deletion
- actually backed by the expected MachineSet
- stable enough to survive the upcoming test body

Because teardown now stops after Machine drain + MachineSet delete, there can still be a short lag before the corresponding node object disappears. During that lag, a later run can skip setup based on stale state.

### Follow-on weak readiness check

After setup, both CSI suites do:

```go
Eventually(func() error {
	return framework.WaitForMachineSetMachines(ctx, clients.Machine, msName, 1)
}, framework.LongTimeout, framework.DefaultPolling).Should(Succeed())

Eventually(func() bool {
	nodeList, err := clients.Kube.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return false
	}
	for _, node := range nodeList.Items {
		if node.Labels[topoKeys.Region] == lab.FailureDomain.Region &&
			node.Labels[topoKeys.Zone] == lab.FailureDomain.Zone {
			return true
		}
	}
	return false
}, framework.LongTimeout, framework.DefaultPolling).Should(BeTrue())
```

That combination can still pass against leftover cluster state:

- the Machine wait can be satisfied by a deleting `Running` Machine
- the node wait can be satisfied by any node in the target FD, including a stale leftover node

Neither condition proves that the newly requested MachineSet replica is the one that became healthy.

### Recommended fix

Harden both CSI suites so they wait for a node that is all of the following:

1. `Ready`
2. topology-labeled for the target FD
3. linked to a non-deleting `Running` Machine belonging to the expected MachineSet

At minimum:

- ignore nodes that are not `Ready`
- ignore deleting Machines in `WaitForMachineSetMachines()`
- prefer a helper that correlates MachineSet -> Machine -> NodeRef -> Node

## Suggested implementation order

1. Update `WaitForMachineSetMachines()` to ignore deleting Machines.
2. Add a helper that waits for a specific MachineSet-backed node to be `Ready` and topology-labeled.
3. Replace the ad hoc "any node in this FD exists" checks in:
   - `test/e2e/csi_storage_test.go`
   - `test/e2e/csi_operator_lifecycle_test.go`
4. Consider hardening the "skip MachineSet creation if node already exists" logic with the same helper or equivalent ownership checks.

## Review comments (2026-07-09)

Both findings verified against HEAD (`44de28a`).

### Finding 1: Confirmed, fix now

The `DeletionTimestamp` gap at `pkg/framework/machine.go:116` is real and the fix is one line (`if m.DeletionTimestamp != nil { continue }`). This also transitively improves Finding 2's readiness wait since both CSI suites call `WaitForMachineSetMachines` after setup.

### Finding 2: Confirmed, but lower priority

The stale-node window is narrow in practice — the node controller garbage-collects orphaned node objects quickly after the Machine is gone. The worst case for the "skip MachineSet creation" path is that the suite reuses a draining node and tests fail with connection errors, which is a noisy (not silent) failure.

**Recommendation:** Add `Ready` condition checking to the node lookups in both CSI suites. Skip the full MachineSet->Machine->NodeRef correlation — that's a lot of machinery for a setup shortcut whose failure mode is already noisy.

### Implementation order

1. Fix `WaitForMachineSetMachines()` to skip deleting Machines (Finding 1).
2. Add `Ready` + non-deleting checks to the CSI node lookups (Finding 2, lighter version).
3. Defer the full ownership-correlation helper unless stale-node flakes persist after steps 1-2.

## Confidence

Moderate.

I do not see another bug as direct as the now-fixed `Provisioned` check, but these two follow-on issues are plausible sources of flaky, stale-state-driven false positives in the same lifecycle path.
