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

			result, err := framework.WatchMachinePhaseTimestamps(ctx, clients.Machine, msName, workerCount, framework.PerfTimeout)
			Expect(err).NotTo(HaveOccurred(), "all machines should reach Running within timeout")

			result.ScaleRequestTime = t0
			result.TotalDuration = result.AllRunningTime.Sub(t0).String()

			GinkgoWriter.Printf("all %d machines Running at %s (total: %s)\n",
				workerCount, result.AllRunningTime.Format(time.RFC3339), result.TotalDuration)

			GinkgoWriter.Printf("waiting for all nodes to be Ready\n")
			Expect(framework.WaitForAllNodesReady(ctx, clients.Kube, framework.PerfTimeout)).To(Succeed(),
				"all nodes should become Ready")

			metrics, err := framework.ScrapeOperatorMetrics(ctx, clients.Kube,
				framework.MachineAPINamespace, "api=clusterapi,k8s-app=controller-manager")
			if err == nil {
				GinkgoWriter.Printf("MAO metrics:\n%s\n", metrics)
			} else {
				GinkgoWriter.Printf("warning: could not scrape MAO metrics: %v\n", err)
			}

			printPerfSummary(result)

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
