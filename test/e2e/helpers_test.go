package e2e

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	"github.com/jcallen/testing-day2-vcenter/pkg/labconfig"
	"github.com/jcallen/testing-day2-vcenter/pkg/vsphere"
	configv1 "github.com/openshift/api/config/v1"
	machinev1 "github.com/openshift/api/machine/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	suiteCtx    context.Context
	clients     *framework.Clients
	infraBackup *configv1.Infrastructure
	gateEnabled bool
	labCfg      *labconfig.LabConfig
	labCfgPath     string
	csiTopoKeys    *framework.CSITopologyKeys
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

	for _, co := range []string{"cloud-controller-manager", "config-operator", "machine-api"} {
		err = framework.WaitForClusterOperatorAvailable(suiteCtx, clients.Config, co, framework.ShortTimeout)
		if err != nil {
			GinkgoWriter.Printf("warning: clusteroperator %q not available before suite: %v\n", co, err)
		}
	}

	GinkgoWriter.Printf("VSphereMultiVCenterDay2 enabled=%v vCenters=%d\n", gateEnabled, len(framework.GetVCenters(infra)))

	if keys, err := framework.DiscoverCSITopologyKeys(suiteCtx, clients.Kube); err == nil {
		csiTopoKeys = keys
		GinkgoWriter.Printf("CSI topology keys: region=%s zone=%s\n", keys.Region, keys.Zone)
	} else {
		GinkgoWriter.Printf("CSI topology keys not discovered: %v\n", err)
	}

	if cfg, path, err := labconfig.LoadFromEnv(); err == nil {
		labCfg = cfg
		labCfgPath = path
		GinkgoWriter.Printf("loaded lab config from %s (vCenter2=%s)\n", path, cfg.SecondVCenter.Server)
	}
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

func requireMultiVCenter() {
	infra := currentInfrastructure()
	if len(framework.GetVCenters(infra)) < 2 {
		Skip("cluster has fewer than 2 vCenters — run make apply-lab first")
	}
}

func requireLabConfig() *labconfig.LabConfig {
	if labCfg != nil {
		return labCfg
	}
	cfg, path, err := labconfig.LoadFromEnv()
	if err != nil {
		Skip(err.Error())
	}
	labCfg = cfg
	labCfgPath = path
	return labCfg
}

func currentInfrastructure() *configv1.Infrastructure {
	infra, err := framework.GetInfrastructure(suiteCtx, clients.Config)
	Expect(err).NotTo(HaveOccurred())
	return infra
}

func patchInfrastructureSpec(spec *configv1.InfrastructureSpec, dryRun bool) (*configv1.Infrastructure, error) {
	return framework.ReplaceInfrastructureSpec(suiteCtx, clients.Config, spec, dryRun)
}

func patchInfrastructureRaw(patch []byte, dryRun bool) (*configv1.Infrastructure, error) {
	return framework.PatchInfrastructure(suiteCtx, clients.Config, patch, dryRun)
}

func expectRawPatchRejected(patch []byte, messageFragment string) {
	_, err := patchInfrastructureRaw(patch, true)
	Expect(err).To(HaveOccurred(), "expected patch to be rejected")
	errMsg := framework.InfrastructurePatchError(err)
	Expect(errMsg).To(SatisfyAny(
		ContainSubstring(messageFragment),
		ContainSubstring("ValidatingAdmissionPolicy"),
	), "expected rejection containing %q or VAP denial, got: %s", messageFragment, errMsg)
}

func expectPatchRejected(spec *configv1.InfrastructureSpec, messageFragment string) {
	_, err := patchInfrastructureSpec(spec, true)
	Expect(err).To(HaveOccurred())
	errMsg := framework.InfrastructurePatchError(err)
	Expect(errMsg).To(SatisfyAny(
		ContainSubstring(messageFragment),
		ContainSubstring("ValidatingAdmissionPolicy"),
	), "expected xValidation rejection (%s) or VAP denial, got: %s", messageFragment, errMsg)
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


func expectFailureDomainRemovalDenied(infra *configv1.Infrastructure, region, zone string) {
	spec := specWithoutFailureDomain(infra, region, zone)
	_, err := patchInfrastructureSpec(spec, false)
	Expect(err).To(HaveOccurred(),
		"removing FD region=%s zone=%s should be denied by VAP", region, zone)
	Expect(framework.InfrastructurePatchError(err)).To(SatisfyAny(
		ContainSubstring("failure domain"),
		ContainSubstring("still in use"),
	))
}

func findCPMSBackedFailureDomain(infra *configv1.Infrastructure) (region, zone string, found bool) {
	fds := framework.GetFailureDomains(infra)
	fdByName := map[string]configv1.VSpherePlatformFailureDomainSpec{}
	for _, fd := range fds {
		fdByName[fd.Name] = fd
	}
	for _, cpms := range listCPMS() {
		for _, name := range framework.CPMSVSphereFailureDomainNames(&cpms) {
			if fd, ok := fdByName[name]; ok {
				return fd.Region, fd.Zone, true
			}
		}
	}
	return "", "", false
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
	return cm.Data[framework.SourceCloudConfigDataKey], true
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

func requireLabConfigWithFD() *labconfig.LabConfig {
	cfg := requireLabConfig()
	if cfg.FailureDomain == nil {
		Skip("lab config has no failureDomain — required for CSI storage tests")
	}
	return cfg
}

func createTestNamespaceWithCleanup(prefix string) string {
	ns, err := framework.CreateTestNamespace(suiteCtx, clients.Kube, prefix)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() {
		_ = framework.DeleteNamespace(suiteCtx, clients.Kube, ns, framework.DefaultTimeout)
	})
	return ns
}

func createTestNamespace(prefix string) string {
	ns, err := framework.CreateTestNamespace(suiteCtx, clients.Kube, prefix)
	Expect(err).NotTo(HaveOccurred())
	return ns
}

func requireCSITopologyKeys() *framework.CSITopologyKeys {
	if csiTopoKeys == nil {
		Skip("CSI topology keys not available — vCenter tag categories may not be configured")
	}
	return csiTopoKeys
}

func requireDefaultStorageClass() *storagev1.StorageClass {
	sc, err := framework.GetDefaultStorageClass(suiteCtx, clients.Kube)
	if err != nil {
		Skip(fmt.Sprintf("no default StorageClass: %v", err))
	}
	return sc
}

func ensureTemplateInSecondVC(lab *labconfig.LabConfig, infraID string) string {
	fd := lab.FailureDomain
	topo := fd.Topology

	templateName := topo.Template
	if templateName == "" {
		templateName = vsphere.TemplateNameForFailureDomain(infraID, fd.Name)
	}
	Expect(vsphere.ValidateTemplateName(templateName)).To(Succeed())

	password, err := lab.SecondVCenter.PasswordValue()
	Expect(err).NotTo(HaveOccurred(), "get second vCenter password")

	session, err := vsphere.NewSession(suiteCtx, vsphere.Params{
		Server:     lab.SecondVCenter.Server,
		Datacenter: topo.Datacenter,
		Username:   lab.SecondVCenter.Username,
		Password:   password,
		Insecure:   true,
	})
	Expect(err).NotTo(HaveOccurred(), "connect to second vCenter %s", lab.SecondVCenter.Server)
	defer session.Close(suiteCtx)

	Expect(vsphere.EnsureVMFolder(suiteCtx, session, infraID)).To(Succeed(),
		"ensure VM folder %s exists in %s", infraID, topo.Datacenter)

	path, found, err := vsphere.FindTemplateByName(suiteCtx, session, templateName)
	if found && err == nil {
		GinkgoWriter.Printf("template %s already exists at %s, skipping import\n", templateName, path)
		return templateName
	}
	if err != nil {
		Expect(err).NotTo(HaveOccurred(), "check template %s", templateName)
	}

	GinkgoWriter.Printf("template %s not found in %s, importing RHCOS OVA\n", templateName, topo.Datacenter)

	cm, err := clients.Kube.CoreV1().ConfigMaps(framework.MCONamespace).Get(
		suiteCtx, framework.CoreOSBootImagesCM, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "get %s/%s ConfigMap", framework.MCONamespace, framework.CoreOSBootImagesCM)

	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}
	ova, err := vsphere.ResolveRHCOSOVAFromConfigMap(cm, arch)
	Expect(err).NotTo(HaveOccurred(), "resolve RHCOS OVA for arch %s", arch)
	GinkgoWriter.Printf("resolved RHCOS OVA: %s (sha256=%s)\n", ova.Location, ova.Sha256)

	ovaPath, err := vsphere.DownloadOVAToDir(suiteCtx, ova.Location, ova.Sha256, "/tmp/ova-cache")
	Expect(err).NotTo(HaveOccurred(), "download RHCOS OVA")
	GinkgoWriter.Printf("OVA downloaded to %s\n", ovaPath)

	network := topo.Networks[0]
	vmFolder := fmt.Sprintf("/%s/vm/%s", topo.Datacenter, infraID)
	_, err = vsphere.ImportOVA(suiteCtx, vsphere.ImportOVAParams{
		Session:        session,
		OVAPath:        ovaPath,
		TemplateName:   templateName,
		ComputeCluster: topo.ComputeCluster,
		Datastore:      topo.Datastore,
		Network:        network,
		Folder:         vmFolder,
		ResourcePool:   topo.ResourcePool,
	})
	Expect(err).NotTo(HaveOccurred(), "import OVA as template %s", templateName)
	GinkgoWriter.Printf("template %s imported successfully\n", templateName)

	return templateName
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
