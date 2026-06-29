package e2e

import (
	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	"github.com/jcallen/testing-day2-vcenter/pkg/lab"
	"github.com/jcallen/testing-day2-vcenter/pkg/vsphere"
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
})
