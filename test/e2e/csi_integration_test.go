package e2e

import (
	"fmt"

	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	"github.com/jcallen/testing-day2-vcenter/pkg/vsphere"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CSI driver integration", Label("readonly", "integration", "p1"), func() {
	It("should have CSI driver credential secret with entries for all vCenters", func() {
		infra := currentInfrastructure()
		vcenters := framework.GetVCenters(infra)
		Expect(vcenters).NotTo(BeEmpty())

		secret, err := framework.GetSecret(suiteCtx, clients.Kube, "openshift-cluster-csi-drivers", "vmware-vsphere-cloud-credentials")
		if err != nil {
			Skip(fmt.Sprintf("CSI credential secret not found: %v", err))
		}

		keys := framework.SecretDataKeys(secret)
		GinkgoWriter.Printf("CSI credential secret keys: %v\n", keys)

		for _, vc := range vcenters {
			Expect(framework.SecretHasKeyPrefix(secret, vc.Server)).To(BeTrue(),
				"CSI credential secret missing key for vCenter %s", vc.Server)
		}
	})

	It("should have CSI driver pods running", func() {
		pods, err := framework.ListPodsByLabel(suiteCtx, clients.Kube, "openshift-cluster-csi-drivers", "app=vmware-vsphere-csi-driver-controller")
		if err != nil || len(pods) == 0 {
			Skip("vSphere CSI driver controller pods not found")
		}
		Eventually(func(g Gomega) {
			pods, err := framework.ListPodsByLabel(suiteCtx, clients.Kube, "openshift-cluster-csi-drivers", "app=vmware-vsphere-csi-driver-controller")
			g.Expect(err).NotTo(HaveOccurred())
			for _, pod := range pods {
				g.Expect(string(pod.Status.Phase)).To(Equal("Running"),
					"CSI driver pod %s phase is %s, expected Running", pod.Name, pod.Status.Phase)
			}
		}).WithTimeout(framework.DefaultTimeout).WithPolling(framework.DefaultPolling).Should(Succeed())
	})

	It("should have managed cloud config listing all vCenter datacenters for CSI", func() {
		infra := currentInfrastructure()
		vcenters := framework.GetVCenters(infra)
		if len(vcenters) < 2 {
			Skip("single vCenter cluster — CSI multi-vCenter parity not applicable")
		}

		raw := managedCloudConfigYAML()
		if raw == "" {
			Skip("managed cloud config not available")
		}

		cfg, err := vsphere.ParseCloudConfigYAML(raw)
		Expect(err).NotTo(HaveOccurred())

		Expect(vsphere.AssertInfrastructureVCentersPresent(infra, cfg)).To(Succeed(),
			"managed cloud config should list all Infrastructure vCenters for CSI consumption")
	})
})
