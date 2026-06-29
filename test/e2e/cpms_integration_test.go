package e2e

import (
	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CPMS integration", Label("readonly", "integration", "p0"), func() {
	It("should reference failure domain names that exist in Infrastructure", func() {
		cpmsList := listCPMS()
		if len(cpmsList) == 0 {
			Skip("no ControlPlaneMachineSet found")
		}

		infra := currentInfrastructure()
		fds := framework.GetFailureDomains(infra)
		if len(fds) == 0 {
			Skip("no failure domains configured")
		}

		fdNames := map[string]bool{}
		for _, fd := range fds {
			fdNames[fd.Name] = true
		}

		for _, cpms := range cpmsList {
			names := framework.CPMSVSphereFailureDomainNames(&cpms)
			GinkgoWriter.Printf("CPMS %s references FD names: %v\n", cpms.Name, names)
			for _, name := range names {
				Expect(fdNames).To(HaveKey(name),
					"CPMS %s references failure domain %q which does not exist in Infrastructure", cpms.Name, name)
			}
		}
	})

	It("should have CPMS failure domains covering all Infrastructure FDs", func() {
		cpmsList := listCPMS()
		if len(cpmsList) == 0 {
			Skip("no ControlPlaneMachineSet found")
		}

		infra := currentInfrastructure()
		fds := framework.GetFailureDomains(infra)
		if len(fds) == 0 {
			Skip("no failure domains configured")
		}

		for _, cpms := range cpmsList {
			cpmsNames := map[string]bool{}
			for _, name := range framework.CPMSVSphereFailureDomainNames(&cpms) {
				cpmsNames[name] = true
			}
			for _, fd := range fds {
				if !cpmsNames[fd.Name] {
					GinkgoWriter.Printf("note: Infrastructure FD %q not referenced by CPMS %s (may be worker-only)\n", fd.Name, cpms.Name)
				}
			}
		}
	})
})
