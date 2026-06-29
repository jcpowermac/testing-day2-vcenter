package e2e

import (
	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	"github.com/jcallen/testing-day2-vcenter/pkg/vsphere"
	configv1 "github.com/openshift/api/config/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Topology lifecycle", Label("mutating", "p0"), func() {
	Context("negative dry-run prechecks", func() {
		It("should deny removing a failure domain that still has Machines (N-SEQ-05 precheck)", func() {
			requireGateEnabled()
			infra := currentInfrastructure()
			region, zone, ok := findMachineBackedFailureDomain(infra)
			if !ok {
				Skip("no Machine-backed failure domain found")
			}
			expectFailureDomainRemovalDenied(infra, region, zone)
		})

		It("should deny removing a vCenter referenced by a failure domain (N-SEQ-04)", func() {
			requireGateEnabled()
			infra := currentInfrastructure()
			spec := fdReferencingRemovedVCenterSpec(infra)
			_, err := patchInfrastructureSpec(spec, true)
			if err == nil {
				Fail("CRD allows removing a vCenter still referenced by a failure domain — " +
					"no xValidation rule enforces FD.server must reference an existing vCenter entry (see SPLAT-2827)")
			}
			Expect(framework.InfrastructurePatchError(err)).To(SatisfyAny(
				ContainSubstring("failure domain"),
				ContainSubstring("vCenter"),
				ContainSubstring("ValidatingAdmissionPolicy"),
			))
		})
	})

	Context("active MachineSet VAP test", Label("p1"), func() {
		It("should deny removing an FD referenced by a 0-replica MachineSet", func() {
			requireGateEnabled()
			infra := currentInfrastructure()
			fds := framework.GetFailureDomains(infra)
			if len(fds) == 0 {
				Skip("no failure domains configured")
			}

			fd := fds[0]
			msName := "e2e-vap-probe-ms"
			ms := framework.BuildMachineSet(msName, framework.MachineAPINamespace, fd.Region, fd.Zone)

			created, err := framework.CreateMachineSet(suiteCtx, clients.Machine, ms)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				_ = framework.DeleteMachineSet(suiteCtx, clients.Machine, created.Name)
			})

			expectFailureDomainRemovalDenied(infra, fd.Region, fd.Zone)
		})
	})

	Context("mutating sequences", func() {
		It("should add and remove a temporary vCenter without leaving stale cloud config (#469)", func() {
			if labCfg != nil {
				Skip("lab config present; use make apply-lab / make test-real / make restore-lab for real vCenter testing")
			}
			requireGateEnabled()
			infra := currentInfrastructure()
			if len(framework.GetVCenters(infra)) >= 3 {
				Skip("cluster already has 3 vCenters")
			}

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				Expect(spec.PlatformSpec.VSphere).NotTo(BeNil())
				extra := vsphere.CloneVCenter(spec.PlatformSpec.VSphere.VCenters[0])
				extra.Server = "temp-vcenter-e2e.example.com"
				extra.Datacenters = []string{"TEMP-DC"}
				spec.PlatformSpec.VSphere.VCenters = append(spec.PlatformSpec.VSphere.VCenters, extra)

				_, err := patchInfrastructureSpec(spec, false)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() error {
					cfg, err := vsphere.ParseCloudConfigYAML(managedCloudConfigYAML())
					if err != nil {
						return err
					}
					return vsphere.AssertInfrastructureVCentersPresent(currentInfrastructure(), cfg)
				}).WithTimeout(framework.DefaultTimeout).WithPolling(framework.DefaultPolling).Should(Succeed())

				current := currentInfrastructure()
				removeSpec := vsphere.CloneInfrastructureSpec(current.Spec)
				removeSpec.PlatformSpec.VSphere.VCenters = vsphere.RemoveVCenterByServer(removeSpec.PlatformSpec.VSphere.VCenters, extra.Server)
				_, err = patchInfrastructureSpec(&removeSpec, false)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() error {
					cfg, err := vsphere.ParseCloudConfigYAML(managedCloudConfigYAML())
					if err != nil {
						return err
					}
					return vsphere.AssertNoStaleVCenters(currentInfrastructure(), cfg)
				}).WithTimeout(framework.DefaultTimeout).WithPolling(framework.DefaultPolling).Should(Succeed())
			})
		})
	})
})
