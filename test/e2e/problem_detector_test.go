package e2e

import (
	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	configv1 "github.com/openshift/api/config/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("vsphere-problem-detector", Label("readonly", "operator", "p1"), func() {
	It("should keep ClusterOperator/vsphere-problem-detector Available when installed", func() {
		available, err := framework.GetClusterOperatorCondition(suiteCtx, clients.Config, "vsphere-problem-detector", configv1.OperatorAvailable)
		if err != nil {
			Skip("vsphere-problem-detector ClusterOperator not installed")
		}
		Expect(available.Status).To(Equal(configv1.ConditionTrue))
	})

	It("should validate GetVCenter behavior after failure domain removal when #224 merges", func() {
		Skip("waiting for vsphere-problem-detector#224 merge and lab scenarios (N-CFG-08)")
	})
})
