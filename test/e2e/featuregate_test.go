package e2e

import (
	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Feature gate", Label("readonly", "p0", "operator"), func() {
	It("should expose VSphereMultiVCenterDay2 on FeatureGate/cluster", func() {
		enabled, version, found, err := framework.GetFeatureGateAttributes(suiteCtx, clients.Config, framework.VSphereMultiVCenterDay2Gate)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue(), "VSphereMultiVCenterDay2 must be listed in FeatureGate/cluster status")
		Expect(version).NotTo(BeEmpty())
		Expect(enabled).To(Equal(gateEnabled))
	})

	It("should report gate enabled state consistently", func() {
		enabled, err := framework.IsFeatureGateEnabled(suiteCtx, clients.Config, framework.VSphereMultiVCenterDay2Gate)
		Expect(err).NotTo(HaveOccurred())
		Expect(enabled).To(Equal(gateEnabled))
	})
})
