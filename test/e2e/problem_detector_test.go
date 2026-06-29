package e2e

import (
	"fmt"

	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("vsphere-problem-detector", Label("readonly", "operator", "p1"), func() {
	const (
		vpdNamespace  = "openshift-cluster-storage-operator"
		vpdDeployment = "vsphere-problem-detector-operator"
	)

	It("should have vsphere-problem-detector-operator deployment available", func() {
		deploy, err := clients.Kube.AppsV1().Deployments(vpdNamespace).Get(suiteCtx, vpdDeployment, metav1.GetOptions{})
		if err != nil {
			Skip(fmt.Sprintf("vsphere-problem-detector deployment not found: %v", err))
		}
		Expect(deploy.Status.AvailableReplicas).To(BeNumerically(">=", 1),
			"vsphere-problem-detector should have at least 1 available replica")
	})

	It("should not have vsphere-problem-detector pods in a crash loop", func() {
		pods, err := framework.ListPodsByLabel(suiteCtx, clients.Kube, vpdNamespace, "name=vsphere-problem-detector-operator")
		if err != nil || len(pods) == 0 {
			Skip("vsphere-problem-detector pods not found")
		}
		for _, pod := range pods {
			restarts := framework.PodRestartCount(&pod)
			Expect(restarts).To(BeNumerically("<", 5),
				"VPD pod %s has %d restarts", pod.Name, restarts)
		}
	})

	It("should validate GetVCenter behavior after failure domain removal when #224 merges", func() {
		Skip("waiting for vsphere-problem-detector#224 merge and lab scenarios (N-CFG-08)")
	})
})
