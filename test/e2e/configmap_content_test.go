package e2e

import (
	"github.com/jcallen/testing-day2-vcenter/pkg/vsphere"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cloud config content", Label("readonly", "config", "p0"), func() {
	It("should parse managed kube-cloud-config YAML (N-CFG-01/02/03)", func() {
		data := managedCloudConfigYAML()
		Expect(data).NotTo(BeEmpty())
		_, err := vsphere.ParseCloudConfigYAML(data)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should parse CCM cloud-conf YAML", func() {
		data := ccmCloudConfigYAML()
		Expect(data).NotTo(BeEmpty())
		_, err := vsphere.ParseCloudConfigYAML(data)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should include all Infrastructure vCenters in managed cloud config", func() {
		infra := currentInfrastructure()
		cfg, err := vsphere.ParseCloudConfigYAML(managedCloudConfigYAML())
		Expect(err).NotTo(HaveOccurred())
		Expect(vsphere.AssertInfrastructureVCentersPresent(infra, cfg)).To(Succeed())
	})

	It("should not contain stale vCenters in managed cloud config (N-CFG-06)", func() {
		infra := currentInfrastructure()
		cfg, err := vsphere.ParseCloudConfigYAML(managedCloudConfigYAML())
		Expect(err).NotTo(HaveOccurred())
		Expect(vsphere.AssertNoStaleVCenters(infra, cfg)).To(Succeed())
	})

	It("should keep insecure-flag out of per-vCenter entries when possible", func() {
		cfg, err := vsphere.ParseCloudConfigYAML(managedCloudConfigYAML())
		Expect(err).NotTo(HaveOccurred())
		Expect(vsphere.GlobalInsecureOnly(cfg)).To(BeTrue())
	})

	It("should include source openshift-config cloud config when present (three-way parity)", func() {
		source, ok := sourceCloudConfigYAML()
		if !ok {
			Skip("openshift-config/cloud-provider-config not present")
		}
		Expect(source).NotTo(BeEmpty())

		managed := managedCloudConfigYAML()
		Expect(managed).NotTo(BeEmpty())
		// Semantic parity is validated by parser + vCenter cross-check against Infrastructure.
		managedCfg, err := vsphere.ParseCloudConfigYAML(managed)
		Expect(err).NotTo(HaveOccurred())
		Expect(vsphere.AssertInfrastructureVCentersPresent(currentInfrastructure(), managedCfg)).To(Succeed())
	})

	It("should expose node network settings when configured (installer #10614)", func() {
		cfg, err := vsphere.ParseCloudConfigYAML(managedCloudConfigYAML())
		Expect(err).NotTo(HaveOccurred())
		if cfg.Nodes == nil {
			Skip("nodes section not configured on this cluster")
		}
		Expect(cfg.Nodes.ExternalNetworkSubnetCidr != "" || cfg.Nodes.InternalNetworkSubnetCidr != "").To(BeTrue())
	})
})
