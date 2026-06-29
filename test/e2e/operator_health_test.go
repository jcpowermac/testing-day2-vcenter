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
		"cluster-config-operator",
		"machine-api",
	}

	for _, name := range operators {
		name := name
		It("should keep ClusterOperator/"+name+" Available and not Degraded", func() {
			available, err := framework.GetClusterOperatorCondition(suiteCtx, clients.Config, name, configv1.OperatorAvailable)
			Expect(err).NotTo(HaveOccurred())
			Expect(available.Status).To(Equal(configv1.ConditionTrue))

			degraded, err := framework.GetClusterOperatorCondition(suiteCtx, clients.Config, name, configv1.OperatorDegraded)
			Expect(err).NotTo(HaveOccurred())
			Expect(degraded.Status).To(Equal(configv1.ConditionFalse))
		})
	}
})
