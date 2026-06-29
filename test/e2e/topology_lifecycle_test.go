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
			expectPatchRejected(fdReferencingRemovedVCenterSpec(infra), "Cannot add and remove vCenters at the same time")
		})
	})

	Context("mutating sequences", func() {
		It("should add and remove a temporary vCenter without leaving stale cloud config (#469)", func() {
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
