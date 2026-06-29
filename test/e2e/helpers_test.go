package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	"github.com/jcallen/testing-day2-vcenter/pkg/vsphere"
	configv1 "github.com/openshift/api/config/v1"
	machinev1 "github.com/openshift/api/machine/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	suiteCtx    context.Context
	clients     *framework.Clients
	infraBackup *configv1.Infrastructure
	gateEnabled bool
	vapDryRunWorks bool
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "vSphere Multi-vCenter Day 2 E2E Suite")
}

var _ = BeforeSuite(func() {
	suiteCtx = context.Background()

	var err error
	clients, err = framework.NewClients()
	Expect(err).NotTo(HaveOccurred(), "initialize kubernetes clients")

	infra, err := framework.GetInfrastructure(suiteCtx, clients.Config)
	Expect(err).NotTo(HaveOccurred())
	Expect(framework.IsVSphereCluster(infra)).To(BeTrue(), "cluster platform must be VSphere")

	gateEnabled, err = framework.IsFeatureGateEnabled(suiteCtx, clients.Config, framework.VSphereMultiVCenterDay2Gate)
	Expect(err).NotTo(HaveOccurred())

	infraBackup, err = framework.BackupInfrastructure(suiteCtx, clients.Config)
	Expect(err).NotTo(HaveOccurred())

	for _, co := range []string{"cloud-controller-manager", "cluster-config-operator", "machine-api"} {
		err = framework.WaitForClusterOperatorAvailable(suiteCtx, clients.Config, co, framework.ShortTimeout)
		if err != nil {
			GinkgoWriter.Printf("warning: clusteroperator %q not available before suite: %v\n", co, err)
		}
	}

	vapDryRunWorks = probeVAPDryRun()
	GinkgoWriter.Printf("VSphereMultiVCenterDay2 enabled=%v vapDryRunWorks=%v\n", gateEnabled, vapDryRunWorks)
})

var _ = AfterSuite(func() {
	if clients == nil || infraBackup == nil {
		return
	}
	_ = framework.RestoreInfrastructure(suiteCtx, clients.Config, infraBackup)
})

func requireGateEnabled() {
	if !gateEnabled {
		Skip("VSphereMultiVCenterDay2 feature gate is not enabled")
	}
}

func requireGateDisabled() {
	if gateEnabled {
		Skip("VSphereMultiVCenterDay2 feature gate is enabled; gate-off coverage delegated to openshift/api fixtures")
	}
}

func currentInfrastructure() *configv1.Infrastructure {
	infra, err := framework.GetInfrastructure(suiteCtx, clients.Config)
	Expect(err).NotTo(HaveOccurred())
	return infra
}

func patchInfrastructureSpec(spec *configv1.InfrastructureSpec, dryRun bool) (*configv1.Infrastructure, error) {
	return framework.ReplaceInfrastructureSpec(suiteCtx, clients.Config, spec, dryRun)
}

func expectPatchRejected(spec *configv1.InfrastructureSpec, messageFragment string) {
	_, err := patchInfrastructureSpec(spec, true)
	Expect(err).To(HaveOccurred())
	Expect(framework.InfrastructurePatchError(err)).To(ContainSubstring(messageFragment))
}

func expectPatchAllowedDryRun(spec *configv1.InfrastructureSpec) {
	_, err := patchInfrastructureSpec(spec, true)
	Expect(err).NotTo(HaveOccurred())
}

func withInfrastructureRestore(fn func(spec *configv1.InfrastructureSpec)) {
	backup, err := framework.BackupInfrastructure(suiteCtx, clients.Config)
	Expect(err).NotTo(HaveOccurred())

	DeferCleanup(func() {
		_ = framework.RestoreInfrastructure(suiteCtx, clients.Config, backup)
	})

	infra, err := framework.GetInfrastructure(suiteCtx, clients.Config)
	Expect(err).NotTo(HaveOccurred())
	spec := vsphere.CloneInfrastructureSpec(infra.Spec)
	fn(&spec)
}

func listMachines() []machinev1beta1.Machine {
	machines, err := clients.Machine.MachineV1beta1().Machines(framework.MachineAPINamespace).List(suiteCtx, metav1ListOptions())
	Expect(err).NotTo(HaveOccurred())
	return machines.Items
}

func listMachineSets() []machinev1beta1.MachineSet {
	sets, err := clients.Machine.MachineV1beta1().MachineSets(framework.MachineAPINamespace).List(suiteCtx, metav1ListOptions())
	Expect(err).NotTo(HaveOccurred())
	return sets.Items
}

func listCPMS() []machinev1.ControlPlaneMachineSet {
	sets, err := clients.Machine.MachineV1().ControlPlaneMachineSets(framework.MachineAPINamespace).List(suiteCtx, metav1ListOptions())
	Expect(err).NotTo(HaveOccurred())
	return sets.Items
}

func metav1ListOptions() metav1.ListOptions {
	return metav1.ListOptions{}
}

func machineLabeledFailureDomain(m machinev1beta1.Machine) (region, zone string, ok bool) {
	if m.Labels == nil {
		return "", "", false
	}
	region = m.Labels[framework.MachineRegionLabel]
	zone = m.Labels[framework.MachineZoneLabel]
	return region, zone, region != "" && zone != ""
}

func findMachineBackedFailureDomain(infra *configv1.Infrastructure) (region, zone string, found bool) {
	for _, machine := range listMachines() {
		r, z, ok := machineLabeledFailureDomain(machine)
		if !ok {
			continue
		}
		if vsphere.FindFailureDomainByRegionZone(framework.GetFailureDomains(infra), r, z) != nil {
			return r, z, true
		}
	}
	return "", "", false
}

func specWithoutFailureDomain(infra *configv1.Infrastructure, region, zone string) *configv1.InfrastructureSpec {
	spec := vsphere.CloneInfrastructureSpec(infra.Spec)
	Expect(spec.PlatformSpec.VSphere).NotTo(BeNil())
	spec.PlatformSpec.VSphere.FailureDomains = vsphere.RemoveFailureDomainByRegionZone(spec.PlatformSpec.VSphere.FailureDomains, region, zone)
	return &spec
}

func specWithoutVCenter(infra *configv1.Infrastructure, server string) *configv1.InfrastructureSpec {
	spec := vsphere.CloneInfrastructureSpec(infra.Spec)
	Expect(spec.PlatformSpec.VSphere).NotTo(BeNil())
	spec.PlatformSpec.VSphere.VCenters = vsphere.RemoveVCenterByServer(spec.PlatformSpec.VSphere.VCenters, server)
	return &spec
}

func probeVAPDryRun() bool {
	infra := currentInfrastructure()
	if infra.Spec.PlatformSpec.VSphere == nil {
		return false
	}
	region, zone, ok := findMachineBackedFailureDomain(infra)
	if !ok {
		return false
	}

	spec := specWithoutFailureDomain(infra, region, zone)
	_, err := patchInfrastructureSpec(spec, true)
	return err != nil
}

func expectFailureDomainRemovalDenied(infra *configv1.Infrastructure, region, zone string) {
	spec := specWithoutFailureDomain(infra, region, zone)

	tryDryRun := vapDryRunWorks
	if tryDryRun {
		_, err := patchInfrastructureSpec(spec, true)
		if err != nil {
			Expect(framework.InfrastructurePatchError(err)).To(SatisfyAny(
				ContainSubstring("failure domain"),
				ContainSubstring("still in use"),
			))
			return
		}
	}

	_, err := patchInfrastructureSpec(spec, false)
	Expect(err).To(HaveOccurred())
	Expect(framework.InfrastructurePatchError(err)).To(SatisfyAny(
		ContainSubstring("failure domain"),
		ContainSubstring("still in use"),
	))
}

func managedCloudConfigYAML() string {
	cm, err := framework.GetConfigMap(suiteCtx, clients.Kube, framework.ManagedConfigNamespace, framework.ManagedConfigName)
	if err != nil {
		return ""
	}
	return cm.Data[framework.CloudConfigDataKey]
}

func ccmCloudConfigYAML() string {
	cm, err := framework.GetConfigMap(suiteCtx, clients.Kube, framework.CCMConfigNamespace, framework.CCMConfigName)
	if err != nil {
		return ""
	}
	return cm.Data[framework.CloudConfigDataKey]
}

func sourceCloudConfigYAML() (string, bool) {
	cm, err := framework.GetConfigMap(suiteCtx, clients.Kube, framework.SourceConfigNamespace, framework.SourceConfigName)
	if err != nil {
		return "", false
	}
	return cm.Data[framework.CloudConfigDataKey], true
}

func marshalPatchFromSpec(spec *configv1.InfrastructureSpec) []byte {
	payload := map[string]interface{}{"spec": spec}
	data, err := json.Marshal(payload)
	Expect(err).NotTo(HaveOccurred())
	return data
}

func duplicateFirstVCenterSpec(infra *configv1.Infrastructure) *configv1.InfrastructureSpec {
	spec := vsphere.CloneInfrastructureSpec(infra.Spec)
	Expect(spec.PlatformSpec.VSphere).NotTo(BeNil())
	Expect(spec.PlatformSpec.VSphere.VCenters).NotTo(BeEmpty())

	dup := vsphere.CloneVCenter(spec.PlatformSpec.VSphere.VCenters[0])
	dup.Datacenters = []string{"dup-dc"}
	spec.PlatformSpec.VSphere.VCenters = append(spec.PlatformSpec.VSphere.VCenters, dup)
	return &spec
}

func swapSecondVCenterServer(infra *configv1.Infrastructure, newServer string) *configv1.InfrastructureSpec {
	spec := vsphere.CloneInfrastructureSpec(infra.Spec)
	Expect(spec.PlatformSpec.VSphere).NotTo(BeNil())
	Expect(len(spec.PlatformSpec.VSphere.VCenters)).To(BeNumerically(">=", 2))
	spec.PlatformSpec.VSphere.VCenters[1].Server = newServer
	return &spec
}

func addAndRemoveVCenterSamePatch(infra *configv1.Infrastructure) *configv1.InfrastructureSpec {
	spec := vsphere.CloneInfrastructureSpec(infra.Spec)
	Expect(spec.PlatformSpec.VSphere).NotTo(BeNil())
	Expect(len(spec.PlatformSpec.VSphere.VCenters)).To(BeNumerically(">=", 2))

	newEntry := vsphere.CloneVCenter(spec.PlatformSpec.VSphere.VCenters[0])
	newEntry.Server = "new-vcenter.example.com"
	newEntry.Datacenters = []string{"new-dc"}

	kept := vsphere.CloneVCenter(spec.PlatformSpec.VSphere.VCenters[0])
	spec.PlatformSpec.VSphere.VCenters = append([]configv1.VSpherePlatformVCenterSpec{kept}, newEntry)
	return &spec
}

func emptyVCentersSpec(infra *configv1.Infrastructure) *configv1.InfrastructureSpec {
	spec := vsphere.CloneInfrastructureSpec(infra.Spec)
	Expect(spec.PlatformSpec.VSphere).NotTo(BeNil())
	spec.PlatformSpec.VSphere.VCenters = []configv1.VSpherePlatformVCenterSpec{}
	return &spec
}

func removeVCentersFieldSpec(infra *configv1.Infrastructure) *configv1.InfrastructureSpec {
	spec := vsphere.CloneInfrastructureSpec(infra.Spec)
	Expect(spec.PlatformSpec.VSphere).NotTo(BeNil())
	spec.PlatformSpec.VSphere.VCenters = nil
	return &spec
}

func addSecondVCenterSpec(infra *configv1.Infrastructure) *configv1.InfrastructureSpec {
	spec := vsphere.CloneInfrastructureSpec(infra.Spec)
	Expect(spec.PlatformSpec.VSphere).NotTo(BeNil())
	Expect(spec.PlatformSpec.VSphere.VCenters).NotTo(BeEmpty())

	extra := vsphere.CloneVCenter(spec.PlatformSpec.VSphere.VCenters[0])
	extra.Server = "vcenter2.example.com"
	extra.Datacenters = []string{"DC2"}
	spec.PlatformSpec.VSphere.VCenters = append(spec.PlatformSpec.VSphere.VCenters, extra)
	return &spec
}

func tooManyVCentersSpec(infra *configv1.Infrastructure) *configv1.InfrastructureSpec {
	spec := vsphere.CloneInfrastructureSpec(infra.Spec)
	Expect(spec.PlatformSpec.VSphere).NotTo(BeNil())
	base := spec.PlatformSpec.VSphere.VCenters
	for i := 0; len(spec.PlatformSpec.VSphere.VCenters) < 4; i++ {
		clone := vsphere.CloneVCenter(base[0])
		clone.Server = fmt.Sprintf("vcenter-extra-%d.example.com", i)
		spec.PlatformSpec.VSphere.VCenters = append(spec.PlatformSpec.VSphere.VCenters, clone)
	}
	return &spec
}

func fdReferencingRemovedVCenterSpec(infra *configv1.Infrastructure) *configv1.InfrastructureSpec {
	spec := vsphere.CloneInfrastructureSpec(infra.Spec)
	Expect(spec.PlatformSpec.VSphere).NotTo(BeNil())
	Expect(spec.PlatformSpec.VSphere.VCenters).NotTo(BeEmpty())

	removedServer := spec.PlatformSpec.VSphere.VCenters[len(spec.PlatformSpec.VSphere.VCenters)-1].Server
	spec.PlatformSpec.VSphere.VCenters = spec.PlatformSpec.VSphere.VCenters[:len(spec.PlatformSpec.VSphere.VCenters)-1]

	for i := range spec.PlatformSpec.VSphere.FailureDomains {
		if spec.PlatformSpec.VSphere.FailureDomains[i].Server == removedServer {
			return &spec
		}
	}
	Skip("no failure domain references the vCenter targeted for removal")
	return &spec
}
