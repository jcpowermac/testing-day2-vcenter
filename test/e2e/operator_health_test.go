package e2e

import (
	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	configv1 "github.com/openshift/api/config/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Operator health", Label("readonly", "operator", "p0"), func() {
	operators := []string{
		"cloud-controller-manager",
		"config-operator",
		"machine-api",
	}

	for _, name := range operators {
		It("should keep ClusterOperator/"+name+" Available and not Degraded", func() {
			available, err := framework.GetClusterOperatorCondition(suiteCtx, clients.Config, name, configv1.OperatorAvailable)
			Expect(err).NotTo(HaveOccurred())
			Expect(available.Status).To(Equal(configv1.ConditionTrue))

			degraded, err := framework.GetClusterOperatorCondition(suiteCtx, clients.Config, name, configv1.OperatorDegraded)
			Expect(err).NotTo(HaveOccurred())
			Expect(degraded.Status).To(Equal(configv1.ConditionFalse))
		})
	}

	It("should not have CCM pods in a crash loop", Label("p1"), func() {
		pods, err := framework.ListPodsByLabel(suiteCtx, clients.Kube, framework.CCMConfigNamespace, "k8s-app=vsphere-cloud-controller-manager")
		if err != nil || len(pods) == 0 {
			Skip("CCM pods not found")
		}
		for _, pod := range pods {
			restarts := framework.PodRestartCount(&pod)
			Expect(restarts).To(BeNumerically("<", 5),
				"CCM pod %s has %d restarts, possible crash loop", pod.Name, restarts)
		}
	})

	It("should not have MAO pods in a crash loop", Label("p1"), func() {
		pods, err := framework.ListPodsByLabel(suiteCtx, clients.Kube, framework.MachineAPINamespace, "k8s-app=machine-api-operator")
		if err != nil || len(pods) == 0 {
			Skip("MAO pods not found")
		}
		for _, pod := range pods {
			restarts := framework.PodRestartCount(&pod)
			Expect(restarts).To(BeNumerically("<", 5),
				"MAO pod %s has %d restarts, possible crash loop", pod.Name, restarts)
		}
	})

	It("should not have CSI driver pods in a crash loop", Label("p1"), func() {
		pods, err := framework.ListPodsByLabel(suiteCtx, clients.Kube, "openshift-cluster-csi-drivers", "app=vmware-vsphere-csi-driver-controller")
		if err != nil || len(pods) == 0 {
			Skip("vSphere CSI driver pods not found")
		}
		for _, pod := range pods {
			restarts := framework.PodRestartCount(&pod)
			Expect(restarts).To(BeNumerically("<", 5),
				"CSI pod %s has %d restarts, possible crash loop", pod.Name, restarts)
		}
	})
})
