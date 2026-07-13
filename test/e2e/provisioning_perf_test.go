package e2e

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Provisioning performance benchmark", Ordered, Label("perf", "mutating", "p1"), func() {
	var (
		msName      string
		newMLName   string
		workerCount int
		resultsDir  string
		result      *framework.PerfBenchmarkResult
		source      string
	)

	BeforeAll(func() {
		wcStr := os.Getenv("PERF_WORKER_COUNT")
		if wcStr == "" {
			workerCount = 64
		} else {
			var err error
			workerCount, err = strconv.Atoi(wcStr)
			Expect(err).NotTo(HaveOccurred(), "PERF_WORKER_COUNT must be a valid integer")
			Expect(workerCount).To(BeNumerically(">", 0), "PERF_WORKER_COUNT must be > 0")
		}

		resultsDir = os.Getenv("PERF_RESULTS_DIR")
		if resultsDir == "" {
			resultsDir = "reports"
		}
	})

	AfterAll(NodeTimeout(30*time.Minute), func(ctx SpecContext) {
		for _, name := range []string{newMLName, msName} {
			if name == "" {
				continue
			}
			cleanupMachineSet(ctx, name)
		}
	})

	It("PERF-01: should provision N machines and record per-machine timing",
		NodeTimeout(90*time.Minute),
		func(ctx SpecContext) {
			By("verifying cluster is ready before benchmark")
			for _, co := range []string{"cloud-controller-manager", "config-operator", "machine-api"} {
				GinkgoWriter.Printf("waiting for clusteroperator %s to be stable\n", co)
				Expect(framework.WaitForClusterOperatorStable(ctx, clients.Config, co, framework.DefaultTimeout)).To(Succeed(),
					"clusteroperator %s must be stable before benchmark", co)
			}
			GinkgoWriter.Printf("waiting for all existing machines to be healthy\n")
			Expect(framework.WaitForAllMachinesHealthy(ctx, clients.Machine, framework.DefaultTimeout)).To(Succeed(),
				"all existing machines must be Running before benchmark")
			GinkgoWriter.Printf("waiting for all existing nodes to be ready\n")
			Expect(framework.WaitForAllNodesReady(ctx, clients.Kube, framework.DefaultTimeout)).To(Succeed(),
				"all existing nodes must be Ready before benchmark")
			GinkgoWriter.Printf("cluster ready, starting benchmark\n")

			sets := listMachineSets()
			Expect(sets).NotTo(BeEmpty(), "cluster must have at least one MachineSet")

			msName = fmt.Sprintf("perf-bench-%s", perfRandomSuffix())
			source = sets[0].Name
			ms := framework.CloneMachineSetSameSpec(sets[0], msName)
			replicas := int32(workerCount)
			ms.Spec.Replicas = &replicas

			t0 := time.Now()
			GinkgoWriter.Printf("creating benchmark MachineSet %s (cloned from %s) with replicas=%d at %s\n",
				msName, source, workerCount, t0.Format(time.RFC3339))
			_, err := framework.CreateMachineSet(suiteCtx, clients.Machine, ms)
			Expect(err).NotTo(HaveOccurred(), "create benchmark MachineSet")

			var watchErr error
			result, watchErr = framework.WatchMachinePhaseTimestamps(ctx, clients.Machine, msName, workerCount, framework.PerfTimeout)
			Expect(result).NotTo(BeNil(), "result must be returned even on partial completion")

			runningCount := 0
			for _, m := range result.Machines {
				if !m.Running.IsZero() {
					runningCount++
				}
			}

			if watchErr != nil {
				GinkgoWriter.Printf("WARNING: not all machines reached Running: %v\n", watchErr)
				GinkgoWriter.Printf("  %d/%d machines Running — proceeding with partial results\n", runningCount, workerCount)
			}
			Expect(runningCount).To(BeNumerically(">", 0), "at least one machine must reach Running")

			result.ScaleRequestTime = t0
			if !result.AllRunningTime.IsZero() {
				result.TotalDuration = result.AllRunningTime.Sub(t0).String()
			} else {
				result.TotalDuration = time.Since(t0).String()
			}

			GinkgoWriter.Printf("%d/%d machines Running (total: %s)\n",
				runningCount, workerCount, result.TotalDuration)

			GinkgoWriter.Printf("waiting for nodes to be Ready (best-effort)\n")
			if err := framework.WaitForAllNodesReady(ctx, clients.Kube, framework.PerfTimeout); err != nil {
				GinkgoWriter.Printf("WARNING: not all nodes became Ready: %v — continuing\n", err)
			}

			steadyStateSeconds := 720
			if ssStr := os.Getenv("PERF_STEADY_STATE_SECONDS"); ssStr != "" {
				var parseErr error
				steadyStateSeconds, parseErr = strconv.Atoi(ssStr)
				Expect(parseErr).NotTo(HaveOccurred(), "PERF_STEADY_STATE_SECONDS must be a valid integer")
				Expect(steadyStateSeconds).To(BeNumerically(">", 0))
			}
			steadyStateDuration := time.Duration(steadyStateSeconds) * time.Second

			By("observing steady-state reconciliation")
			beforeReconciles := queryMetricOrZero(ctx,
				`controller_runtime_reconcile_total{controller="machine-controller"}`,
				"controller_runtime_reconcile_total", nil)
			beforeQueueAdds := queryMetricOrZero(ctx,
				`workqueue_adds_total{name="machine-controller"}`,
				"workqueue_adds_total", nil)

			rvBefore, err := framework.SnapshotMachineResourceVersions(ctx, clients.Machine, msName)
			Expect(err).NotTo(HaveOccurred(), "snapshot machine resourceVersions before steady-state")

			GinkgoWriter.Printf("steady-state observation: waiting %s with %d/%d Running machines\n",
				steadyStateDuration, runningCount, workerCount)
			select {
			case <-time.After(steadyStateDuration):
			case <-ctx.Done():
				Fail("context cancelled during steady-state observation")
			}

			afterReconciles := queryMetricOrZero(ctx,
				`controller_runtime_reconcile_total{controller="machine-controller"}`,
				"controller_runtime_reconcile_total", nil)
			afterQueueAdds := queryMetricOrZero(ctx,
				`workqueue_adds_total{name="machine-controller"}`,
				"workqueue_adds_total", nil)
			queueDepth := queryMetricOrZero(ctx,
				`workqueue_depth{name="machine-controller"}`,
				"workqueue_depth", nil)

			rvAfter, err := framework.SnapshotMachineResourceVersions(ctx, clients.Machine, msName)
			Expect(err).NotTo(HaveOccurred(), "snapshot machine resourceVersions after steady-state")

			reconcileDelta := afterReconciles - beforeReconciles
			queueAddsDelta := afterQueueAdds - beforeQueueAdds
			rvChanges := framework.CountResourceVersionChanges(rvBefore, rvAfter)
			reconcileRate := reconcileDelta / (float64(steadyStateSeconds) / 60.0)

			result.SteadyState = &framework.SteadyStateResult{
				WindowDuration:        steadyStateDuration.String(),
				ReconcileDelta:        reconcileDelta,
				ReconcileRate:         reconcileRate,
				QueueAddsDelta:        queueAddsDelta,
				QueueDepthEnd:         queueDepth,
				ResourceVersionDeltas: rvChanges,
				MachineCount:          len(rvBefore),
			}

			metrics, err := framework.ScrapeOperatorMetrics(ctx, clients.Kube,
				framework.MachineAPINamespace, "api=clusterapi,k8s-app=controller-manager")
			if err == nil {
				GinkgoWriter.Printf("MAO metrics:\n%s\n", metrics)
			} else {
				GinkgoWriter.Printf("warning: could not scrape MAO metrics: %v\n", err)
			}

			printPerfSummary(result)
			printSteadyStateSummary(result.SteadyState)

			writePerfResults(resultsDir, result)
		})

	It("PERF-02: should measure new-machine latency while existing machines are Running",
		NodeTimeout(30*time.Minute),
		func(ctx SpecContext) {
			if result == nil || msName == "" {
				Skip("PERF-01 did not complete — no background machines available")
			}

			bgCount := 0
			for _, m := range result.Machines {
				if !m.Running.IsZero() {
					bgCount++
				}
			}
			if bgCount == 0 {
				Skip("PERF-01 produced no Running machines — cannot test under load")
			}

			sets := listMachineSets()
			Expect(sets).NotTo(BeEmpty(), "cluster must have at least one MachineSet")

			const newMachineCount = 5
			newMLName = fmt.Sprintf("perf-newml-%s", perfRandomSuffix())
			ms := framework.CloneMachineSetSameSpec(sets[0], newMLName)
			replicas := int32(newMachineCount)
			ms.Spec.Replicas = &replicas

			By(fmt.Sprintf("creating %d new machines while %d existing machines are Running", newMachineCount, bgCount))
			t0 := time.Now()
			GinkgoWriter.Printf("creating new-machine-latency MachineSet %s with replicas=%d at %s (background: %d Running machines)\n",
				newMLName, newMachineCount, t0.Format(time.RFC3339), bgCount)
			_, err := framework.CreateMachineSet(ctx, clients.Machine, ms)
			Expect(err).NotTo(HaveOccurred(), "create new-machine-latency MachineSet")

			newResult, watchErr := framework.WatchMachinePhaseTimestamps(ctx, clients.Machine, newMLName, newMachineCount, framework.PerfTimeout)
			Expect(newResult).NotTo(BeNil(), "result must be returned even on partial completion")

			runningCount := 0
			for _, m := range newResult.Machines {
				if !m.Running.IsZero() {
					runningCount++
				}
			}

			if watchErr != nil {
				GinkgoWriter.Printf("WARNING: not all new machines reached Running: %v\n", watchErr)
				GinkgoWriter.Printf("  %d/%d new machines Running\n", runningCount, newMachineCount)
			}
			Expect(runningCount).To(BeNumerically(">", 0), "at least one new machine must reach Running")

			totalDuration := ""
			if !newResult.AllRunningTime.IsZero() {
				totalDuration = newResult.AllRunningTime.Sub(t0).String()
			} else {
				totalDuration = time.Since(t0).String()
			}

			nml := &framework.NewMachineLatencyResult{
				BackgroundMachines: bgCount,
				NewMachineCount:    newMachineCount,
				TotalDuration:      totalDuration,
				Machines:           newResult.Machines,
			}

			GinkgoWriter.Printf("\n=== New-Machine Latency Under Load ===\n")
			GinkgoWriter.Printf("Background machines:     %d\n", bgCount)
			GinkgoWriter.Printf("New machines created:    %d\n", newMachineCount)
			GinkgoWriter.Printf("New machines Running:    %d\n", runningCount)
			GinkgoWriter.Printf("Total time:              %s\n", totalDuration)
			printNewMLSummary(nml, t0)

			result.NewMachineLatency = nml
			writePerfResults(resultsDir, result)
		})
})

func cleanupMachineSet(ctx SpecContext, name string) {
	GinkgoWriter.Printf("cleanup: releasing DHCP leases for %s\n", name)
	if err := framework.ReleaseDHCPLeases(ctx, clients.Machine, name, GinkgoWriter.Printf); err != nil {
		GinkgoWriter.Printf("cleanup: DHCP release failed (best-effort) for %s: %v\n", name, err)
	}
	GinkgoWriter.Printf("cleanup: scaling MachineSet %s to 0\n", name)
	_ = framework.ScaleMachineSet(ctx, clients.Machine, name, 0)
	err := framework.WaitForMachineSetDrainedWithLog(ctx, clients.Machine, name, 20*time.Minute)
	if err != nil {
		GinkgoWriter.Printf("cleanup: drain failed for %s, force-deleting machines: %v\n", name, err)
		framework.ForceDeleteMachineSetMachines(ctx, clients.Machine, name)
		_ = framework.WaitForMachineSetDrainedWithDelete(ctx, clients.Machine, name, 10*time.Minute)
	}
	_ = framework.DeleteMachineSet(ctx, clients.Machine, name)
	GinkgoWriter.Printf("cleanup: MachineSet %s deleted\n", name)
}

func writePerfResults(dir string, result *framework.PerfBenchmarkResult) {
	Expect(os.MkdirAll(dir, 0o755)).To(Succeed())
	outPath := filepath.Join(dir, "perf-results.json")
	data, err := json.MarshalIndent(result, "", "  ")
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(outPath, data, 0o644)).To(Succeed())
	GinkgoWriter.Printf("perf results written to %s\n", outPath)
}

func perfRandomSuffix() string {
	b := make([]byte, 2)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func printNewMLSummary(nml *framework.NewMachineLatencyResult, t0 time.Time) {
	if len(nml.Machines) == 0 {
		GinkgoWriter.Printf("no new-machine timing data to summarize\n")
		return
	}

	var totals []time.Duration
	var pendingToProvisioning []time.Duration
	var provisioningToProvisioned []time.Duration
	var provisionedToRunning []time.Duration

	GinkgoWriter.Printf("\n%-50s %12s %12s %12s %12s %12s\n",
		"Machine", "Created+", "Provisioning+", "Provisioned+", "Running+", "Total")
	GinkgoWriter.Printf("%s\n", "----------------------------------------------------------------------------------------------------------------------------")

	sort.Slice(nml.Machines, func(i, j int) bool {
		return nml.Machines[i].Created.Before(nml.Machines[j].Created)
	})

	for _, m := range nml.Machines {
		if m.Running.IsZero() {
			continue
		}

		total := m.Running.Sub(m.Created)
		totals = append(totals, total)

		createdOffset := m.Created.Sub(t0)
		provisioningOffset := m.Provisioning.Sub(t0)
		provisionedOffset := m.Provisioned.Sub(t0)
		runningOffset := m.Running.Sub(t0)

		if !m.Provisioning.IsZero() {
			pendingToProvisioning = append(pendingToProvisioning, m.Provisioning.Sub(m.Created))
		}
		if !m.Provisioning.IsZero() && !m.Provisioned.IsZero() {
			provisioningToProvisioned = append(provisioningToProvisioned, m.Provisioned.Sub(m.Provisioning))
		}
		if !m.Provisioned.IsZero() {
			provisionedToRunning = append(provisionedToRunning, m.Running.Sub(m.Provisioned))
		}

		GinkgoWriter.Printf("%-50s %12s %12s %12s %12s %12s\n",
			truncateName(m.Name, 50),
			fmtDuration(createdOffset),
			fmtDuration(provisioningOffset),
			fmtDuration(provisionedOffset),
			fmtDuration(runningOffset),
			fmtDuration(total))
	}

	if len(totals) > 0 {
		GinkgoWriter.Printf("\nPer-machine total latency:\n")
		GinkgoWriter.Printf("  p50: %s\n", fmtDuration(percentile(totals, 0.50)))
		if len(totals) >= 5 {
			GinkgoWriter.Printf("  p90: %s\n", fmtDuration(percentile(totals, 0.90)))
		}
	}

	if len(pendingToProvisioning) > 0 {
		GinkgoWriter.Printf("\nPhase breakdown (p50):\n")
		GinkgoWriter.Printf("  Pending → Provisioning:      %s\n", fmtDuration(percentile(pendingToProvisioning, 0.50)))
	}
	if len(provisioningToProvisioned) > 0 {
		GinkgoWriter.Printf("  Provisioning → Provisioned:  %s\n", fmtDuration(percentile(provisioningToProvisioned, 0.50)))
	}
	if len(provisionedToRunning) > 0 {
		GinkgoWriter.Printf("  Provisioned → Running:       %s\n", fmtDuration(percentile(provisionedToRunning, 0.50)))
	}
}

func printPerfSummary(result *framework.PerfBenchmarkResult) {
	if len(result.Machines) == 0 {
		GinkgoWriter.Printf("no machine timing data to summarize\n")
		return
	}

	var totals []time.Duration
	var pendingToProvisioning []time.Duration
	var provisioningToProvisioned []time.Duration
	var provisionedToRunning []time.Duration

	GinkgoWriter.Printf("\n%-50s %12s %12s %12s %12s %12s\n",
		"Machine", "Created+", "Provisioning+", "Provisioned+", "Running+", "Total")
	GinkgoWriter.Printf("%s\n", "----------------------------------------------------------------------------------------------------------------------------")

	sort.Slice(result.Machines, func(i, j int) bool {
		return result.Machines[i].Created.Before(result.Machines[j].Created)
	})

	for _, m := range result.Machines {
		if m.Running.IsZero() {
			continue
		}

		total := m.Running.Sub(m.Created)
		totals = append(totals, total)

		createdOffset := m.Created.Sub(result.ScaleRequestTime)
		provisioningOffset := m.Provisioning.Sub(result.ScaleRequestTime)
		provisionedOffset := m.Provisioned.Sub(result.ScaleRequestTime)
		runningOffset := m.Running.Sub(result.ScaleRequestTime)

		if !m.Provisioning.IsZero() {
			pendingToProvisioning = append(pendingToProvisioning, m.Provisioning.Sub(m.Created))
		}
		if !m.Provisioning.IsZero() && !m.Provisioned.IsZero() {
			provisioningToProvisioned = append(provisioningToProvisioned, m.Provisioned.Sub(m.Provisioning))
		}
		if !m.Provisioned.IsZero() {
			provisionedToRunning = append(provisionedToRunning, m.Running.Sub(m.Provisioned))
		}

		GinkgoWriter.Printf("%-50s %12s %12s %12s %12s %12s\n",
			truncateName(m.Name, 50),
			fmtDuration(createdOffset),
			fmtDuration(provisioningOffset),
			fmtDuration(provisionedOffset),
			fmtDuration(runningOffset),
			fmtDuration(total))
	}

	totalTime := result.AllRunningTime.Sub(result.ScaleRequestTime)
	throughput := float64(len(totals)) / totalTime.Minutes()

	GinkgoWriter.Printf("\n=== Summary ===\n")
	GinkgoWriter.Printf("Total time:              %s\n", fmtDuration(totalTime))
	GinkgoWriter.Printf("Machines provisioned:    %d\n", len(totals))
	GinkgoWriter.Printf("Throughput:              %.2f machines/min\n", throughput)

	if len(totals) > 0 {
		GinkgoWriter.Printf("\nPer-machine total latency:\n")
		GinkgoWriter.Printf("  p50: %s\n", fmtDuration(percentile(totals, 0.50)))
		GinkgoWriter.Printf("  p90: %s\n", fmtDuration(percentile(totals, 0.90)))
		GinkgoWriter.Printf("  p99: %s\n", fmtDuration(percentile(totals, 0.99)))
	}

	if len(pendingToProvisioning) > 0 {
		GinkgoWriter.Printf("\nPhase breakdown (p50):\n")
		GinkgoWriter.Printf("  Pending → Provisioning:      %s\n", fmtDuration(percentile(pendingToProvisioning, 0.50)))
	}
	if len(provisioningToProvisioned) > 0 {
		GinkgoWriter.Printf("  Provisioning → Provisioned:  %s\n", fmtDuration(percentile(provisioningToProvisioned, 0.50)))
	}
	if len(provisionedToRunning) > 0 {
		GinkgoWriter.Printf("  Provisioned → Running:       %s\n", fmtDuration(percentile(provisionedToRunning, 0.50)))
	}
}

func percentile(durations []time.Duration, p float64) time.Duration {
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

func fmtDuration(d time.Duration) string {
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%02ds", m, s)
}

func truncateName(name string, max int) string {
	if len(name) <= max {
		return name
	}
	return name[:max-3] + "..."
}

func queryMetricOrZero(ctx SpecContext, query, metricName string, labels map[string]string) float64 {
	text, err := framework.QueryPromQL(ctx, clients.Kube, query)
	if err != nil {
		GinkgoWriter.Printf("warning: query %q failed: %v\n", query, err)
		return 0
	}
	val, err := framework.ParseMetricValue(text, metricName, labels)
	if err != nil {
		GinkgoWriter.Printf("warning: parse %q from query %q: %v\n", metricName, query, err)
		return 0
	}
	return val
}

func printSteadyStateSummary(ss *framework.SteadyStateResult) {
	if ss == nil {
		return
	}
	GinkgoWriter.Printf("\n=== Steady-State Observation ===\n")
	GinkgoWriter.Printf("Window:                  %s\n", ss.WindowDuration)
	GinkgoWriter.Printf("Reconcile delta:         %.0f  (%.1f/min)\n", ss.ReconcileDelta, ss.ReconcileRate)
	GinkgoWriter.Printf("Queue adds delta:        %.0f\n", ss.QueueAddsDelta)
	GinkgoWriter.Printf("Queue depth at end:      %.0f\n", ss.QueueDepthEnd)
	GinkgoWriter.Printf("Machine RV changes:      %d/%d\n", ss.ResourceVersionDeltas, ss.MachineCount)
}
