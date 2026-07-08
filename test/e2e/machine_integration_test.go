package e2e

import (
	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	"github.com/jcallen/testing-day2-vcenter/pkg/vsphere"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Machine integration", Label("readonly", "integration", "p0"), func() {
	It("should have all worker Machines in a healthy phase", func() {
		machines := listMachines()
		Expect(machines).NotTo(BeEmpty(), "cluster must have at least one Machine")

		for _, m := range machines {
			if m.DeletionTimestamp != nil {
				continue
			}
			phase := ""
			if m.Status.Phase != nil {
				phase = *m.Status.Phase
			}
			Expect(phase).To(SatisfyAny(
				Equal("Running"),
				Equal("Provisioned"),
			), "Machine %s has unexpected phase %q", m.Name, phase)
		}
	})

	It("should label every Machine with region and zone", Label("multi-vcenter"), func() {
		requireMultiVCenter()
		machines := listMachines()
		for _, m := range machines {
			if m.DeletionTimestamp != nil {
				continue
			}
			region, zone, ok := machineLabeledFailureDomain(m)
			Expect(ok).To(BeTrue(), "Machine %s missing region/zone labels", m.Name)
			Expect(region).NotTo(BeEmpty(), "Machine %s has empty region label", m.Name)
			Expect(zone).NotTo(BeEmpty(), "Machine %s has empty zone label", m.Name)
		}
	})

	It("should map every Machine to a valid Infrastructure failure domain", Label("multi-vcenter"), func() {
		requireMultiVCenter()
		infra := currentInfrastructure()
		fds := framework.GetFailureDomains(infra)
		if len(fds) == 0 {
			Skip("no failure domains configured")
		}

		machines := listMachines()
		for _, m := range machines {
			if m.DeletionTimestamp != nil {
				continue
			}
			region, zone, ok := machineLabeledFailureDomain(m)
			if !ok {
				continue
			}
			fd := vsphere.FindFailureDomainByRegionZone(fds, region, zone)
			Expect(fd).NotTo(BeNil(),
				"Machine %s has labels region=%s zone=%s but no matching Infrastructure failure domain", m.Name, region, zone)
		}
	})

	It("should have Machine providerSpec workspace matching Infrastructure topology", Label("multi-vcenter"), func() {
		requireMultiVCenter()
		infra := currentInfrastructure()
		fds := framework.GetFailureDomains(infra)
		if len(fds) == 0 {
			Skip("no failure domains configured")
		}

		machines := listMachines()
		for _, m := range machines {
			if m.DeletionTimestamp != nil {
				continue
			}
			providerSpec, err := framework.ExtractVSphereMachineProviderSpec(&m)
			if err != nil {
				GinkgoWriter.Printf("warning: cannot extract providerSpec for Machine %s: %v\n", m.Name, err)
				continue
			}
			if providerSpec.Workspace == nil {
				continue
			}

			region, zone, ok := machineLabeledFailureDomain(m)
			if !ok {
				continue
			}
			fd := vsphere.FindFailureDomainByRegionZone(fds, region, zone)
			if fd == nil {
				continue
			}

			Expect(providerSpec.Workspace.Datacenter).To(Equal(fd.Topology.Datacenter),
				"Machine %s workspace datacenter should match FD %s topology", m.Name, fd.Name)
		}
	})
})
