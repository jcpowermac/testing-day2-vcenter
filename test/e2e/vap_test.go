package e2e

import (
	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	"github.com/jcallen/testing-day2-vcenter/pkg/vsphere"
	configv1 "github.com/openshift/api/config/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
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

		It("should deny removing a failure domain referenced by a Machine (N-SEQ-01)", func() {
			infra := currentInfrastructure()
			region, zone, ok := findMachineBackedFailureDomain(infra)
			if !ok {
				Skip("no Machine-backed failure domain found")
			}
			expectFailureDomainRemovalDenied(infra, region, zone)
		})

		It("should allow removing an unreferenced failure domain via dry-run", func() {
			infra := currentInfrastructure()
			fds := framework.GetFailureDomains(infra)
			if len(fds) == 0 {
				Skip("no failure domains configured")
			}

			machineFDs := map[string]struct{}{}
			for _, machine := range listMachines() {
				r, z, ok := machineLabeledFailureDomain(machine)
				if ok {
					machineFDs[vsphere.FailureDomainKey(r, z)] = struct{}{}
				}
			}

			var candidate *configv1.VSpherePlatformFailureDomainSpec
			for i := range fds {
				key := vsphere.FailureDomainKey(fds[i].Region, fds[i].Zone)
				if _, inUse := machineFDs[key]; !inUse {
					candidate = &fds[i]
					break
				}
			}
			if candidate == nil {
				Skip("all failure domains are referenced by Machines")
			}

			spec := specWithoutFailureDomain(infra, candidate.Region, candidate.Zone)
			expectPatchAllowedDryRun(spec)
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

var _ = Describe("VAP dry-run probe", Label("readonly", "admission"), func() {
	It("records whether dry-run triggers VAP denials", func() {
		GinkgoWriter.Printf("vapDryRunWorks=%v\n", vapDryRunWorks)
		if !gateEnabled {
			Skip("feature gate disabled")
		}
		if !vapDryRunWorks {
			GinkgoWriter.Println("VAP denials will use real patch attempts that should be rejected without persisting state")
		}
		Expect(admissionregistrationv1.Deny).NotTo(BeEmpty())
	})
})
