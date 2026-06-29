package framework

import (
	"context"
	"encoding/json"
	"fmt"

	machinev1 "github.com/openshift/api/machine/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// BuildMachineSet creates a 0-replica MachineSet spec with region/zone labels for VAP testing.
func BuildMachineSet(name, namespace, region, zone string) *machinev1beta1.MachineSet {
	replicas := int32(0)
	return &machinev1beta1.MachineSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
			},
		},
	}
}

// CreateMachineSet creates a MachineSet in the machine API namespace.
func CreateMachineSet(ctx context.Context, client machineclient.Interface, ms *machinev1beta1.MachineSet) (*machinev1beta1.MachineSet, error) {
	return client.MachineV1beta1().MachineSets(MachineAPINamespace).Create(ctx, ms, metav1.CreateOptions{})
}

// DeleteMachineSet deletes a MachineSet by name in the machine API namespace.
func DeleteMachineSet(ctx context.Context, client machineclient.Interface, name string) error {
	return client.MachineV1beta1().MachineSets(MachineAPINamespace).Delete(ctx, name, metav1.DeleteOptions{})
}
