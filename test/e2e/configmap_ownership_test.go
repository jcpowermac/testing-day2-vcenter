package e2e

import (
	"time"

	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ConfigMap ownership", Label("readonly", "config", "operator", "p0"), func() {
	It("should expose kube-cloud-config in openshift-config-managed", func() {
		cm, err := framework.GetConfigMap(suiteCtx, clients.Kube, framework.ManagedConfigNamespace, framework.ManagedConfigName)
		Expect(err).NotTo(HaveOccurred())
		Expect(cm.Data).To(HaveKey(framework.CloudConfigDataKey))
	})

	It("should keep managed ConfigMap stable over observation window (steady-state single writer)", func() {
		err := framework.WaitForConfigMapStable(suiteCtx, clients.Kube, framework.ManagedConfigNamespace, framework.ManagedConfigName, 60*time.Second)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should expose cloud-conf for CCM consumption", func() {
		cm, err := framework.GetConfigMap(suiteCtx, clients.Kube, framework.CCMConfigNamespace, framework.CCMConfigName)
		Expect(err).NotTo(HaveOccurred())
		Expect(cm.Data).To(HaveKey(framework.CloudConfigDataKey))
	})
})

var _ = Describe("ConfigMap recreation", Label("mutating", "config", "operator", "p1"), func() {
	It("should recreate kube-cloud-config if deleted when gate is enabled (N-OP-07)", func() {
		requireGateEnabled()

		snapshot, err := framework.SnapshotConfigMap(suiteCtx, clients.Kube, framework.ManagedConfigNamespace, framework.ManagedConfigName)
		Expect(err).NotTo(HaveOccurred())

		DeferCleanup(func() {
			_ = framework.RestoreConfigMapFromSnapshot(suiteCtx, clients.Kube, snapshot)
		})

		err = clients.Kube.CoreV1().ConfigMaps(framework.ManagedConfigNamespace).Delete(suiteCtx, framework.ManagedConfigName, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() error {
			_, err := framework.GetConfigMap(suiteCtx, clients.Kube, framework.ManagedConfigNamespace, framework.ManagedConfigName)
			return err
		}).WithTimeout(framework.DefaultTimeout).WithPolling(framework.DefaultPolling).Should(Succeed())
	})
})
