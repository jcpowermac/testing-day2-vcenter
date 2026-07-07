package e2e

import (
	"fmt"
	"strings"

	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// expectedInfraTopologyCategories returns the category names (without the
// "topology.csi.vmware.com/" prefix) that the operator hardcodes when the
// Infrastructure CR has more than one failure domain.
func expectedInfraTopologyCategories() []string {
	topoKeys := requireCSITopologyKeys()
	return []string{
		strings.TrimPrefix(topoKeys.Region, framework.CSITopologyKeyPrefix),
		strings.TrimPrefix(topoKeys.Zone, framework.CSITopologyKeyPrefix),
	}
}

func csiOperatorMetrics() (string, error) {
	return framework.ScrapeOperatorMetrics(suiteCtx, clients.Kube,
		framework.CSIDriverNamespace, "name=vmware-vsphere-csi-driver-operator")
}

var _ = Describe("CSI topology configuration", Label("csi-operator", "csi-topology", "readonly"), func() {

	BeforeEach(func() {
		requireGateEnabled()
		requireMultiVCenter()
	})

	It("TOPO-01: CSI config secret topology-categories matches discovered topology keys", Label("p1"), func() {
		expected := expectedInfraTopologyCategories()

		config, err := framework.GetCSIDriverConfig(suiteCtx, clients.Kube)
		Expect(err).NotTo(HaveOccurred())

		categories, ok := framework.CSIConfigTopologyCategories(config)
		Expect(ok).To(BeTrue(), "cloud.conf [Labels] section should have a topology-categories entry")
		Expect(categories).To(ConsistOf(expected))
		GinkgoWriter.Printf("TOPO-01: PASS — topology-categories=%v\n", categories)
	})

	It("TOPO-02: csi-provisioner has Topology feature gate and strict-topology args", Label("p1"), func() {
		args, err := framework.GetCSIProvisionerArgs(suiteCtx, clients.Kube)
		Expect(err).NotTo(HaveOccurred())

		Expect(args).To(ContainElement("--feature-gates=Topology=true"),
			"csi-provisioner should run with the Topology feature gate enabled")
		Expect(args).To(ContainElement(ContainSubstring("--strict-topology")),
			"csi-provisioner should run with --strict-topology")
		GinkgoWriter.Printf("TOPO-02: PASS — args=%v\n", args)
	})

	It("TOPO-03: internal feature states configmap has improved-volume-topology enabled", Label("p1"), func() {
		data, err := framework.GetFeatureConfigMapData(suiteCtx, clients.Kube)
		Expect(err).NotTo(HaveOccurred())

		Expect(data).To(HaveKeyWithValue("improved-volume-topology", "true"))
		GinkgoWriter.Println("TOPO-03: PASS")
	})

	It("TOPO-04: CSINode topology keys match discovered categories", Label("p1"), func() {
		topoKeys := requireCSITopologyKeys()

		keys, err := framework.GetCSINodeTopologyKeys(suiteCtx, clients.Kube)
		Expect(err).NotTo(HaveOccurred())

		Expect(keys).To(ConsistOf(topoKeys.Region, topoKeys.Zone))
		GinkgoWriter.Printf("TOPO-04: PASS — CSINode topology keys=%v\n", keys)
	})

	It("TOPO-05: topology tags metric reflects Infrastructure-sourced baseline", Label("p1"), func() {
		requireCSITopologyKeys()

		metrics, err := csiOperatorMetrics()
		if err != nil {
			Skip(fmt.Sprintf("cannot scrape operator metrics: %v", err))
		}

		infraVal, err := framework.ParseMetricValue(metrics, framework.TopologyTagsMetric,
			map[string]string{"source": framework.TopologyTagsSourceInfra})
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  %s{source=infrastructure}=%v\n", framework.TopologyTagsMetric, infraVal)
		Expect(infraVal).To(BeNumerically("==", 2),
			"infrastructure-sourced topology tags should be 2 (hardcoded region+zone)")

		ccdVal, err := framework.ParseMetricValue(metrics, framework.TopologyTagsMetric,
			map[string]string{"source": framework.TopologyTagsSourceCCD})
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  %s{source=clustercsidriver}=%v\n", framework.TopologyTagsMetric, ccdVal)
		Expect(ccdVal).To(BeNumerically("==", 0),
			"clustercsidriver-sourced topology tags should be 0 at baseline (ClusterCSIDriver field unset)")
		GinkgoWriter.Println("TOPO-05: PASS")
	})
})

var _ = Describe("CSI topology configuration precedence", Serial, Ordered, Label("csi-operator", "csi-topology", "mutating"), func() {

	BeforeEach(func() {
		requireGateEnabled()
		requireMultiVCenter()
	})

	It("TOPO-06: ClusterCSIDriver topologyCategories updates metric without overriding Infrastructure", Label("p1"), func() {
		expected := expectedInfraTopologyCategories()

		ccd, err := clients.Operator.OperatorV1().ClusterCSIDrivers().Get(suiteCtx, framework.ClusterCSIDriverName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		var originalCategories []string
		if ccd.Spec.DriverConfig.VSphere != nil {
			originalCategories = ccd.Spec.DriverConfig.VSphere.TopologyCategories
		}

		DeferCleanup(func() {
			GinkgoWriter.Println("  restoring ClusterCSIDriver topologyCategories")
			Expect(framework.SetClusterCSIDriverTopologyCategories(suiteCtx, clients.Operator, originalCategories)).To(Succeed())
		})

		GinkgoWriter.Println("  setting ClusterCSIDriver spec.driverConfig.vSphere.topologyCategories=[custom-zone]")
		Expect(framework.SetClusterCSIDriverTopologyCategories(suiteCtx, clients.Operator, []string{"custom-zone"})).To(Succeed())

		GinkgoWriter.Println("  waiting for clustercsidriver-sourced topology tags metric to become 1")
		Eventually(func() (float64, error) {
			metrics, mErr := csiOperatorMetrics()
			if mErr != nil {
				return 0, mErr
			}
			return framework.ParseMetricValue(metrics, framework.TopologyTagsMetric,
				map[string]string{"source": framework.TopologyTagsSourceCCD})
		}).WithTimeout(framework.DefaultTimeout).WithPolling(framework.DefaultPolling).Should(
			BeNumerically("==", 1), "clustercsidriver-sourced topology tags metric should become 1")

		GinkgoWriter.Println("  verifying CSI config secret still reflects Infrastructure categories (precedence)")
		config, err := framework.GetCSIDriverConfig(suiteCtx, clients.Kube)
		Expect(err).NotTo(HaveOccurred())
		categories, ok := framework.CSIConfigTopologyCategories(config)
		Expect(ok).To(BeTrue())
		Expect(categories).To(ConsistOf(expected),
			"Infrastructure topology categories should take precedence over ClusterCSIDriver's since >1 FDs exist")
		GinkgoWriter.Println("TOPO-06: PASS")
	})
})
