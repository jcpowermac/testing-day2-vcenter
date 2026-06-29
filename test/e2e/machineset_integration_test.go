package e2e

import (
	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	"github.com/jcallen/testing-day2-vcenter/pkg/vsphere"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MachineSet integration", Label("readonly", "integration", "p0"), func() {
	It("should have providerSpec workspace matching an Infrastructure FD topology", func() {
		sets := listMachineSets()
		if len(sets) == 0 {
			Skip("no MachineSets found")
		}

		infra := currentInfrastructure()
		fds := framework.GetFailureDomains(infra)
		if len(fds) == 0 {
			Skip("no failure domains configured")
		}

		fdByDC := map[string]string{}
		for _, fd := range fds {
			fdByDC[fd.Topology.Datacenter] = fd.Name
		}

		for _, ms := range sets {
			providerSpec, err := framework.ExtractVSphereMachineSetProviderSpec(&ms)
			if err != nil {
				GinkgoWriter.Printf("warning: cannot extract providerSpec for MachineSet %s: %v\n", ms.Name, err)
				continue
			}
			if providerSpec.Workspace == nil {
				continue
			}

			dc := providerSpec.Workspace.Datacenter
			fdName, found := fdByDC[dc]
			Expect(found).To(BeTrue(),
				"MachineSet %s workspace datacenter %q does not match any Infrastructure FD topology", ms.Name, dc)

			fd := vsphere.FindFailureDomainByName(fds, fdName)
			if fd != nil {
				GinkgoWriter.Printf("MachineSet %s maps to FD %s (dc=%s, server=%s)\n", ms.Name, fd.Name, fd.Topology.Datacenter, fd.Server)
			}
		}
	})

	It("should have MachineSet template labels matching Infrastructure failure domains", func() {
		sets := listMachineSets()
		if len(sets) == 0 {
			Skip("no MachineSets found")
		}

		infra := currentInfrastructure()
		fds := framework.GetFailureDomains(infra)
		if len(fds) == 0 {
			Skip("no failure domains configured")
		}

		for _, ms := range sets {
			labels := ms.Spec.Template.Labels
			if labels == nil {
				continue
			}
			region := labels[framework.MachineRegionLabel]
			zone := labels[framework.MachineZoneLabel]
			if region == "" || zone == "" {
				continue
			}

			fd := vsphere.FindFailureDomainByRegionZone(fds, region, zone)
			Expect(fd).NotTo(BeNil(),
				"MachineSet %s template labels region=%s zone=%s don't match any Infrastructure FD", ms.Name, region, zone)
		}
	})
})
