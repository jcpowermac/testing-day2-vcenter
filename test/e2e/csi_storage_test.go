package e2e

import (
	"fmt"
	"strings"

	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	"github.com/jcallen/testing-day2-vcenter/pkg/labconfig"
	"github.com/jcallen/testing-day2-vcenter/pkg/vsphere"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Category A: CSI Driver Health and Config (readonly)
var _ = Describe("CSI driver health", Label("readonly", "storage", "p0"), func() {
	It("should have ClusterCSIDriver Available and not Degraded (N-CSI-01)", func() {
		csi, err := clients.Operator.OperatorV1().ClusterCSIDrivers().Get(suiteCtx, framework.ClusterCSIDriverName, metav1.GetOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				Skip("ClusterCSIDriver CRD not available on this cluster")
			}
			Expect(err).NotTo(HaveOccurred(), "get ClusterCSIDriver %s", framework.ClusterCSIDriverName)
		}

		var anyAvailable bool
		var degradedConditions []string
		for _, cond := range csi.Status.Conditions {
			if strings.HasSuffix(cond.Type, "Available") && cond.Status == "True" {
				anyAvailable = true
			}
			if strings.HasSuffix(cond.Type, "Degraded") && cond.Status == "True" {
				degradedConditions = append(degradedConditions, cond.Type)
			}
		}
		Expect(anyAvailable).To(BeTrue(), "at least one ClusterCSIDriver *Available condition should be True")
		Expect(degradedConditions).To(BeEmpty(), "no ClusterCSIDriver *Degraded condition should be True, got: %v", degradedConditions)
	})
})

var _ = Describe("CSI StorageClass topology", Label("readonly", "storage"), func() {
	It("should have a default StorageClass backed by vSphere CSI (N-CSI-10)", Label("p1"), func() {
		sc := requireDefaultStorageClass()
		Expect(sc.Provisioner).To(Equal(framework.ClusterCSIDriverName),
			"default StorageClass provisioner should be %s, got %s", framework.ClusterCSIDriverName, sc.Provisioner)
		Expect(framework.StorageClassIsWaitForFirstConsumer(sc)).To(BeTrue(),
			"default StorageClass should use WaitForFirstConsumer binding mode")
	})

	It("should have StorageClass topology plumbing connected to Infrastructure FDs (N-CSI-11)", Label("p2"), func() {
		sc := requireDefaultStorageClass()
		topoKeys := requireCSITopologyKeys()
		infra := currentInfrastructure()
		fds := framework.GetFailureDomains(infra)

		if len(sc.AllowedTopologies) > 0 {
			for _, term := range sc.AllowedTopologies {
				for _, expr := range term.MatchLabelExpressions {
					if expr.Key == topoKeys.Region || expr.Key == topoKeys.Zone {
						matched := false
						for _, fd := range fds {
							if (expr.Key == topoKeys.Region && containsString(expr.Values, fd.Region)) ||
								(expr.Key == topoKeys.Zone && containsString(expr.Values, fd.Zone)) {
								matched = true
								break
							}
						}
						Expect(matched).To(BeTrue(),
							"StorageClass allowedTopology key=%s values=%v should map to an Infrastructure FD", expr.Key, expr.Values)
					}
				}
			}
		} else {
			nodes, err := clients.Kube.CoreV1().Nodes().List(suiteCtx, metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			found := false
			for _, node := range nodes.Items {
				region := node.Labels[topoKeys.Region]
				zone := node.Labels[topoKeys.Zone]
				if region != "" && zone != "" {
					for _, fd := range fds {
						if fd.Region == region && fd.Zone == zone {
							found = true
							break
						}
					}
				}
				if found {
					break
				}
			}
			Expect(found).To(BeTrue(),
				"at least one node should have CSI topology labels matching an Infrastructure failure domain")
		}
	})
})

// Category D: CSI Health After Topology Change
var _ = Describe("CSI driver topology health", Label("storage"), func() {
	It("should have CSI driver pods healthy with current Infrastructure topology (N-CSI-02)", Label("real-vcenter", "mutating", "p1"), func() {
		controllerPods, err := framework.ListPodsByLabel(suiteCtx, clients.Kube, framework.CSIDriverNamespace, framework.CSIDriverControllerLabel)
		if err != nil || len(controllerPods) == 0 {
			Skip("vSphere CSI driver controller pods not found")
		}
		for i := range controllerPods {
			pod := &controllerPods[i]
			Expect(string(pod.Status.Phase)).To(Equal("Running"),
				"CSI controller pod %s phase is %s, expected Running", pod.Name, pod.Status.Phase)
			Expect(framework.PodRestartCount(pod)).To(BeNumerically("<", 5),
				"CSI controller pod %s has too many restarts", pod.Name)
		}

		nodePods, err := framework.ListPodsByLabel(suiteCtx, clients.Kube, framework.CSIDriverNamespace, framework.CSIDriverNodeLabel)
		if err == nil && len(nodePods) > 0 {
			for _, pod := range nodePods {
				Expect(string(pod.Status.Phase)).To(Equal("Running"),
					"CSI node pod %s phase is %s, expected Running", pod.Name, pod.Status.Phase)
			}
		}
	})

	It("should have CSI credential secret reflecting all vCenters (N-CSI-03)", Label("real-vcenter", "readonly", "p1"), func() {
		infra := currentInfrastructure()
		vcenters := framework.GetVCenters(infra)
		if len(vcenters) < 2 {
			Skip("single vCenter cluster — multi-vCenter CSI credential check not applicable")
		}

		requireGateEnabled()

		secret, err := framework.GetSecret(suiteCtx, clients.Kube, framework.CSIDriverNamespace, framework.CSICredentialSecretName)
		if err != nil {
			Skip(fmt.Sprintf("CSI credential secret not found: %v", err))
		}

		for _, vc := range vcenters {
			Expect(framework.SecretHasKeyPrefix(secret, vc.Server)).To(BeTrue(),
				"CSI credential secret missing key for vCenter %s", vc.Server)
		}
	})
})

// Category B: PV Provisioning in existing failure domain (mutating, real-vcenter)
var _ = Describe("CSI storage provisioning baseline", Label("real-vcenter", "mutating", "storage"), func() {
	It("should provision and bind a PVC in existing failure domain (N-CSI-04)", Label("p1"), func() {
		sc := requireDefaultStorageClass()
		_ = requireLabConfig()
		topoKeys := requireCSITopologyKeys()

		ns := createTestNamespaceWithCleanup(framework.TestNamespacePrefix)

		pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "csi-baseline-pvc", framework.TestPVCSize, sc.Name)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = framework.DeletePVC(suiteCtx, clients.Kube, ns, pvc.Name) })

		_, err = framework.CreateBusyboxPod(suiteCtx, clients.Kube, ns, "csi-baseline-pod", pvc.Name)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = framework.DeletePod(suiteCtx, clients.Kube, ns, "csi-baseline-pod") })

		boundPVC, err := framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.LongTimeout)
		Expect(err).NotTo(HaveOccurred(), "PVC should bind within timeout")

		pv, err := framework.GetPV(suiteCtx, clients.Kube, boundPVC.Spec.VolumeName)
		Expect(err).NotTo(HaveOccurred())

		pvRegion, pvZone, ok := framework.PVTopologyLabels(pv, topoKeys)
		if ok {
			infra := currentInfrastructure()
			fds := framework.GetFailureDomains(infra)
			fd := vsphere.FindFailureDomainByRegionZone(fds, pvRegion, pvZone)
			Expect(fd).NotTo(BeNil(),
				"PV topology region=%s zone=%s should match an Infrastructure failure domain", pvRegion, pvZone)
		}
	})

	It("should delete PV when PVC is deleted with reclaimPolicy Delete (N-CSI-07)", Label("p2"), func() {
		sc := requireDefaultStorageClass()
		_ = requireLabConfig()

		if sc.ReclaimPolicy != nil && *sc.ReclaimPolicy != corev1.PersistentVolumeReclaimDelete {
			Skip(fmt.Sprintf("default StorageClass reclaimPolicy is %s, not Delete", *sc.ReclaimPolicy))
		}

		ns := createTestNamespaceWithCleanup(framework.TestNamespacePrefix)

		pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "csi-cleanup-pvc", framework.TestPVCSize, sc.Name)
		Expect(err).NotTo(HaveOccurred())

		_, err = framework.CreateBusyboxPod(suiteCtx, clients.Kube, ns, "csi-cleanup-pod", pvc.Name)
		Expect(err).NotTo(HaveOccurred())

		boundPVC, err := framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.LongTimeout)
		Expect(err).NotTo(HaveOccurred())

		pvName := boundPVC.Spec.VolumeName
		GinkgoWriter.Printf("PV %s bound, deleting Pod and PVC to test cleanup\n", pvName)

		Expect(framework.DeletePod(suiteCtx, clients.Kube, ns, "csi-cleanup-pod")).To(Succeed())
		Expect(framework.DeletePVC(suiteCtx, clients.Kube, ns, pvc.Name)).To(Succeed())

		err = framework.WaitForPVDeleted(suiteCtx, clients.Kube, pvName, framework.LongTimeout)
		Expect(err).NotTo(HaveOccurred(),
			"PV %s should be deleted after PVC deletion with reclaimPolicy Delete", pvName)
	})
})

// Category B+C: Tests requiring a node in the second (lab) failure domain.
// Ordered so BeforeAll creates the MachineSet once and all specs share it.
var _ = Describe("CSI storage in new failure domain", Ordered, Label("real-vcenter", "mutating", "storage"), func() {
	var (
		lab       *labconfig.LabConfig
		topoKeys  *framework.CSITopologyKeys
		msName    string
		msCreated bool
	)

	BeforeAll(func() {
		lab = requireLabConfigWithFD()
		requireGateEnabled()
		topoKeys = requireCSITopologyKeys()

		infra := currentInfrastructure()
		fds := framework.GetFailureDomains(infra)
		fd := vsphere.FindFailureDomainByRegionZone(fds, lab.FailureDomain.Region, lab.FailureDomain.Zone)
		Expect(fd).NotTo(BeNil(),
			"Infrastructure should contain failure domain region=%s zone=%s — run make apply-lab first",
			lab.FailureDomain.Region, lab.FailureDomain.Zone)

		nodes, err := clients.Kube.CoreV1().Nodes().List(suiteCtx, metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, node := range nodes.Items {
			if node.Labels[topoKeys.Region] == lab.FailureDomain.Region &&
				node.Labels[topoKeys.Zone] == lab.FailureDomain.Zone {
				GinkgoWriter.Printf("found existing node %s in lab FD, skipping MachineSet creation\n", node.Name)
				return
			}
		}

		infraID := infra.Status.InfrastructureName
		Expect(infraID).NotTo(BeEmpty(), "infrastructure.status.infrastructureName must be set")

		templateName := ensureTemplateInSecondVC(lab, infraID)
		lab.FailureDomain.Topology.Template = templateName

		machineSets := listMachineSets()
		Expect(machineSets).NotTo(BeEmpty(), "need at least one MachineSet to clone")

		msName = fmt.Sprintf("csi-test-%s", lab.FailureDomain.Name)

		existing, err := clients.Machine.MachineV1beta1().MachineSets(framework.MachineAPINamespace).Get(
			suiteCtx, msName, metav1.GetOptions{})
		if err == nil {
			GinkgoWriter.Printf("MachineSet %s already exists (replicas=%d), reusing\n", msName, *existing.Spec.Replicas)
			if *existing.Spec.Replicas == 0 {
				Expect(framework.ScaleMachineSet(suiteCtx, clients.Machine, msName, 1)).To(Succeed(),
					"scale up existing MachineSet %s", msName)
			}
			msCreated = true
		} else {
			ms, cloneErr := framework.CloneMachineSetForFD(machineSets[0], msName, lab)
			Expect(cloneErr).NotTo(HaveOccurred(), "clone MachineSet for FD")

			_, createErr := framework.CreateMachineSet(suiteCtx, clients.Machine, ms)
			Expect(createErr).NotTo(HaveOccurred(), "create MachineSet %s", msName)
			msCreated = true
		}

		GinkgoWriter.Printf("waiting for Machine in MachineSet %s to be Running\n", msName)
		Eventually(func() error {
			return framework.WaitForMachineSetMachines(suiteCtx, clients.Machine, msName, 1)
		}, framework.LongTimeout, framework.DefaultPolling).Should(Succeed(),
			"Machine in new FD should reach Running/Provisioned")

		GinkgoWriter.Println("waiting for new node to get CSI topology labels")
		Eventually(func() bool {
			nodeList, err := clients.Kube.CoreV1().Nodes().List(suiteCtx, metav1.ListOptions{})
			if err != nil {
				return false
			}
			for _, node := range nodeList.Items {
				if node.Labels[topoKeys.Region] == lab.FailureDomain.Region &&
					node.Labels[topoKeys.Zone] == lab.FailureDomain.Zone {
					return true
				}
			}
			return false
		}, framework.LongTimeout, framework.DefaultPolling).Should(BeTrue(),
			"a node with CSI topology labels region=%s zone=%s should appear",
			lab.FailureDomain.Region, lab.FailureDomain.Zone)
	})

	AfterAll(func() {
		if !msCreated || msName == "" {
			return
		}
		GinkgoWriter.Printf("cleaning up MachineSet %s\n", msName)
		_ = framework.ScaleMachineSet(suiteCtx, clients.Machine, msName, 0)
		Eventually(func() error {
			return framework.WaitForMachineSetDrained(suiteCtx, clients.Machine, msName)
		}, framework.LongTimeout, framework.DefaultPolling).Should(Succeed())
		_ = framework.DeleteMachineSet(suiteCtx, clients.Machine, msName)
	})

	It("should provision a PV in new failure domain with correct topology labels (N-CSI-05)", Label("p1"), func() {
		sc := requireDefaultStorageClass()
		ns := createTestNamespaceWithCleanup(framework.TestNamespacePrefix)

		pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "csi-newfd-pvc", framework.TestPVCSize, sc.Name)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = framework.DeletePVC(suiteCtx, clients.Kube, ns, pvc.Name) })

		nodeSelector := map[string]string{
			topoKeys.Region: lab.FailureDomain.Region,
			topoKeys.Zone:   lab.FailureDomain.Zone,
		}
		_, err = framework.CreateBusyboxPodWithNodeSelector(suiteCtx, clients.Kube, ns, "csi-newfd-pod", pvc.Name, nodeSelector)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = framework.DeletePod(suiteCtx, clients.Kube, ns, "csi-newfd-pod") })

		boundPVC, err := framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.LongTimeout)
		Expect(err).NotTo(HaveOccurred(), "PVC should bind in new failure domain within timeout")

		pv, err := framework.GetPV(suiteCtx, clients.Kube, boundPVC.Spec.VolumeName)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			_ = framework.DeletePV(suiteCtx, clients.Kube, pv.Name)
			_ = framework.WaitForPVDeleted(suiteCtx, clients.Kube, pv.Name, framework.LongTimeout)
		})

		pvRegion, pvZone, ok := framework.PVTopologyLabels(pv, topoKeys)
		Expect(ok).To(BeTrue(), "PV should have CSI topology labels in nodeAffinity")
		Expect(pvRegion).To(Equal(lab.FailureDomain.Region),
			"PV region should match lab FD region")
		Expect(pvZone).To(Equal(lab.FailureDomain.Zone),
			"PV zone should match lab FD zone")

		infra := currentInfrastructure()
		fds := framework.GetFailureDomains(infra)
		matchedFD := vsphere.FindFailureDomainByRegionZone(fds, pvRegion, pvZone)
		Expect(matchedFD).NotTo(BeNil())
		Expect(matchedFD.Server).To(Equal(lab.SecondVCenter.Server),
			"PV should be provisioned on the second vCenter %s", lab.SecondVCenter.Server)
	})

	It("should provision PVC with explicit topology constraint in correct FD (N-CSI-06)", Label("p2"), func() {
		defaultSC := requireDefaultStorageClass()
		ns := createTestNamespaceWithCleanup(framework.TestNamespacePrefix)

		topologyTerms := []corev1.TopologySelectorTerm{
			{
				MatchLabelExpressions: []corev1.TopologySelectorLabelRequirement{
					{Key: topoKeys.Region, Values: []string{lab.FailureDomain.Region}},
					{Key: topoKeys.Zone, Values: []string{lab.FailureDomain.Zone}},
				},
			},
		}
		scName := "csi-test-topo-" + lab.FailureDomain.Name
		_, err := framework.CloneStorageClassWithTopology(suiteCtx, clients.Kube, defaultSC, scName, topologyTerms)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = framework.DeleteStorageClass(suiteCtx, clients.Kube, scName) })

		pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "csi-topo-pvc", framework.TestPVCSize, scName)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = framework.DeletePVC(suiteCtx, clients.Kube, ns, pvc.Name) })

		_, err = framework.CreateBusyboxPod(suiteCtx, clients.Kube, ns, "csi-topo-pod", pvc.Name)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = framework.DeletePod(suiteCtx, clients.Kube, ns, "csi-topo-pod") })

		boundPVC, err := framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.LongTimeout)
		Expect(err).NotTo(HaveOccurred())

		pv, err := framework.GetPV(suiteCtx, clients.Kube, boundPVC.Spec.VolumeName)
		Expect(err).NotTo(HaveOccurred())

		pvRegion, pvZone, ok := framework.PVTopologyLabels(pv, topoKeys)
		Expect(ok).To(BeTrue())
		Expect(pvRegion).To(Equal(lab.FailureDomain.Region))
		Expect(pvZone).To(Equal(lab.FailureDomain.Zone))
	})

	It("should probe FD removal behavior when PVs exist in that FD (N-CSI-08)", Label("p1"), func() {
		sc := requireDefaultStorageClass()
		ns := createTestNamespaceWithCleanup(framework.TestNamespacePrefix)

		pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "csi-guard-pvc", framework.TestPVCSize, sc.Name)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = framework.DeletePVC(suiteCtx, clients.Kube, ns, pvc.Name) })

		nodeSelector := map[string]string{
			topoKeys.Region: lab.FailureDomain.Region,
			topoKeys.Zone:   lab.FailureDomain.Zone,
		}
		_, err = framework.CreateBusyboxPodWithNodeSelector(suiteCtx, clients.Kube, ns, "csi-guard-pod", pvc.Name, nodeSelector)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = framework.DeletePod(suiteCtx, clients.Kube, ns, "csi-guard-pod") })

		boundPVC, err := framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.LongTimeout)
		Expect(err).NotTo(HaveOccurred(), "PVC must bind before probing FD removal")

		pv, err := framework.GetPV(suiteCtx, clients.Kube, boundPVC.Spec.VolumeName)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			_ = framework.DeletePV(suiteCtx, clients.Kube, pv.Name)
			_ = framework.WaitForPVDeleted(suiteCtx, clients.Kube, pv.Name, framework.LongTimeout)
		})

		GinkgoWriter.Printf("PV %s bound in FD region=%s zone=%s, probing FD removal via dry-run\n",
			pv.Name, lab.FailureDomain.Region, lab.FailureDomain.Zone)

		infra := currentInfrastructure()
		spec := specWithoutFailureDomain(infra, lab.FailureDomain.Region, lab.FailureDomain.Zone)
		_, err = patchInfrastructureSpec(spec, true)

		if err != nil {
			errMsg := framework.InfrastructurePatchError(err)
			GinkgoWriter.Printf("FD removal denied: %s\n", errMsg)

			if strings.Contains(errMsg, framework.VAPMachineFailureDomainName) {
				GinkgoWriter.Println("Denial from Machine VAP — not PV-specific protection")
			} else if strings.Contains(errMsg, framework.VAPCPMSFailureDomainName) {
				GinkgoWriter.Println("Denial from CPMS VAP — not PV-specific protection")
			} else if strings.Contains(errMsg, framework.VAPMachineSetFailureDomainName) {
				GinkgoWriter.Println("Denial from MachineSet VAP — not PV-specific protection")
			} else if strings.Contains(errMsg, "volume") || strings.Contains(errMsg, "PersistentVolume") {
				GinkgoWriter.Println("PV-specific VAP detected — FD removal denied due to PVs")
			} else {
				GinkgoWriter.Printf("Unknown denial reason: %s\n", errMsg)
			}
		} else {
			GinkgoWriter.Println("PRODUCT GAP: FD removal allowed via dry-run despite PVs present. " +
				"No VAP/xValidation rule protects PV-backed failure domains.")
		}
	})

	It("should confirm vCenter removal blocked by existing xValidation, PV presence irrelevant (N-CSI-09)", Label("p1"), func() {
		sc := requireDefaultStorageClass()
		ns := createTestNamespaceWithCleanup(framework.TestNamespacePrefix)

		infra := currentInfrastructure()
		vcenters := framework.GetVCenters(infra)
		if len(vcenters) < 2 {
			Skip("need at least 2 vCenters for vCenter removal test")
		}

		pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "csi-vc-guard-pvc", framework.TestPVCSize, sc.Name)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = framework.DeletePVC(suiteCtx, clients.Kube, ns, pvc.Name) })

		nodeSelector := map[string]string{
			topoKeys.Region: lab.FailureDomain.Region,
			topoKeys.Zone:   lab.FailureDomain.Zone,
		}
		_, err = framework.CreateBusyboxPodWithNodeSelector(suiteCtx, clients.Kube, ns, "csi-vc-guard-pod", pvc.Name, nodeSelector)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = framework.DeletePod(suiteCtx, clients.Kube, ns, "csi-vc-guard-pod") })

		_, err = framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.LongTimeout)
		Expect(err).NotTo(HaveOccurred(), "PVC must bind before probing vCenter removal")

		GinkgoWriter.Printf("probing removal of vCenter %s (hosts FD with PVs) via dry-run\n", lab.SecondVCenter.Server)

		spec := specWithoutVCenter(infra, lab.SecondVCenter.Server)
		_, err = patchInfrastructureSpec(spec, true)
		Expect(err).To(HaveOccurred(), "vCenter removal should be denied — FDs still reference it")

		errMsg := framework.InfrastructurePatchError(err)
		GinkgoWriter.Printf("vCenter removal denied as expected: %s\n", errMsg)
		GinkgoWriter.Println("PV presence irrelevant — denial is based on FD reference to vCenter, not PV protection")
	})
})

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
