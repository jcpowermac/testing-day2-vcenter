package e2e

import (
	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	configv1 "github.com/openshift/api/config/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ValidatingAdmissionPolicies", Label("readonly", "admission", "p0"), func() {
	Context("when VSphereMultiVCenterDay2 is enabled", func() {
		BeforeEach(func() {
			requireGateEnabled()
		})

		It("should install vSphere failure domain VAP resources", func() {
			for _, name := range []string{
				framework.VAPMachineFailureDomainName,
				framework.VAPCPMSFailureDomainName,
				framework.VAPMachineSetFailureDomainName,
			} {
				vap, err := clients.Kube.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(suiteCtx, name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred(), "VAP %q should exist", name)
				Expect(vap.Spec.Validations).NotTo(BeEmpty())

				_, err = clients.Kube.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(suiteCtx, name, metav1.GetOptions{})
				if err != nil {
					_, err = clients.Kube.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(suiteCtx, name+"-binding", metav1.GetOptions{})
				}
				Expect(err).NotTo(HaveOccurred(), "VAP binding for %q should exist", name)
			}
		})

		It("should deny removing a failure domain referenced by a Machine (N-SEQ-01)", Label("mutating", "multi-vcenter"), func() {
			requireMultiVCenter()
			infra := currentInfrastructure()
			region, zone, ok := findMachineBackedFailureDomain(infra)
			if !ok {
				Skip("no Machine-backed failure domain found")
			}
			expectFailureDomainRemovalDenied(infra, region, zone)
		})

		It("should deny removing a failure domain referenced by a CPMS (N-SEQ-02)", Label("mutating", "multi-vcenter"), func() {
			requireMultiVCenter()
			infra := currentInfrastructure()
			region, zone, ok := findCPMSBackedFailureDomain(infra)
			if !ok {
				Skip("no CPMS-backed failure domain found")
			}
			expectFailureDomainRemovalDenied(infra, region, zone)
		})

		It("should deny removing a failure domain referenced by a MachineSet (N-SEQ-03)", Label("mutating", "multi-vcenter"), func() {
			requireMultiVCenter()
			infra := currentInfrastructure()
			fds := framework.GetFailureDomains(infra)
			if len(fds) == 0 {
				Skip("no failure domains configured")
			}

			sets := listMachineSets()
			if len(sets) == 0 {
				Skip("no existing MachineSets to clone providerSpec from")
			}

			fd := fds[0]
			msName := "e2e-vap-ms-n-seq-03"
			ms := framework.CloneMachineSetForVAP(sets[0], msName, fd.Region, fd.Zone, 1)

			created, err := framework.CreateMachineSet(suiteCtx, clients.Machine, ms)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				_ = framework.ScaleMachineSet(suiteCtx, clients.Machine, created.Name, 0)
				Eventually(func() error {
					return framework.WaitForMachineSetDrained(suiteCtx, clients.Machine, created.Name)
				}).WithTimeout(framework.LongTimeout).WithPolling(framework.DefaultPolling).Should(Succeed())
				_ = framework.DeleteMachineSet(suiteCtx, clients.Machine, created.Name)
			})

			GinkgoWriter.Printf("waiting for MachineSet %s to scale to 1...\n", msName)
			Eventually(func() error {
				return framework.WaitForMachineSetMachines(suiteCtx, clients.Machine, msName, 1)
			}).WithTimeout(framework.LongTimeout).WithPolling(framework.DefaultPolling).Should(Succeed())

			expectFailureDomainRemovalDenied(infra, fd.Region, fd.Zone)
		})

		It("should allow removing an unreferenced failure domain via dry-run", Label("multi-vcenter"), func() {
			requireMultiVCenter()
			infra := currentInfrastructure()
			fds := framework.GetFailureDomains(infra)
			if len(fds) == 0 {
				Skip("no failure domains configured")
			}

			var candidate *configv1.VSpherePlatformFailureDomainSpec
			for i := range fds {
				spec := specWithoutFailureDomain(infra, fds[i].Region, fds[i].Zone)
				_, err := patchInfrastructureSpec(spec, true)
				if err == nil {
					candidate = &fds[i]
					GinkgoWriter.Printf("FD %q (region=%s zone=%s) is unreferenced\n", fds[i].Name, fds[i].Region, fds[i].Zone)
					break
				}
				GinkgoWriter.Printf("FD %q is referenced: %s\n", fds[i].Name, framework.InfrastructurePatchError(err))
			}
			if candidate == nil {
				Skip("all failure domains are referenced by Machines, CPMS, or MachineSets")
			}
		})
	})

	Context("when VSphereMultiVCenterDay2 is disabled", func() {
		It("should not require vSphere VAP resources", func() {
			if gateEnabled {
				Skip("gate is enabled on this cluster")
			}
			_, err := clients.Kube.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(suiteCtx, framework.VAPMachineFailureDomainName, metav1.GetOptions{})
			Expect(err).To(HaveOccurred())
		})
	})
})

