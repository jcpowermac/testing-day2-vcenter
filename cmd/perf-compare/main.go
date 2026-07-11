package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"time"
)

type MachineTimingRecord struct {
	Name         string    `json:"name"`
	Created      time.Time `json:"created"`
	Provisioning time.Time `json:"provisioning"`
	Provisioned  time.Time `json:"provisioned"`
	Running      time.Time `json:"running"`
}

type PerfBenchmarkResult struct {
	ScaleRequestTime  time.Time                `json:"scaleRequestTime"`
	AllRunningTime    time.Time                `json:"allRunningTime"`
	TargetCount       int                      `json:"targetCount"`
	TotalDuration     string                   `json:"totalDuration"`
	Machines          []MachineTimingRecord    `json:"machines"`
	SteadyState       *SteadyStateResult       `json:"steadyState,omitempty"`
	NewMachineLatency *NewMachineLatencyResult `json:"newMachineLatency,omitempty"`
}

type SteadyStateResult struct {
	WindowDuration        string  `json:"windowDuration"`
	ReconcileDelta        float64 `json:"reconcileDelta"`
	ReconcileRate         float64 `json:"reconcileRate"`
	QueueAddsDelta        float64 `json:"queueAddsDelta"`
	QueueDepthEnd         float64 `json:"queueDepthEnd"`
	ResourceVersionDeltas int     `json:"resourceVersionDeltas"`
	MachineCount          int     `json:"machineCount"`
}

type NewMachineLatencyResult struct {
	BackgroundMachines int                   `json:"backgroundMachines"`
	NewMachineCount    int                   `json:"newMachineCount"`
	TotalDuration      string                `json:"totalDuration"`
	Machines           []MachineTimingRecord `json:"machines"`
}

type stats struct {
	totalTime               time.Duration
	throughput              float64
	machineCount            int
	totalP50, totalP90      time.Duration
	totalP99                time.Duration
	pendingToProvisionP50   time.Duration
	provisionToReadyP50     time.Duration
	provisionedToRunningP50 time.Duration
	steadyState             *SteadyStateResult
	newML                   *newMLStats
}

type newMLStats struct {
	backgroundMachines      int
	newMachineCount         int
	pendingToProvisionP50   time.Duration
	provisionToReadyP50     time.Duration
	provisionedToRunningP50 time.Duration
	totalP50                time.Duration
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: perf-compare <baseline.json> <pr.json> [--output <file>]\n")
		os.Exit(1)
	}

	baselineFile := os.Args[1]
	prFile := os.Args[2]

	var outputFile string
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--output" && i+1 < len(os.Args) {
			outputFile = os.Args[i+1]
			i++
		}
	}

	baseline, err := loadResult(baselineFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading baseline: %v\n", err)
		os.Exit(1)
	}
	pr, err := loadResult(prFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading PR result: %v\n", err)
		os.Exit(1)
	}

	baseStats := computeStats(baseline)
	prStats := computeStats(pr)

	report := formatComparison(baseStats, prStats)

	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(report), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Comparison written to %s\n", outputFile)
	}
	fmt.Print(report)
}

func loadResult(path string) (*PerfBenchmarkResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result PerfBenchmarkResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func computeStats(r *PerfBenchmarkResult) stats {
	var totals []time.Duration
	var pendingToProvisioning []time.Duration
	var provisioningToProvisioned []time.Duration
	var provisionedToRunning []time.Duration

	for _, m := range r.Machines {
		if m.Running.IsZero() {
			continue
		}
		totals = append(totals, m.Running.Sub(m.Created))
		if !m.Provisioning.IsZero() {
			pendingToProvisioning = append(pendingToProvisioning, m.Provisioning.Sub(m.Created))
		}
		if !m.Provisioning.IsZero() && !m.Provisioned.IsZero() {
			provisioningToProvisioned = append(provisioningToProvisioned, m.Provisioned.Sub(m.Provisioning))
		}
		if !m.Provisioned.IsZero() {
			provisionedToRunning = append(provisionedToRunning, m.Running.Sub(m.Provisioned))
		}
	}

	totalTime := r.AllRunningTime.Sub(r.ScaleRequestTime)

	s := stats{
		totalTime:    totalTime,
		machineCount: len(totals),
	}
	if totalTime > 0 {
		s.throughput = float64(len(totals)) / totalTime.Minutes()
	}
	if len(totals) > 0 {
		s.totalP50 = percentile(totals, 0.50)
		s.totalP90 = percentile(totals, 0.90)
		s.totalP99 = percentile(totals, 0.99)
	}
	if len(pendingToProvisioning) > 0 {
		s.pendingToProvisionP50 = percentile(pendingToProvisioning, 0.50)
	}
	if len(provisioningToProvisioned) > 0 {
		s.provisionToReadyP50 = percentile(provisioningToProvisioned, 0.50)
	}
	if len(provisionedToRunning) > 0 {
		s.provisionedToRunningP50 = percentile(provisionedToRunning, 0.50)
	}
	s.steadyState = r.SteadyState

	if r.NewMachineLatency != nil {
		s.newML = computeNewMLStats(r.NewMachineLatency)
	}

	return s
}

func computeNewMLStats(nml *NewMachineLatencyResult) *newMLStats {
	var totals []time.Duration
	var pendingToProvisioning []time.Duration
	var provisioningToProvisioned []time.Duration
	var provisionedToRunning []time.Duration

	for _, m := range nml.Machines {
		if m.Running.IsZero() {
			continue
		}
		totals = append(totals, m.Running.Sub(m.Created))
		if !m.Provisioning.IsZero() {
			pendingToProvisioning = append(pendingToProvisioning, m.Provisioning.Sub(m.Created))
		}
		if !m.Provisioning.IsZero() && !m.Provisioned.IsZero() {
			provisioningToProvisioned = append(provisioningToProvisioned, m.Provisioned.Sub(m.Provisioning))
		}
		if !m.Provisioned.IsZero() {
			provisionedToRunning = append(provisionedToRunning, m.Running.Sub(m.Provisioned))
		}
	}

	if len(totals) == 0 {
		return nil
	}

	s := &newMLStats{
		backgroundMachines: nml.BackgroundMachines,
		newMachineCount:    nml.NewMachineCount,
		totalP50:           percentile(totals, 0.50),
	}
	if len(pendingToProvisioning) > 0 {
		s.pendingToProvisionP50 = percentile(pendingToProvisioning, 0.50)
	}
	if len(provisioningToProvisioned) > 0 {
		s.provisionToReadyP50 = percentile(provisioningToProvisioned, 0.50)
	}
	if len(provisionedToRunning) > 0 {
		s.provisionedToRunningP50 = percentile(provisionedToRunning, 0.50)
	}
	return s
}

func formatComparison(base, pr stats) string {
	out := ""
	out += "=== MAO Provisioning Performance Comparison ===\n\n"
	out += fmt.Sprintf("%-32s %14s %14s %14s %10s\n", "", "Baseline", "PR", "Delta", "Change")
	out += fmt.Sprintf("%s\n", "--------------------------------------------------------------------------------------------")

	out += fmtRow("Total time", base.totalTime, pr.totalTime)
	out += fmtRowFloat("Throughput (machines/min)", base.throughput, pr.throughput)
	out += "\n"

	out += "Per-machine latency:\n"
	out += fmtRow("  p50", base.totalP50, pr.totalP50)
	out += fmtRow("  p90", base.totalP90, pr.totalP90)
	out += fmtRow("  p99", base.totalP99, pr.totalP99)
	out += "\n"

	out += "Phase breakdown (p50):\n"
	out += fmtRow("  Pending → Provisioning", base.pendingToProvisionP50, pr.pendingToProvisionP50)
	out += fmtRow("  Provisioning → Provisioned", base.provisionToReadyP50, pr.provisionToReadyP50)
	out += fmtRow("  Provisioned → Running", base.provisionedToRunningP50, pr.provisionedToRunningP50)
	out += "\n"

	out += fmt.Sprintf("%-32s %14d %14d\n", "Machines provisioned", base.machineCount, pr.machineCount)

	if base.steadyState != nil || pr.steadyState != nil {
		out += "\n"
		bss := base.steadyState
		pss := pr.steadyState
		if bss == nil {
			bss = &SteadyStateResult{}
		}
		if pss == nil {
			pss = &SteadyStateResult{}
		}
		window := bss.WindowDuration
		if window == "" {
			window = pss.WindowDuration
		}
		out += fmt.Sprintf("Steady-state (%s window):\n", window)
		out += fmtRowFloat("  Reconcile delta", bss.ReconcileDelta, pss.ReconcileDelta)
		out += fmtRowFloat("  Reconcile rate (/min)", bss.ReconcileRate, pss.ReconcileRate)
		out += fmtRowFloat("  Queue adds delta", bss.QueueAddsDelta, pss.QueueAddsDelta)
		out += fmtRowFloat("  Queue depth at end", bss.QueueDepthEnd, pss.QueueDepthEnd)
		out += fmt.Sprintf("%-32s %10d/%-4d %10d/%-4d\n",
			"  Machine RV changes",
			bss.ResourceVersionDeltas, bss.MachineCount,
			pss.ResourceVersionDeltas, pss.MachineCount)
	}

	if base.newML != nil || pr.newML != nil {
		out += "\n"
		bnml := base.newML
		pnml := pr.newML
		if bnml == nil {
			bnml = &newMLStats{}
		}
		if pnml == nil {
			pnml = &newMLStats{}
		}

		bgCount := bnml.backgroundMachines
		if bgCount == 0 {
			bgCount = pnml.backgroundMachines
		}
		newCount := bnml.newMachineCount
		if newCount == 0 {
			newCount = pnml.newMachineCount
		}

		out += fmt.Sprintf("New-machine latency (%d new under %d-machine load, p50):\n", newCount, bgCount)
		out += fmtRow("  Pending → Provisioning", bnml.pendingToProvisionP50, pnml.pendingToProvisionP50)
		out += fmtRow("  Provisioning → Provisioned", bnml.provisionToReadyP50, pnml.provisionToReadyP50)
		out += fmtRow("  Provisioned → Running", bnml.provisionedToRunningP50, pnml.provisionedToRunningP50)
		out += fmtRow("  Total", bnml.totalP50, pnml.totalP50)
	}

	return out
}

func fmtRow(label string, base, pr time.Duration) string {
	delta := pr - base
	pct := 0.0
	if base > 0 {
		pct = float64(delta) / float64(base) * 100
	}
	sign := ""
	if delta > 0 {
		sign = "+"
	}
	return fmt.Sprintf("%-32s %14s %14s %13s%s %9.1f%%\n",
		label, fmtDuration(base), fmtDuration(pr), sign, fmtDuration(delta), pct)
}

func fmtRowFloat(label string, base, pr float64) string {
	delta := pr - base
	pct := 0.0
	if math.Abs(base) > 0.001 {
		pct = delta / base * 100
	}
	sign := ""
	if delta > 0 {
		sign = "+"
	}
	return fmt.Sprintf("%-32s %14.2f %14.2f %13s%.2f %9.1f%%\n",
		label, base, pr, sign, delta, pct)
}

func percentile(durations []time.Duration, p float64) time.Duration {
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

func fmtDuration(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	neg := d < 0
	if neg {
		d = -d
	}
	prefix := ""
	if neg {
		prefix = "-"
	}
	if d < time.Minute {
		return prefix + d.Round(time.Second).String()
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%s%dm%02ds", prefix, m, s)
}
