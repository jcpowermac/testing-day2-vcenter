package framework

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jcallen/testing-day2-vcenter/pkg/labconfig"
	machinev1 "github.com/openshift/api/machine/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

// ExtractVSphereMachineProviderSpec unmarshals the raw providerSpec from a Machine.
func ExtractVSphereMachineProviderSpec(m *machinev1beta1.Machine) (*machinev1beta1.VSphereMachineProviderSpec, error) {
	if m.Spec.ProviderSpec.Value == nil {
		return nil, fmt.Errorf("machine %s has nil providerSpec", m.Name)
	}
	spec := &machinev1beta1.VSphereMachineProviderSpec{}
	if err := json.Unmarshal(m.Spec.ProviderSpec.Value.Raw, spec); err != nil {
		return nil, fmt.Errorf("unmarshal machine %s providerSpec: %w", m.Name, err)
	}
	return spec, nil
}

// ExtractVSphereMachineSetProviderSpec unmarshals the raw providerSpec from a MachineSet template.
func ExtractVSphereMachineSetProviderSpec(ms *machinev1beta1.MachineSet) (*machinev1beta1.VSphereMachineProviderSpec, error) {
	if ms.Spec.Template.Spec.ProviderSpec.Value == nil {
		return nil, fmt.Errorf("machineset %s has nil providerSpec", ms.Name)
	}
	spec := &machinev1beta1.VSphereMachineProviderSpec{}
	if err := json.Unmarshal(ms.Spec.Template.Spec.ProviderSpec.Value.Raw, spec); err != nil {
		return nil, fmt.Errorf("unmarshal machineset %s providerSpec: %w", ms.Name, err)
	}
	return spec, nil
}

// CPMSVSphereFailureDomainNames returns the failure domain names from a CPMS spec.
func CPMSVSphereFailureDomainNames(cpms *machinev1.ControlPlaneMachineSet) []string {
	if cpms.Spec.Template.OpenShiftMachineV1Beta1Machine == nil {
		return nil
	}
	fds := cpms.Spec.Template.OpenShiftMachineV1Beta1Machine.FailureDomains
	if fds == nil {
		return nil
	}
	names := make([]string, 0, len(fds.VSphere))
	for _, fd := range fds.VSphere {
		names = append(names, fd.Name)
	}
	return names
}

// CloneMachineSetForVAP clones an existing MachineSet with the given replica
// count and overridden region/zone labels, preserving the providerSpec so the
// MAO admission webhook accepts the create.
func CloneMachineSetForVAP(source machinev1beta1.MachineSet, name, region, zone string, replicas int32) *machinev1beta1.MachineSet {
	ms := &machinev1beta1.MachineSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: source.Namespace,
			Labels: map[string]string{
				MachineRegionLabel: region,
				MachineZoneLabel:   zone,
			},
		},
		Spec: machinev1beta1.MachineSetSpec{
			Replicas: &replicas,
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"machine.openshift.io/cluster-api-machineset": name,
				},
			},
			Template: machinev1beta1.MachineTemplateSpec{
				ObjectMeta: machinev1beta1.ObjectMeta{
					Labels: map[string]string{
						"machine.openshift.io/cluster-api-machineset": name,
						MachineRegionLabel: region,
						MachineZoneLabel:   zone,
					},
				},
				Spec: machinev1beta1.MachineSpec{
					ProviderSpec: source.Spec.Template.Spec.ProviderSpec,
				},
			},
		},
	}
	return ms
}

// ScaleMachineSet sets the replica count on a MachineSet.
func ScaleMachineSet(ctx context.Context, client machineclient.Interface, name string, replicas int32) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		ms, err := client.MachineV1beta1().MachineSets(MachineAPINamespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("get machineset %s: %w", name, err)
		}
		ms.Spec.Replicas = &replicas
		_, err = client.MachineV1beta1().MachineSets(MachineAPINamespace).Update(ctx, ms, metav1.UpdateOptions{})
		return err
	})
}

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
		if m.DeletionTimestamp != nil {
			continue
		}
		if m.Status.Phase != nil && *m.Status.Phase == "Running" {
			ready++
		}
	}
	if ready < count {
		return fmt.Errorf("machineset %s has %d/%d running machines", msName, ready, count)
	}
	return nil
}

// WaitForMachineSetDrained waits until a MachineSet has no Machines.
func WaitForMachineSetDrained(ctx context.Context, client machineclient.Interface, msName string) error {
	machines, err := client.MachineV1beta1().Machines(MachineAPINamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "machine.openshift.io/cluster-api-machineset=" + msName,
	})
	if err != nil {
		return fmt.Errorf("list machines for machineset %s: %w", msName, err)
	}
	if len(machines.Items) > 0 {
		return fmt.Errorf("machineset %s still has %d machines", msName, len(machines.Items))
	}
	return nil
}

// WaitForMachineSetDrainedWithDelete polls until a MachineSet has no Machines.
func WaitForMachineSetDrainedWithDelete(ctx context.Context, client machineclient.Interface, msName string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, DefaultPolling, timeout, true, func(ctx context.Context) (bool, error) {
		err := WaitForMachineSetDrained(ctx, client, msName)
		return err == nil, nil
	})
}

// WaitForMachineSetDrainedWithLog polls until a MachineSet has no Machines,
// logging remaining Machine names and phases every 30 seconds.
func WaitForMachineSetDrainedWithLog(ctx context.Context, client machineclient.Interface, msName string, timeout time.Duration) error {
	lastLog := time.Now()
	return wait.PollUntilContextTimeout(ctx, DefaultPolling, timeout, true, func(ctx context.Context) (bool, error) {
		machines, err := client.MachineV1beta1().Machines(MachineAPINamespace).List(ctx, metav1.ListOptions{
			LabelSelector: "machine.openshift.io/cluster-api-machineset=" + msName,
		})
		if err != nil {
			return false, nil
		}
		if len(machines.Items) == 0 {
			return true, nil
		}
		if time.Since(lastLog) >= 30*time.Second {
			for _, m := range machines.Items {
				phase := "<unknown>"
				if m.Status.Phase != nil {
					phase = *m.Status.Phase
				}
				deleting := ""
				if m.DeletionTimestamp != nil {
					deleting = ", deleting"
				}
				fmt.Printf("  drain-wait: %s phase=%s%s\n", m.Name, phase, deleting)
			}
			fmt.Printf("  drain-wait: %d machine(s) remaining for %s\n", len(machines.Items), msName)
			lastLog = time.Now()
		}
		return false, nil
	})
}

// ForceDeleteMachineSetMachines deletes all Machines belonging to a MachineSet.
func ForceDeleteMachineSetMachines(ctx context.Context, client machineclient.Interface, msName string) {
	machines, err := client.MachineV1beta1().Machines(MachineAPINamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "machine.openshift.io/cluster-api-machineset=" + msName,
	})
	if err != nil {
		return
	}
	for _, m := range machines.Items {
		_ = client.MachineV1beta1().Machines(MachineAPINamespace).Delete(ctx, m.Name, metav1.DeleteOptions{})
	}
}

// WaitForAllMachinesHealthy polls until every non-deleting Machine is in
// "Running" phase. Returns an error listing unhealthy Machines on timeout.
func WaitForAllMachinesHealthy(ctx context.Context, client machineclient.Interface, timeout time.Duration) error {
	var lastErr error
	lastLog := time.Now()
	pollErr := wait.PollUntilContextTimeout(ctx, DefaultPolling, timeout, true, func(ctx context.Context) (bool, error) {
		machines, err := client.MachineV1beta1().Machines(MachineAPINamespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			lastErr = err
			return false, nil
		}
		var unhealthy []string
		for _, m := range machines.Items {
			if m.DeletionTimestamp != nil {
				continue
			}
			phase := ""
			if m.Status.Phase != nil {
				phase = *m.Status.Phase
			}
			// Skip Failed Machines that were never provisioned (no NodeRef).
			// These are dead CPMS replacements that won't self-heal.
			if phase == "Failed" && m.Status.NodeRef == nil {
				continue
			}
			if phase != "Running" {
				unhealthy = append(unhealthy, fmt.Sprintf("%s(%s)", m.Name, phase))
			}
		}
		if len(unhealthy) > 0 {
			lastErr = fmt.Errorf("%d machines not Running: %v", len(unhealthy), unhealthy)
			if time.Since(lastLog) >= 30*time.Second {
				fmt.Printf("  wait: %d machine(s) not Running: %v\n", len(unhealthy), unhealthy)
				lastLog = time.Now()
			}
			return false, nil
		}
		lastErr = nil
		return true, nil
	})
	if pollErr != nil && lastErr != nil {
		return fmt.Errorf("%w: %v", pollErr, lastErr)
	}
	return pollErr
}

// WaitForAllMachinesLabeled polls until every Running, non-deleting Machine
// has region and zone labels. After an Infrastructure spec change the CCM
// syncs vCenter tags to these labels asynchronously.
func WaitForAllMachinesLabeled(ctx context.Context, client machineclient.Interface, timeout time.Duration) error {
	var lastErr error
	lastLog := time.Now()
	pollErr := wait.PollUntilContextTimeout(ctx, DefaultPolling, timeout, true, func(ctx context.Context) (bool, error) {
		machines, err := client.MachineV1beta1().Machines(MachineAPINamespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			lastErr = err
			return false, nil
		}
		var unlabeled []string
		for _, m := range machines.Items {
			if m.DeletionTimestamp != nil {
				continue
			}
			phase := ""
			if m.Status.Phase != nil {
				phase = *m.Status.Phase
			}
			if phase != "Running" {
				continue
			}
			if m.Labels == nil || m.Labels[MachineRegionLabel] == "" || m.Labels[MachineZoneLabel] == "" {
				unlabeled = append(unlabeled, m.Name)
			}
		}
		if len(unlabeled) > 0 {
			lastErr = fmt.Errorf("%d machines missing region/zone labels: %v", len(unlabeled), unlabeled)
			if time.Since(lastLog) >= 30*time.Second {
				fmt.Printf("  wait: %d machine(s) missing region/zone labels: %v\n", len(unlabeled), unlabeled)
				lastLog = time.Now()
			}
			return false, nil
		}
		lastErr = nil
		return true, nil
	})
	if pollErr != nil && lastErr != nil {
		return fmt.Errorf("%w: %v", pollErr, lastErr)
	}
	return pollErr
}

// CreateMachineSet creates a MachineSet in the machine API namespace.
func CreateMachineSet(ctx context.Context, client machineclient.Interface, ms *machinev1beta1.MachineSet) (*machinev1beta1.MachineSet, error) {
	return client.MachineV1beta1().MachineSets(MachineAPINamespace).Create(ctx, ms, metav1.CreateOptions{})
}

// DeleteMachineSet deletes a MachineSet by name in the machine API namespace.
func DeleteMachineSet(ctx context.Context, client machineclient.Interface, name string) error {
	return client.MachineV1beta1().MachineSets(MachineAPINamespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// CloneMachineSetForFD clones a MachineSet with providerSpec workspace rewritten
// to target the failure domain described in labCfg. This sets workspace fields
// (server, datacenter, datastore, resourcePool, folder), network, template path,
// and region/zone labels.
func CloneMachineSetForFD(source machinev1beta1.MachineSet, name string, labCfg *labconfig.LabConfig) (*machinev1beta1.MachineSet, error) {
	if labCfg.FailureDomain == nil {
		return nil, fmt.Errorf("lab config has no failureDomain")
	}

	srcSpec, err := ExtractVSphereMachineSetProviderSpec(&source)
	if err != nil {
		return nil, fmt.Errorf("extract source providerSpec: %w", err)
	}

	fd := labCfg.FailureDomain
	topo := fd.Topology

	newSpec := srcSpec.DeepCopy()
	if newSpec.Workspace == nil {
		newSpec.Workspace = &machinev1beta1.Workspace{}
	}
	newSpec.Workspace.Server = labCfg.SecondVCenter.Server
	newSpec.Workspace.Datacenter = topo.Datacenter
	newSpec.Workspace.Datastore = topo.Datastore
	newSpec.Workspace.ResourcePool = topo.ResourcePool

	// Derive folder: /<datacenter>/vm/<infraID> from source folder
	if srcSpec.Workspace != nil && srcSpec.Workspace.Folder != "" {
		newSpec.Workspace.Folder = rewriteFolderDatacenter(srcSpec.Workspace.Folder, topo.Datacenter)
	}

	if topo.Template != "" {
		newSpec.Template = topo.Template
	} else if srcSpec.Template != "" {
		newSpec.Template = rewriteTemplateDatacenter(srcSpec.Template, topo.Datacenter)
	}

	if len(topo.Networks) > 0 {
		devices := make([]machinev1beta1.NetworkDeviceSpec, len(topo.Networks))
		for i, net := range topo.Networks {
			devices[i] = machinev1beta1.NetworkDeviceSpec{NetworkName: net}
		}
		newSpec.Network = machinev1beta1.NetworkSpec{Devices: devices}
	}

	rawSpec, err := json.Marshal(newSpec)
	if err != nil {
		return nil, fmt.Errorf("marshal providerSpec: %w", err)
	}

	replicas := int32(1)
	ms := &machinev1beta1.MachineSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: source.Namespace,
			Labels: map[string]string{
				MachineRegionLabel: fd.Region,
				MachineZoneLabel:   fd.Zone,
			},
		},
		Spec: machinev1beta1.MachineSetSpec{
			Replicas: &replicas,
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"machine.openshift.io/cluster-api-machineset": name,
				},
			},
			Template: machinev1beta1.MachineTemplateSpec{
				ObjectMeta: machinev1beta1.ObjectMeta{
					Labels: map[string]string{
						"machine.openshift.io/cluster-api-machineset": name,
						MachineRegionLabel: fd.Region,
						MachineZoneLabel:   fd.Zone,
					},
				},
				Spec: machinev1beta1.MachineSpec{
					ProviderSpec: machinev1beta1.ProviderSpec{
						Value: &runtime.RawExtension{Raw: rawSpec},
					},
				},
			},
		},
	}
	return ms, nil
}

// rewriteFolderDatacenter replaces the datacenter in a vSphere folder path.
// Input: "/<oldDC>/vm/<infraID>" → "/<newDC>/vm/<infraID>"
func rewriteFolderDatacenter(folder, newDC string) string {
	parts := splitVSpherePath(folder)
	if len(parts) >= 1 {
		parts[0] = newDC
	}
	return "/" + joinVSpherePath(parts)
}

// rewriteTemplateDatacenter replaces the datacenter in a vSphere template path.
// Input: "/<oldDC>/vm/<infraID>/<infraID>-rhcos" → "/<newDC>/vm/<infraID>/<infraID>-rhcos"
func rewriteTemplateDatacenter(template, newDC string) string {
	parts := splitVSpherePath(template)
	if len(parts) >= 1 {
		parts[0] = newDC
	}
	return "/" + joinVSpherePath(parts)
}

func splitVSpherePath(p string) []string {
	var parts []string
	for _, s := range splitOn(p, '/') {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return parts
}

func splitOn(s string, sep byte) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func joinVSpherePath(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += "/" + p
	}
	return result
}


// MachineTimingRecord holds per-machine phase transition timestamps.
type MachineTimingRecord struct {
	Name         string    `json:"name"`
	Created      time.Time `json:"created"`
	Provisioning time.Time `json:"provisioning"`
	Provisioned  time.Time `json:"provisioned"`
	Running      time.Time `json:"running"`
}

// PerfBenchmarkResult holds the full benchmark output.
type PerfBenchmarkResult struct {
	ScaleRequestTime time.Time             `json:"scaleRequestTime"`
	AllRunningTime   time.Time             `json:"allRunningTime"`
	TargetCount      int                   `json:"targetCount"`
	TotalDuration    string                `json:"totalDuration"`
	Machines         []MachineTimingRecord `json:"machines"`
}

// CloneMachineSetSameSpec clones a MachineSet preserving the source's provider
// spec and region/zone labels but with a new name and replicas=0.
func CloneMachineSetSameSpec(source machinev1beta1.MachineSet, newName string) *machinev1beta1.MachineSet {
	region := ""
	zone := ""
	if source.Spec.Template.Labels != nil {
		region = source.Spec.Template.Labels[MachineRegionLabel]
		zone = source.Spec.Template.Labels[MachineZoneLabel]
	}

	replicas := int32(0)
	ms := &machinev1beta1.MachineSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      newName,
			Namespace: source.Namespace,
			Labels: map[string]string{
				MachineRegionLabel: region,
				MachineZoneLabel:   zone,
			},
		},
		Spec: machinev1beta1.MachineSetSpec{
			Replicas: &replicas,
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"machine.openshift.io/cluster-api-machineset": newName,
				},
			},
			Template: machinev1beta1.MachineTemplateSpec{
				ObjectMeta: machinev1beta1.ObjectMeta{
					Labels: map[string]string{
						"machine.openshift.io/cluster-api-machineset": newName,
						MachineRegionLabel: region,
						MachineZoneLabel:   zone,
					},
				},
				Spec: machinev1beta1.MachineSpec{
					ProviderSpec: source.Spec.Template.Spec.ProviderSpec,
				},
			},
		},
	}
	return ms
}

// WatchMachinePhaseTimestamps polls machines belonging to a MachineSet and records
// the wall-clock time at which each machine first reaches each phase. Returns when
// targetCount machines have reached Running.
func WatchMachinePhaseTimestamps(ctx context.Context, client machineclient.Interface, msName string, targetCount int, timeout time.Duration) (*PerfBenchmarkResult, error) {
	records := make(map[string]*MachineTimingRecord)
	lastLog := time.Now()

	var lastErr error
	pollErr := wait.PollUntilContextTimeout(ctx, DefaultPolling, timeout, true, func(ctx context.Context) (bool, error) {
		machines, err := client.MachineV1beta1().Machines(MachineAPINamespace).List(ctx, metav1.ListOptions{
			LabelSelector: "machine.openshift.io/cluster-api-machineset=" + msName,
		})
		if err != nil {
			lastErr = err
			return false, nil
		}

		now := time.Now()
		runningCount := 0
		provisioningCount := 0
		provisionedCount := 0

		for _, m := range machines.Items {
			if m.DeletionTimestamp != nil {
				continue
			}

			rec, exists := records[m.Name]
			if !exists {
				rec = &MachineTimingRecord{
					Name:    m.Name,
					Created: m.CreationTimestamp.Time,
				}
				records[m.Name] = rec
			}

			phase := ""
			if m.Status.Phase != nil {
				phase = *m.Status.Phase
			}

			switch phase {
			case "Provisioning":
				if rec.Provisioning.IsZero() {
					rec.Provisioning = now
				}
				provisioningCount++
			case "Provisioned":
				if rec.Provisioning.IsZero() {
					rec.Provisioning = now
				}
				if rec.Provisioned.IsZero() {
					rec.Provisioned = now
				}
				provisionedCount++
			case "Running":
				if rec.Provisioning.IsZero() {
					rec.Provisioning = now
				}
				if rec.Provisioned.IsZero() {
					rec.Provisioned = now
				}
				if rec.Running.IsZero() {
					rec.Running = now
				}
				runningCount++
			}
		}

		if time.Since(lastLog) >= 30*time.Second {
			fmt.Printf("  perf-watch: %d/%d Running, %d Provisioned, %d Provisioning, %d total machines\n",
				runningCount, targetCount, provisionedCount, provisioningCount, len(machines.Items))
			lastLog = now
		}

		lastErr = nil
		if runningCount >= targetCount {
			return true, nil
		}
		lastErr = fmt.Errorf("%d/%d machines Running", runningCount, targetCount)
		return false, nil
	})

	result := &PerfBenchmarkResult{
		TargetCount: targetCount,
	}
	for _, rec := range records {
		result.Machines = append(result.Machines, *rec)
		if !rec.Running.IsZero() && rec.Running.After(result.AllRunningTime) {
			result.AllRunningTime = rec.Running
		}
	}

	if pollErr != nil {
		if lastErr != nil {
			return result, fmt.Errorf("%w: %v", pollErr, lastErr)
		}
		return result, pollErr
	}
	return result, nil
}
