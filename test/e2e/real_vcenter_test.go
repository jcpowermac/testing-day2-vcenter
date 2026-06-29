package e2e

import (
	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	"github.com/jcallen/testing-day2-vcenter/pkg/lab"
	"github.com/jcallen/testing-day2-vcenter/pkg/vsphere"
	configv1 "github.com/openshift/api/config/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Real vCenter Day 2", Label("real-vcenter", "p0", "mutating"), func() {
	BeforeEach(func() {
		requireGateEnabled()
		requireLabConfig()
	})

	It("should include configured vCenter in Infrastructure", func() {
		infra := currentInfrastructure()
		servers := vsphere.VCenterServers(framework.GetVCenters(infra))
		Expect(servers).To(ContainElement(labCfg.SecondVCenter.Server))
	})

	It("should reflect configured vCenter in managed cloud config", func() {
		infra := currentInfrastructure()
		cfg, err := vsphere.ParseCloudConfigYAML(managedCloudConfigYAML())
		Expect(err).NotTo(HaveOccurred())
		Expect(vsphere.AssertInfrastructureVCentersPresent(infra, cfg)).To(Succeed())
		Expect(vsphere.AssertNoStaleVCenters(infra, cfg)).To(Succeed())
	})

	It("should pass lab verification helper", func() {
		Expect(lab.Verify(suiteCtx, clients, labCfg)).To(Succeed())
	})

	It("should include failure domain when configured", func() {
		if labCfg.FailureDomain == nil {
			Skip("failureDomain not set in lab config")
		}
		infra := currentInfrastructure()
		fd := vsphere.FindFailureDomainByRegionZone(
			framework.GetFailureDomains(infra),
			labCfg.FailureDomain.Region,
			labCfg.FailureDomain.Zone,
		)
		Expect(fd).NotTo(BeNil())
		Expect(fd.Server).To(Equal(labCfg.SecondVCenter.Server))
	})

	It("should have credential secrets updated with second vCenter entries", func() {
		consumers := []struct {
			namespace string
			name      string
		}{
			{framework.VSphereCredsNamespace, framework.VSphereCredsSecret},
			{framework.MachineAPINamespace, "vsphere-cloud-credentials"},
			{framework.CCMConfigNamespace, "vsphere-cloud-credentials"},
			{"openshift-cluster-csi-drivers", "vmware-vsphere-cloud-credentials"},
		}

		for _, c := range consumers {
			secret, err := framework.GetSecret(suiteCtx, clients.Kube, c.namespace, c.name)
			if err != nil {
				GinkgoWriter.Printf("warning: secret %s/%s not found: %v\n", c.namespace, c.name, err)
				continue
			}
			Expect(framework.SecretHasKeyPrefix(secret, labCfg.SecondVCenter.Server)).To(BeTrue(),
				"secret %s/%s missing credentials for second vCenter %s", c.namespace, c.name, labCfg.SecondVCenter.Server)
		}
	})

	It("should keep all operators healthy after Day 2 add", func() {
		for _, co := range []string{"cloud-controller-manager", "config-operator", "machine-api"} {
			available, err := framework.GetClusterOperatorCondition(suiteCtx, clients.Config, co, configv1.OperatorAvailable)
			Expect(err).NotTo(HaveOccurred(), "ClusterOperator %s should exist", co)
			Expect(available.Status).To(Equal(configv1.ConditionTrue),
				"ClusterOperator %s should be Available after Day 2 add", co)
		}
	})

	It("should have CCM cloud config reflecting second vCenter", func() {
		raw := ccmCloudConfigYAML()
		if raw == "" {
			Skip("CCM cloud config not available")
		}
		cfg, err := vsphere.ParseCloudConfigYAML(raw)
		Expect(err).NotTo(HaveOccurred())
		servers := vsphere.VCenterServersFromConfig(cfg)
		Expect(servers).To(ContainElement(labCfg.SecondVCenter.Server),
			"CCM cloud config should reference second vCenter")
	})
})
