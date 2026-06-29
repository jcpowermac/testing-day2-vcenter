package e2e

import (
	"fmt"

	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("vSphere credentials propagation", Label("readonly", "integration", "p0"), func() {
	type credentialConsumer struct {
		namespace string
		name      string
	}

	consumers := []credentialConsumer{
		{namespace: framework.VSphereCredsNamespace, name: framework.VSphereCredsSecret},
		{namespace: framework.MachineAPINamespace, name: "vsphere-cloud-credentials"},
		{namespace: framework.CCMConfigNamespace, name: "vsphere-cloud-credentials"},
		{namespace: "openshift-cluster-csi-drivers", name: "vmware-vsphere-cloud-credentials"},
	}

	It("should have credential secrets for all Infrastructure vCenters", func() {
		infra := currentInfrastructure()
		vcenters := framework.GetVCenters(infra)
		Expect(vcenters).NotTo(BeEmpty(), "cluster must have at least one vCenter")

		for _, consumer := range consumers {
			secret, err := framework.GetSecret(suiteCtx, clients.Kube, consumer.namespace, consumer.name)
			if err != nil {
				GinkgoWriter.Printf("warning: secret %s/%s not found: %v\n", consumer.namespace, consumer.name, err)
				continue
			}

			for _, vc := range vcenters {
				hasKey := framework.SecretHasKeyPrefix(secret, vc.Server)
				Expect(hasKey).To(BeTrue(),
					fmt.Sprintf("secret %s/%s should have key prefixed with %q for vCenter credentials",
						consumer.namespace, consumer.name, vc.Server))
			}
		}
	})

	for _, consumer := range consumers {
		It(fmt.Sprintf("should have %s/%s with entries for every vCenter", consumer.namespace, consumer.name), func() {
			infra := currentInfrastructure()
			vcenters := framework.GetVCenters(infra)
			Expect(vcenters).NotTo(BeEmpty())

			secret, err := framework.GetSecret(suiteCtx, clients.Kube, consumer.namespace, consumer.name)
			if err != nil {
				Skip(fmt.Sprintf("secret %s/%s not found: %v", consumer.namespace, consumer.name, err))
			}

			keys := framework.SecretDataKeys(secret)
			GinkgoWriter.Printf("secret %s/%s keys: %v\n", consumer.namespace, consumer.name, keys)

			for _, vc := range vcenters {
				Expect(framework.SecretHasKeyPrefix(secret, vc.Server)).To(BeTrue(),
					fmt.Sprintf("missing credential key for vCenter %s in %s/%s", vc.Server, consumer.namespace, consumer.name))
			}
		})
	}
})
