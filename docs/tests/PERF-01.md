# PERF-01: Provision N machines and record per-machine timing

**File:** `test/e2e/provisioning_perf_test.go`
**Labels:** `perf`, `mutating`, `p1`
**Component:** machine-api-operator

## Summary

Performance benchmark that measures machine provisioning throughput and latency
at scale. Creates a dedicated MachineSet, scales to a configurable target count
(default 64), and records per-machine phase transition timestamps (Created →
Provisioning → Provisioned → Running). Designed for A/B comparison of
machine-api-operator changes — e.g., PR#1515 which reduces vCenter API load by
short-circuiting reconciliation of stable machines.

## Actions

1. Read `PERF_WORKER_COUNT` (default 64), `PERF_RESULTS_DIR` (default `reports`), `PERF_STEADY_STATE_SECONDS` (default 720)
2. Clone the first existing worker MachineSet with replicas=0, named `perf-bench-<hex>`
3. Create the MachineSet, register cleanup
4. Record t0, scale to target replica count
5. Poll machines and record the first wall-clock time each machine reaches Provisioning, Provisioned, and Running
6. Wait for all new nodes to reach Ready
7. Steady-state observation: snapshot `controller_runtime_reconcile_total` and `workqueue_adds_total` from Thanos, snapshot Machine resourceVersions, wait for the observation window, snapshot again, compute deltas
8. Best-effort scrape of MAO metrics via Thanos
9. Print summary table with per-machine timing, p50/p90/p99 latency, throughput, and steady-state reconciliation metrics
10. Write structured `perf-results.json` to results directory (includes `steadyState` section)
11. Cleanup: scale to 0, drain, force-delete on timeout, delete MachineSet

## Code

```go
It("PERF-01: should provision N machines and record per-machine timing",
    NodeTimeout(90*time.Minute),
    func(ctx SpecContext) {
        sets := listMachineSets()
        Expect(sets).NotTo(BeEmpty())

        msName = fmt.Sprintf("perf-bench-%s", perfRandomSuffix())
        ms := framework.CloneMachineSetSameSpec(sets[0], msName)
        _, err := framework.CreateMachineSet(suiteCtx, clients.Machine, ms)
        Expect(err).NotTo(HaveOccurred())

        DeferCleanup(NodeTimeout(30*time.Minute), func(ctx SpecContext) { /* scale 0, drain, delete */ })

        t0 := time.Now()
        Expect(framework.ScaleMachineSet(ctx, clients.Machine, msName, int32(workerCount))).To(Succeed())

        result, err := framework.WatchMachinePhaseTimestamps(ctx, clients.Machine, msName, workerCount, framework.PerfTimeout)
        Expect(err).NotTo(HaveOccurred())

        result.ScaleRequestTime = t0
        result.TotalDuration = result.AllRunningTime.Sub(t0).String()

        Expect(framework.WaitForAllNodesReady(ctx, clients.Kube, framework.PerfTimeout)).To(Succeed())

        printPerfSummary(result)
        // write perf-results.json
    })
```
