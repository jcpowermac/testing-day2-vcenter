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

var _ = Describe("Provisioning performance benchmark", Label("perf", "mutating", "p1"), func() {
	var (
		msName      string
		workerCount int
		resultsDir  string
	)

	BeforeEach(func() {
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
			source := sets[0]
			ms := framework.CloneMachineSetSameSpec(source, msName)
			replicas := int32(workerCount)
			ms.Spec.Replicas = &replicas

			t0 := time.Now()
			GinkgoWriter.Printf("creating benchmark MachineSet %s (cloned from %s) with replicas=%d at %s\n",
				msName, source.Name, workerCount, t0.Format(time.RFC3339))
			_, err := framework.CreateMachineSet(suiteCtx, clients.Machine, ms)
			Expect(err).NotTo(HaveOccurred(), "create benchmark MachineSet")

			DeferCleanup(NodeTimeout(30*time.Minute), func(ctx SpecContext) {
				if msName == "" {
					return
				}
				GinkgoWriter.Printf("cleanup: scaling MachineSet %s to 0\n", msName)
				_ = framework.ScaleMachineSet(ctx, clients.Machine, msName, 0)
				err := framework.WaitForMachineSetDrainedWithLog(ctx, clients.Machine, msName, 20*time.Minute)
				if err != nil {
					GinkgoWriter.Printf("cleanup: drain failed, force-deleting machines: %v\n", err)
					framework.ForceDeleteMachineSetMachines(ctx, clients.Machine, msName)
					_ = framework.WaitForMachineSetDrainedWithDelete(ctx, clients.Machine, msName, 10*time.Minute)
				}
				_ = framework.DeleteMachineSet(ctx, clients.Machine, msName)
				GinkgoWriter.Printf("cleanup: MachineSet %s deleted\n", msName)
			})

			result, watchErr := framework.WatchMachinePhaseTimestamps(ctx, clients.Machine, msName, workerCount, framework.PerfTimeout)
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

			steadyStateSeconds := 300
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

			Expect(os.MkdirAll(resultsDir, 0o755)).To(Succeed())
			outPath := filepath.Join(resultsDir, "perf-results.json")
			data, err := json.MarshalIndent(result, "", "  ")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(outPath, data, 0o644)).To(Succeed())
			GinkgoWriter.Printf("perf results written to %s\n", outPath)
		})
})

func perfRandomSuffix() string {
	b := make([]byte, 2)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
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
