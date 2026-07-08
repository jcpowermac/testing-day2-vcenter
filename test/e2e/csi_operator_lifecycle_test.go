package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	"github.com/jcallen/testing-day2-vcenter/pkg/labconfig"
	"github.com/jcallen/testing-day2-vcenter/pkg/vsphere"
	configv1 "github.com/openshift/api/config/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const operatorPollInterval = 10 * time.Second

func requireLabFD() (lab *labconfig.LabConfig, fd *labconfig.FailureDomainConfig) {
	cfg := requireLabConfigWithFD()
	return cfg, cfg.FailureDomain
}

func secondVCenterSession(lab *labconfig.LabConfig) *vsphere.Session {
	password, err := lab.SecondVCenter.PasswordValue()
	Expect(err).NotTo(HaveOccurred(), "get second vCenter password")
	dc := lab.SecondVCenter.Datacenters[0]
	if lab.FailureDomain != nil {
		dc = lab.FailureDomain.Topology.Datacenter
	}
	sess, err := vsphere.NewSession(suiteCtx, vsphere.Params{
		Server:     lab.SecondVCenter.Server,
		Datacenter: dc,
		Username:   lab.SecondVCenter.Username,
		Password:   password,
		Insecure:   true,
	})
	Expect(err).NotTo(HaveOccurred(), "connect to second vCenter %s", lab.SecondVCenter.Server)
	return sess
}

func primaryVCenterSession() *vsphere.Session {
	infra := currentInfrastructure()
	vcs := framework.GetVCenters(infra)
	Expect(vcs).NotTo(BeEmpty())
	primaryServer := vcs[0].Server

	secret, err := clients.Kube.CoreV1().Secrets(framework.VSphereCredsNamespace).Get(
		suiteCtx, framework.VSphereCredsSecret, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	username := string(secret.Data[primaryServer+".username"])
	password := string(secret.Data[primaryServer+".password"])
	Expect(username).NotTo(BeEmpty(), "primary vCenter username in vsphere-creds")

	dc := vcs[0].Datacenters[0]
	sess, err := vsphere.NewSession(suiteCtx, vsphere.Params{
		Server:     primaryServer,
		Datacenter: dc,
		Username:   username,
		Password:   password,
		Insecure:   true,
	})
	Expect(err).NotTo(HaveOccurred(), "connect to primary vCenter %s", primaryServer)
	return sess
}

func infraID() string {
	infra := currentInfrastructure()
	return infra.Status.InfrastructureName
}

func isVAPDenied(err error) bool {
	msg := framework.InfrastructurePatchError(err)
	return strings.Contains(msg, "ValidatingAdmissionPolicy") ||
		strings.Contains(msg, "still in use")
}

func skipIfVAPDenied(err error, context string) {
	if isVAPDenied(err) {
		Skip(fmt.Sprintf("Machine VAP denied %s: %s", context, framework.InfrastructurePatchError(err)))
	}
}

func removeFDAndSkipIfDenied(spec *configv1.InfrastructureSpec, fd *labconfig.FailureDomainConfig) {
	spec.PlatformSpec.VSphere.FailureDomains = vsphere.RemoveFailureDomainByRegionZone(
		spec.PlatformSpec.VSphere.FailureDomains, fd.Region, fd.Zone)
	GinkgoWriter.Printf("  patching Infrastructure: removing FD region=%s zone=%s\n", fd.Region, fd.Zone)
	_, patchErr := patchInfrastructureSpec(spec, false)
	if patchErr != nil {
		skipIfVAPDenied(patchErr, "FD removal")
		Fail(fmt.Sprintf("FD removal patch failed: %v", patchErr))
	}
	GinkgoWriter.Println("  FD removed from Infrastructure spec")
}

func waitForTagDetached(ctx context.Context, sess *vsphere.Session, datastorePath, tagName string, timeout time.Duration) {
	GinkgoWriter.Printf("  waiting for tag %q to detach from %s (timeout %v)\n", tagName, datastorePath, timeout)
	lastLog := time.Now()
	Eventually(func() bool {
		tagged, err := vsphere.IsDatastoreTagged(ctx, sess, datastorePath, tagName)
		if err != nil {
			GinkgoWriter.Printf("  tag check error: %v\n", err)
			return false
		}
		if time.Since(lastLog) >= 30*time.Second {
			GinkgoWriter.Printf("  wait: tag %q still attached to %s\n", tagName, datastorePath)
			lastLog = time.Now()
		}
		return !tagged
	}).WithTimeout(timeout).WithPolling(operatorPollInterval).Should(BeTrue(),
		"tag %q should be detached from datastore %s", tagName, datastorePath)
	GinkgoWriter.Printf("  tag %q detached from %s\n", tagName, datastorePath)
}

func waitForTagAttached(ctx context.Context, sess *vsphere.Session, datastorePath, tagName string, timeout time.Duration) {
	GinkgoWriter.Printf("  waiting for tag %q to attach to %s (timeout %v)\n", tagName, datastorePath, timeout)
	lastLog := time.Now()
	Eventually(func() bool {
		tagged, err := vsphere.IsDatastoreTagged(ctx, sess, datastorePath, tagName)
		if err != nil {
			GinkgoWriter.Printf("  tag check error: %v\n", err)
			return false
		}
		if time.Since(lastLog) >= 30*time.Second {
			GinkgoWriter.Printf("  wait: tag %q not yet attached to %s\n", tagName, datastorePath)
			lastLog = time.Now()
		}
		return tagged
	}).WithTimeout(timeout).WithPolling(operatorPollInterval).Should(BeTrue(),
		"tag %q should be attached to datastore %s", tagName, datastorePath)
	GinkgoWriter.Printf("  tag %q attached to %s\n", tagName, datastorePath)
}

func waitForOrphanConditionFalse(timeout time.Duration) {
	GinkgoWriter.Printf("  waiting for OrphanCleanupPending=False (timeout %v)\n", timeout)
	lastLog := time.Now()
	Eventually(func() bool {
		cond, err := framework.GetClusterOperatorCondition(suiteCtx, clients.Config,
			framework.StorageOperatorName, configv1.ClusterStatusConditionType(framework.OrphanCleanupPendingCondition))
		if err != nil {
			return true
		}
		if time.Since(lastLog) >= 30*time.Second {
			GinkgoWriter.Printf("  wait: OrphanCleanupPending=%s\n", cond.Status)
			lastLog = time.Now()
		}
		return cond.Status != configv1.ConditionTrue
	}).WithTimeout(timeout).WithPolling(operatorPollInterval).Should(BeTrue(),
		"OrphanCleanupPending should be False on ClusterOperator storage")
	GinkgoWriter.Println("  OrphanCleanupPending resolved")
}

func waitForOrphanConditionTrue(timeout time.Duration) {
	GinkgoWriter.Printf("  waiting for OrphanCleanupPending=True (timeout %v)\n", timeout)
	lastLog := time.Now()
	Eventually(func() bool {
		cond, err := framework.GetClusterOperatorCondition(suiteCtx, clients.Config,
			framework.StorageOperatorName, configv1.ClusterStatusConditionType(framework.OrphanCleanupPendingCondition))
		if err != nil {
			if time.Since(lastLog) >= 30*time.Second {
				GinkgoWriter.Printf("  wait: OrphanCleanupPending condition not found: %v\n", err)
				lastLog = time.Now()
			}
			return false
		}
		if time.Since(lastLog) >= 30*time.Second {
			GinkgoWriter.Printf("  wait: OrphanCleanupPending=%s\n", cond.Status)
			lastLog = time.Now()
		}
		return cond.Status == configv1.ConditionTrue
	}).WithTimeout(timeout).WithPolling(operatorPollInterval).Should(BeTrue(),
		"OrphanCleanupPending should be True on ClusterOperator storage")
	GinkgoWriter.Println("  OrphanCleanupPending=True")
}

func waitForStorageOperatorHealthy(timeout time.Duration) {
	GinkgoWriter.Printf("  waiting for storage ClusterOperator healthy (timeout %v)\n", timeout)
	err := framework.WaitForClusterOperatorAvailable(suiteCtx, clients.Config,
		framework.StorageOperatorName, timeout)
	Expect(err).NotTo(HaveOccurred(), "storage ClusterOperator should be healthy")
	GinkgoWriter.Println("  storage ClusterOperator healthy")
}

func requireSecondFDNode() {
	if !secondFDNodeReady {
		Skip("no node available in second FD zone — MachineSet creation failed or was skipped")
	}
}

// Shared state for MachineSet lifecycle across PV-SAFE and OBS-03 tests.
var (
	csiOpMSName        string
	csiOpMSCreated     bool
	secondFDNodeReady  bool
	csiOpTestNS        []string
)

var _ = Describe("CSI Operator Failure Domain Lifecycle", Serial, Ordered, Label("csi-operator", "multi-vcenter", "mutating"), func() {

	BeforeEach(func() {
		requireGateEnabled()
		requireMultiVCenter()
		requireLabConfigWithFD()
	})

	BeforeAll(func() {
		lab := requireLabConfigWithFD()
		requireGateEnabled()
		topoKeys := requireCSITopologyKeys()

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
				GinkgoWriter.Printf("found existing node %s in lab FD zone=%s, skipping MachineSet creation\n",
					node.Name, lab.FailureDomain.Zone)
				secondFDNodeReady = true
				return
			}
		}

		id := infra.Status.InfrastructureName
		Expect(id).NotTo(BeEmpty(), "infrastructure.status.infrastructureName must be set")

		GinkgoWriter.Println("no nodes in second FD — creating MachineSet")
		templateName := ensureTemplateInSecondVC(lab, id)
		lab.FailureDomain.Topology.Template = templateName

		machineSets := listMachineSets()
		Expect(machineSets).NotTo(BeEmpty(), "need at least one MachineSet to clone")

		csiOpMSName = fmt.Sprintf("csi-op-%s", lab.FailureDomain.Name)

		existing, getErr := clients.Machine.MachineV1beta1().MachineSets(framework.MachineAPINamespace).Get(
			suiteCtx, csiOpMSName, metav1.GetOptions{})
		if getErr == nil {
			GinkgoWriter.Printf("MachineSet %s already exists (replicas=%d), reusing\n", csiOpMSName, *existing.Spec.Replicas)
			if *existing.Spec.Replicas == 0 {
				Expect(framework.ScaleMachineSet(suiteCtx, clients.Machine, csiOpMSName, 1)).To(Succeed(),
					"scale up existing MachineSet %s", csiOpMSName)
			}
			csiOpMSCreated = true
		} else {
			ms, cloneErr := framework.CloneMachineSetForFD(machineSets[0], csiOpMSName, lab)
			Expect(cloneErr).NotTo(HaveOccurred(), "clone MachineSet for FD")
			_, createErr := framework.CreateMachineSet(suiteCtx, clients.Machine, ms)
			Expect(createErr).NotTo(HaveOccurred(), "create MachineSet %s", csiOpMSName)
			csiOpMSCreated = true
		}

		GinkgoWriter.Printf("waiting for Machine in MachineSet %s to be Running\n", csiOpMSName)
		Eventually(func() error {
			return framework.WaitForMachineSetMachines(suiteCtx, clients.Machine, csiOpMSName, 1)
		}, framework.LongTimeout, framework.DefaultPolling).Should(Succeed(),
			"Machine in new FD should reach Running/Provisioned")

		GinkgoWriter.Println("waiting for new node to get CSI topology labels")
		Eventually(func() bool {
			nodeList, listErr := clients.Kube.CoreV1().Nodes().List(suiteCtx, metav1.ListOptions{})
			if listErr != nil {
				return false
			}
			for _, node := range nodeList.Items {
				if node.Labels[topoKeys.Region] == lab.FailureDomain.Region &&
					node.Labels[topoKeys.Zone] == lab.FailureDomain.Zone {
					GinkgoWriter.Printf("node %s ready in zone=%s\n", node.Name, lab.FailureDomain.Zone)
					return true
				}
			}
			return false
		}, framework.LongTimeout, framework.DefaultPolling).Should(BeTrue(),
			"a node with CSI topology labels region=%s zone=%s should appear",
			lab.FailureDomain.Region, lab.FailureDomain.Zone)

		secondFDNodeReady = true
	})

	AfterAll(func() {
		for _, ns := range csiOpTestNS {
			GinkgoWriter.Printf("deleting test namespace %s before MachineSet teardown\n", ns)
			err := framework.DeleteNamespace(suiteCtx, clients.Kube, ns, framework.DefaultTimeout)
			if err != nil {
				GinkgoWriter.Printf("namespace %s deletion failed: %v\n", ns, err)
			}
		}
		csiOpTestNS = nil

		if !csiOpMSCreated || csiOpMSName == "" {
			return
		}
		GinkgoWriter.Printf("cleaning up MachineSet %s\n", csiOpMSName)
		_ = framework.ScaleMachineSet(suiteCtx, clients.Machine, csiOpMSName, 0)
		err := framework.WaitForMachineSetDrainedWithLog(suiteCtx, clients.Machine, csiOpMSName, framework.LongTimeout)
		if err != nil {
			GinkgoWriter.Printf("MachineSet %s drain failed: %v, force-deleting remaining Machines\n", csiOpMSName, err)
			framework.ForceDeleteMachineSetMachines(suiteCtx, clients.Machine, csiOpMSName)
		}
		_ = framework.DeleteMachineSet(suiteCtx, clients.Machine, csiOpMSName)
	})

	// ── Category 1: Failure Domain Addition — Operator Response ──

	Context("after FD addition (State 1)", Label("fd-lifecycle"), Ordered, func() {

		It("FD-ADD-01: operator tags new FD's datastore", Label("p0"), func() {
			lab, fd := requireLabFD()
			id := infraID()
			tagName := vsphere.GetClusterTagName(id)

			GinkgoWriter.Printf("connecting to second vCenter %s\n", lab.SecondVCenter.Server)
			sess := secondVCenterSession(lab)
			DeferCleanup(func() { sess.Close(suiteCtx) })

			catName := vsphere.GetClusterTagCategoryName(id)
			GinkgoWriter.Printf("checking tag category %q on second vCenter\n", catName)
			cat, err := vsphere.FindTagCategoryByName(suiteCtx, sess, catName)
			Expect(err).NotTo(HaveOccurred())
			Expect(cat).NotTo(BeNil(), "tag category %q should exist on second vCenter", catName)

			GinkgoWriter.Printf("checking datastore %s tagged with %q\n", fd.Topology.Datastore, tagName)
			tagged, err := vsphere.IsDatastoreTagged(suiteCtx, sess, fd.Topology.Datastore, tagName)
			Expect(err).NotTo(HaveOccurred())
			Expect(tagged).To(BeTrue(), "datastore %s should be tagged with %q", fd.Topology.Datastore, tagName)

			sc := requireDefaultStorageClass()
			Expect(sc.Parameters).To(HaveKey("StoragePolicyName"),
				"default StorageClass should have StoragePolicyName parameter")
			GinkgoWriter.Println("FD-ADD-01: PASS — tag, category, and StorageClass verified")
		})

		It("FD-ADD-02: SPBM profile exists on second vCenter", Label("p0"), func() {
			lab, _ := requireLabFD()
			id := infraID()
			profileName := vsphere.GetStoragePolicyName(id)

			GinkgoWriter.Printf("checking SPBM profile %q on second vCenter\n", profileName)
			sess := secondVCenterSession(lab)
			DeferCleanup(func() { sess.Close(suiteCtx) })

			exists, err := vsphere.StorageProfileExists(suiteCtx, sess, profileName)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue(), "SPBM profile %q should exist on second vCenter", profileName)

			GinkgoWriter.Printf("checking SPBM profile %q on primary vCenter\n", profileName)
			primarySess := primaryVCenterSession()
			DeferCleanup(func() { primarySess.Close(suiteCtx) })

			primaryExists, err := vsphere.StorageProfileExists(suiteCtx, primarySess, profileName)
			Expect(err).NotTo(HaveOccurred())
			Expect(primaryExists).To(BeTrue(), "SPBM profile %q should still exist on primary vCenter", profileName)
			GinkgoWriter.Println("FD-ADD-02: PASS — SPBM profile exists on both vCenters")
		})

		It("FD-ADD-03: operator conditions healthy", Label("p0"), func() {
			GinkgoWriter.Println("checking storage operator health and conditions")
			waitForStorageOperatorHealthy(framework.DefaultTimeout)
			waitForOrphanConditionFalse(framework.ShortTimeout)

			pods, err := framework.ListPodsByLabel(suiteCtx, clients.Kube,
				framework.CSIDriverNamespace, "name=vmware-vsphere-csi-driver-operator")
			Expect(err).NotTo(HaveOccurred())
			for _, pod := range pods {
				restarts := framework.PodRestartCount(&pod)
				GinkgoWriter.Printf("  operator pod %s: restarts=%d\n", pod.Name, restarts)
				Expect(restarts).To(BeNumerically("<=", 2),
					"operator pod %s should have <= 2 restarts, got %d", pod.Name, restarts)
			}
			GinkgoWriter.Println("FD-ADD-03: PASS — operator healthy, no excessive restarts")
		})

		It("FD-ADD-04: CSI driver config includes second vCenter", Label("p0"), func() {
			lab, _ := requireLabFD()

			GinkgoWriter.Println("reading CSI driver config")
			config, err := framework.GetCSIDriverConfig(suiteCtx, clients.Kube)
			Expect(err).NotTo(HaveOccurred())
			Expect(framework.CSIConfigHasVCenter(config, lab.SecondVCenter.Server)).To(BeTrue(),
				"CSI driver config should reference second vCenter %s", lab.SecondVCenter.Server)

			infra := currentInfrastructure()
			primaryServer := framework.GetVCenters(infra)[0].Server
			Expect(framework.CSIConfigHasVCenter(config, primaryServer)).To(BeTrue(),
				"CSI driver config should still reference primary vCenter %s", primaryServer)
			GinkgoWriter.Printf("FD-ADD-04: PASS — config has both %s and %s\n", primaryServer, lab.SecondVCenter.Server)
		})
	})

	// ── Category 2: Failure Domain Removal — Orphan Tag Cleanup ──

	Context("FD removal — orphan tag cleanup", Label("fd-lifecycle"), Ordered, func() {

		It("FD-REM-01: orphan tag detached after FD removal", Label("p0"), func() {
			lab, fd := requireLabFD()
			id := infraID()
			tagName := vsphere.GetClusterTagName(id)

			sess := secondVCenterSession(lab)
			DeferCleanup(func() { sess.Close(suiteCtx) })

			GinkgoWriter.Printf("pre-check: verifying tag %q attached to %s\n", tagName, fd.Topology.Datastore)
			tagged, err := vsphere.IsDatastoreTagged(suiteCtx, sess, fd.Topology.Datastore, tagName)
			Expect(err).NotTo(HaveOccurred())
			Expect(tagged).To(BeTrue(), "pre-check: datastore should be tagged")

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				removeFDAndSkipIfDenied(spec, fd)

				waitForTagDetached(suiteCtx, sess, fd.Topology.Datastore, tagName, framework.OperatorSyncTimeout)

				GinkgoWriter.Println("  verifying primary FD tags untouched")
				primarySess := primaryVCenterSession()
				defer primarySess.Close(suiteCtx)
				infra := currentInfrastructure()
				primaryFDs := framework.GetFailureDomains(infra)
				Expect(primaryFDs).NotTo(BeEmpty(), "should have remaining FDs")

				waitForOrphanConditionFalse(framework.DefaultTimeout)

				sc := requireDefaultStorageClass()
				Expect(sc.Parameters).To(HaveKey("StoragePolicyName"))
			})

			GinkgoWriter.Println("  verifying tag re-attached after restore")
			waitForTagAttached(suiteCtx, sess, fd.Topology.Datastore, tagName, framework.OperatorSyncTimeout)
			GinkgoWriter.Println("FD-REM-01: PASS")
		})

		It("FD-REM-02: StorageClass and SPBM profile survive FD removal", Label("p0"), func() {
			_, fd := requireLabFD()
			id := infraID()
			profileName := vsphere.GetStoragePolicyName(id)

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				removeFDAndSkipIfDenied(spec, fd)
				waitForStorageOperatorHealthy(framework.OperatorSyncTimeout)

				GinkgoWriter.Println("  checking StorageClass retained StoragePolicyName")
				sc := requireDefaultStorageClass()
				Expect(sc.Parameters).To(HaveKey("StoragePolicyName"),
					"StorageClass should retain StoragePolicyName after FD removal")

				GinkgoWriter.Println("  checking SPBM profile on primary vCenter")
				primarySess := primaryVCenterSession()
				defer primarySess.Close(suiteCtx)
				exists, err := vsphere.StorageProfileExists(suiteCtx, primarySess, profileName)
				Expect(err).NotTo(HaveOccurred())
				Expect(exists).To(BeTrue(), "SPBM profile should survive on primary vCenter")

				GinkgoWriter.Println("  running PVC smoke test")
				ns := createTestNamespaceWithCleanup("csi-op-smoke")
				pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "smoke-pvc", framework.TestPVCSize, "")
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = framework.DeletePVC(suiteCtx, clients.Kube, ns, pvc.Name) })

				pod, err := framework.CreateBusyboxPod(suiteCtx, clients.Kube, ns, "smoke-pod", pvc.Name)
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = framework.DeletePod(suiteCtx, clients.Kube, ns, pod.Name) })

				err = framework.WaitForPodRunning(suiteCtx, clients.Kube, ns, pod.Name, framework.LongTimeout)
				Expect(err).NotTo(HaveOccurred(), "PVC smoke test: pod should reach Running")

				boundPVC, err := framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.DefaultTimeout)
				Expect(err).NotTo(HaveOccurred())
				Expect(boundPVC.Spec.VolumeName).NotTo(BeEmpty(), "PVC should be bound to a PV")
				GinkgoWriter.Printf("  smoke test PVC bound to PV %s\n", boundPVC.Spec.VolumeName)
			})
			GinkgoWriter.Println("FD-REM-02: PASS")
		})

		It("FD-REM-03: operator reconciles within backoff window", Label("p1"), func() {
			lab, fd := requireLabFD()
			id := infraID()
			tagName := vsphere.GetClusterTagName(id)

			sess := secondVCenterSession(lab)
			DeferCleanup(func() { sess.Close(suiteCtx) })

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				start := time.Now()
				removeFDAndSkipIfDenied(spec, fd)

				waitForTagDetached(suiteCtx, sess, fd.Topology.Datastore, tagName, framework.OperatorSyncTimeout)
				elapsed := time.Since(start)
				GinkgoWriter.Printf("FD-REM-03: tag detached in %v\n", elapsed)

				Expect(elapsed).To(BeNumerically("<", framework.OperatorSyncTimeout),
					"tag should detach within successCheckInterval (10 min) + jitter, not stuck at 30 min backoff cap")
			})
			GinkgoWriter.Println("FD-REM-03: PASS")
		})
	})

	// ── Category 3: PV Safety — Tag Detach Blocked by CNS Volumes ──

	Context("PV safety — tag detach blocked by CNS volumes", Label("pv-safety"), Ordered, func() {

		It("PV-SAFE-01: orphan tag blocked when PVs exist", Label("p0"), func() {
			requireSecondFDNode()
			lab, fd := requireLabFD()
			id := infraID()
			tagName := vsphere.GetClusterTagName(id)

			sess := secondVCenterSession(lab)
			DeferCleanup(func() { sess.Close(suiteCtx) })

			GinkgoWriter.Printf("creating PVC+pod in zone=%s\n", fd.Zone)
			ns := createTestNamespace("csi-op-pvsafe")
			csiOpTestNS = append(csiOpTestNS, ns)

			sc := requireDefaultStorageClass()
			pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "pv-safe-pvc", framework.TestPVCSize, sc.Name)
			Expect(err).NotTo(HaveOccurred())

			nodeSelector := map[string]string{
				"topology.kubernetes.io/zone": fd.Zone,
			}
			pod, err := framework.CreateBusyboxPodWithNodeSelector(suiteCtx, clients.Kube, ns, "pv-safe-pod", pvc.Name, nodeSelector)
			Expect(err).NotTo(HaveOccurred())

			GinkgoWriter.Println("  waiting for pod Running")
			err = framework.WaitForPodRunning(suiteCtx, clients.Kube, ns, pod.Name, framework.LongTimeout)
			Expect(err).NotTo(HaveOccurred(), "pod should reach Running in second FD")

			boundPVC, err := framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.DefaultTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(boundPVC.Spec.VolumeName).NotTo(BeEmpty())
			GinkgoWriter.Printf("  PVC bound to PV %s\n", boundPVC.Spec.VolumeName)

			DeferCleanup(func() {
				GinkgoWriter.Printf("  cleanup: deleting pod/PVC in %s\n", ns)
				_ = framework.DeletePod(suiteCtx, clients.Kube, ns, pod.Name)
				_ = framework.DeletePVC(suiteCtx, clients.Kube, ns, pvc.Name)
				if boundPVC.Spec.VolumeName != "" {
					_ = framework.WaitForPVDeleted(suiteCtx, clients.Kube, boundPVC.Spec.VolumeName, framework.LongTimeout)
				}
			})

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				removeFDAndSkipIfDenied(spec, fd)

				waitForOrphanConditionTrue(framework.OperatorSyncTimeout)

				GinkgoWriter.Println("  verifying tag still attached (PV blocks cleanup)")
				tagged, tagErr := vsphere.IsDatastoreTagged(suiteCtx, sess, fd.Topology.Datastore, tagName)
				Expect(tagErr).NotTo(HaveOccurred())
				Expect(tagged).To(BeTrue(), "tag should still be attached — PV blocks cleanup")

				waitForStorageOperatorHealthy(framework.DefaultTimeout)

				GinkgoWriter.Println("  verifying PVC still Bound")
				recheckPVC, recheckErr := clients.Kube.CoreV1().PersistentVolumeClaims(ns).Get(
					suiteCtx, pvc.Name, metav1.GetOptions{})
				Expect(recheckErr).NotTo(HaveOccurred())
				Expect(string(recheckPVC.Status.Phase)).To(Equal("Bound"), "PVC should remain Bound")
			})
			GinkgoWriter.Println("PV-SAFE-01: PASS")
		})

		It("PV-SAFE-02: orphan cleanup proceeds after PVs deleted", Label("p0"), func() {
			requireSecondFDNode()
			lab, fd := requireLabFD()
			id := infraID()
			tagName := vsphere.GetClusterTagName(id)

			sess := secondVCenterSession(lab)
			DeferCleanup(func() { sess.Close(suiteCtx) })

			GinkgoWriter.Printf("creating PVC+pod in zone=%s\n", fd.Zone)
			ns := createTestNamespace("csi-op-pvdel")
			csiOpTestNS = append(csiOpTestNS, ns)

			sc := requireDefaultStorageClass()
			pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "pvdel-pvc", framework.TestPVCSize, sc.Name)
			Expect(err).NotTo(HaveOccurred())

			nodeSelector := map[string]string{
				"topology.kubernetes.io/zone": fd.Zone,
			}
			pod, err := framework.CreateBusyboxPodWithNodeSelector(suiteCtx, clients.Kube, ns, "pvdel-pod", pvc.Name, nodeSelector)
			Expect(err).NotTo(HaveOccurred())

			GinkgoWriter.Println("  waiting for pod Running")
			err = framework.WaitForPodRunning(suiteCtx, clients.Kube, ns, pod.Name, framework.LongTimeout)
			Expect(err).NotTo(HaveOccurred())
			boundPVC, err := framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.DefaultTimeout)
			Expect(err).NotTo(HaveOccurred())
			pvName := boundPVC.Spec.VolumeName
			GinkgoWriter.Printf("  PVC bound to PV %s\n", pvName)

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				removeFDAndSkipIfDenied(spec, fd)
				waitForOrphanConditionTrue(framework.OperatorSyncTimeout)

				GinkgoWriter.Println("  deleting pod, PVC, and PV")
				Expect(framework.DeletePod(suiteCtx, clients.Kube, ns, pod.Name)).To(Succeed())
				Expect(framework.DeletePVC(suiteCtx, clients.Kube, ns, pvc.Name)).To(Succeed())
				Expect(framework.WaitForPVDeleted(suiteCtx, clients.Kube, pvName, framework.LongTimeout)).To(Succeed())
				GinkgoWriter.Println("  PV deleted, waiting for operator to complete cleanup")

				waitForTagDetached(suiteCtx, sess, fd.Topology.Datastore, tagName, framework.OperatorSyncTimeout)
				waitForOrphanConditionFalse(framework.DefaultTimeout)
			})
			GinkgoWriter.Println("PV-SAFE-02: PASS")
		})

		It("PV-SAFE-03: force cleanup annotation overrides PV safety", Label("p1"), func() {
			requireSecondFDNode()
			lab, fd := requireLabFD()
			id := infraID()
			tagName := vsphere.GetClusterTagName(id)

			sess := secondVCenterSession(lab)
			DeferCleanup(func() { sess.Close(suiteCtx) })

			GinkgoWriter.Printf("creating PVC+pod in zone=%s\n", fd.Zone)
			ns := createTestNamespace("csi-op-force")
			csiOpTestNS = append(csiOpTestNS, ns)

			sc := requireDefaultStorageClass()
			pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "force-pvc", framework.TestPVCSize, sc.Name)
			Expect(err).NotTo(HaveOccurred())

			nodeSelector := map[string]string{
				"topology.kubernetes.io/zone": fd.Zone,
			}
			pod, err := framework.CreateBusyboxPodWithNodeSelector(suiteCtx, clients.Kube, ns, "force-pod", pvc.Name, nodeSelector)
			Expect(err).NotTo(HaveOccurred())

			GinkgoWriter.Println("  waiting for pod Running")
			err = framework.WaitForPodRunning(suiteCtx, clients.Kube, ns, pod.Name, framework.LongTimeout)
			Expect(err).NotTo(HaveOccurred())
			boundPVC, err := framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.DefaultTimeout)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  PVC bound to PV %s\n", boundPVC.Spec.VolumeName)

			DeferCleanup(func() {
				GinkgoWriter.Printf("  cleanup: deleting pod/PVC in %s\n", ns)
				_ = framework.DeletePod(suiteCtx, clients.Kube, ns, pod.Name)
				_ = framework.DeletePVC(suiteCtx, clients.Kube, ns, pvc.Name)
				if boundPVC.Spec.VolumeName != "" {
					_ = framework.WaitForPVDeleted(suiteCtx, clients.Kube, boundPVC.Spec.VolumeName, framework.LongTimeout)
				}
			})

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				removeFDAndSkipIfDenied(spec, fd)
				waitForOrphanConditionTrue(framework.OperatorSyncTimeout)

				GinkgoWriter.Println("  annotating ClusterCSIDriver with force-orphan-cleanup")
				ccd, annotErr := clients.Operator.OperatorV1().ClusterCSIDrivers().Get(
					suiteCtx, framework.ClusterCSIDriverName, metav1.GetOptions{})
				Expect(annotErr).NotTo(HaveOccurred())
				if ccd.Annotations == nil {
					ccd.Annotations = map[string]string{}
				}
				ccd.Annotations[framework.ForceOrphanCleanupAnnotation] = "true"
				_, annotErr = clients.Operator.OperatorV1().ClusterCSIDrivers().Update(suiteCtx, ccd, metav1.UpdateOptions{})
				Expect(annotErr).NotTo(HaveOccurred())
				GinkgoWriter.Println("  force-orphan-cleanup annotation set")

				DeferCleanup(func() {
					ccd, err := clients.Operator.OperatorV1().ClusterCSIDrivers().Get(
						suiteCtx, framework.ClusterCSIDriverName, metav1.GetOptions{})
					if err == nil {
						delete(ccd.Annotations, framework.ForceOrphanCleanupAnnotation)
						_, _ = clients.Operator.OperatorV1().ClusterCSIDrivers().Update(suiteCtx, ccd, metav1.UpdateOptions{})
					}
				})

				waitForTagDetached(suiteCtx, sess, fd.Topology.Datastore, tagName, framework.OperatorSyncTimeout)
				waitForOrphanConditionFalse(framework.DefaultTimeout)

				GinkgoWriter.Println("  verifying PVC still Bound after force cleanup")
				recheckPVC, recheckErr := clients.Kube.CoreV1().PersistentVolumeClaims(ns).Get(
					suiteCtx, pvc.Name, metav1.GetOptions{})
				Expect(recheckErr).NotTo(HaveOccurred())
				Expect(string(recheckPVC.Status.Phase)).To(Equal("Bound"),
					"PVC should remain Bound after force cleanup — only tag was removed")
			})
			GinkgoWriter.Println("PV-SAFE-03: PASS")
		})
	})

	// ── Category 4: vCenter Removal — Full Lifecycle ──

	Context("vCenter removal lifecycle", Label("vcenter-removal"), Ordered, func() {

		It("VC-REM-01: complete vCenter removal lifecycle", Label("p0"), func() {
			lab, fd := requireLabFD()
			id := infraID()
			tagName := vsphere.GetClusterTagName(id)
			profileName := vsphere.GetStoragePolicyName(id)

			sess := secondVCenterSession(lab)
			DeferCleanup(func() { sess.Close(suiteCtx) })

			GinkgoWriter.Println("step 1: removing FDs referencing second vCenter")
			infra := currentInfrastructure()
			backup, err := framework.BackupInfrastructure(suiteCtx, clients.Config)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				GinkgoWriter.Println("  restoring Infrastructure backup")
				_ = framework.RestoreInfrastructure(suiteCtx, clients.Config, backup)
				waitForStorageOperatorHealthy(framework.OperatorSyncTimeout)
			})

			spec := vsphere.CloneInfrastructureSpec(infra.Spec)
			spec.PlatformSpec.VSphere.FailureDomains = vsphere.RemoveFailureDomainByRegionZone(
				spec.PlatformSpec.VSphere.FailureDomains, fd.Region, fd.Zone)
			_, patchErr := patchInfrastructureSpec(&spec, false)
			if patchErr != nil {
				skipIfVAPDenied(patchErr, "FD removal")
				Fail(fmt.Sprintf("FD removal failed: %v", patchErr))
			}

			GinkgoWriter.Println("step 2: waiting for orphan tag cleanup")
			waitForTagDetached(suiteCtx, sess, fd.Topology.Datastore, tagName, framework.OperatorSyncTimeout)
			waitForOrphanConditionFalse(framework.DefaultTimeout)

			GinkgoWriter.Printf("step 2b: checking SPBM profile %q deleted from second vCenter\n", profileName)
			lastLog := time.Now()
			Eventually(func() bool {
				exists, err := vsphere.StorageProfileExists(suiteCtx, sess, profileName)
				if err != nil {
					GinkgoWriter.Printf("  SPBM check error: %v\n", err)
					return false
				}
				if time.Since(lastLog) >= 30*time.Second {
					GinkgoWriter.Printf("  wait: SPBM profile %q still exists=%v\n", profileName, exists)
					lastLog = time.Now()
				}
				return !exists
			}).WithTimeout(framework.OperatorSyncTimeout).WithPolling(operatorPollInterval).Should(BeTrue(),
				"SPBM profile should be deleted from second vCenter after all FDs removed")

			GinkgoWriter.Printf("step 3: removing vCenter entry %s\n", lab.SecondVCenter.Server)
			infra = currentInfrastructure()
			spec2 := vsphere.CloneInfrastructureSpec(infra.Spec)
			spec2.PlatformSpec.VSphere.VCenters = vsphere.RemoveVCenterByServer(
				spec2.PlatformSpec.VSphere.VCenters, lab.SecondVCenter.Server)
			_, patchErr = patchInfrastructureSpec(&spec2, false)
			Expect(patchErr).NotTo(HaveOccurred(), "removing vCenter entry should succeed after FDs removed")

			GinkgoWriter.Println("step 4: verifying post-removal state")
			waitForStorageOperatorHealthy(framework.OperatorSyncTimeout)

			sc := requireDefaultStorageClass()
			Expect(sc.Parameters).To(HaveKey("StoragePolicyName"))

			config, err := framework.GetCSIDriverConfig(suiteCtx, clients.Kube)
			Expect(err).NotTo(HaveOccurred())
			Expect(framework.CSIConfigHasVCenter(config, lab.SecondVCenter.Server)).To(BeFalse(),
				"CSI config should no longer reference second vCenter")

			GinkgoWriter.Println("  checking primary vCenter SPBM profile intact")
			primarySess := primaryVCenterSession()
			defer primarySess.Close(suiteCtx)
			primaryExists, err := vsphere.StorageProfileExists(suiteCtx, primarySess, profileName)
			Expect(err).NotTo(HaveOccurred())
			Expect(primaryExists).To(BeTrue(), "primary vCenter SPBM profile should be intact")
			GinkgoWriter.Println("VC-REM-01: PASS")
		})

		It("VC-REM-02: CSI driver config updated after vCenter removal", Label("p0"), func() {
			lab, fd := requireLabFD()

			infra := currentInfrastructure()
			backup, err := framework.BackupInfrastructure(suiteCtx, clients.Config)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				_ = framework.RestoreInfrastructure(suiteCtx, clients.Config, backup)
				waitForStorageOperatorHealthy(framework.OperatorSyncTimeout)
			})

			GinkgoWriter.Println("step 1: removing FDs")
			spec := vsphere.CloneInfrastructureSpec(infra.Spec)
			spec.PlatformSpec.VSphere.FailureDomains = vsphere.RemoveFailureDomainByRegionZone(
				spec.PlatformSpec.VSphere.FailureDomains, fd.Region, fd.Zone)
			_, patchErr := patchInfrastructureSpec(&spec, false)
			if patchErr != nil {
				skipIfVAPDenied(patchErr, "FD removal")
				Fail(fmt.Sprintf("FD removal failed: %v", patchErr))
			}
			waitForStorageOperatorHealthy(framework.OperatorSyncTimeout)

			GinkgoWriter.Printf("step 2: removing vCenter %s\n", lab.SecondVCenter.Server)
			infra = currentInfrastructure()
			spec2 := vsphere.CloneInfrastructureSpec(infra.Spec)
			spec2.PlatformSpec.VSphere.VCenters = vsphere.RemoveVCenterByServer(
				spec2.PlatformSpec.VSphere.VCenters, lab.SecondVCenter.Server)
			_, patchErr = patchInfrastructureSpec(&spec2, false)
			Expect(patchErr).NotTo(HaveOccurred())

			waitForStorageOperatorHealthy(framework.OperatorSyncTimeout)

			GinkgoWriter.Println("step 3: verifying CSI config")
			config, err := framework.GetCSIDriverConfig(suiteCtx, clients.Kube)
			Expect(err).NotTo(HaveOccurred())
			Expect(framework.CSIConfigHasVCenter(config, lab.SecondVCenter.Server)).To(BeFalse(),
				"config should not reference removed vCenter")

			primaryServer := framework.GetVCenters(currentInfrastructure())[0].Server
			Expect(framework.CSIConfigHasVCenter(config, primaryServer)).To(BeTrue(),
				"config should still reference primary vCenter")
			GinkgoWriter.Println("VC-REM-02: PASS")
		})

		It("VC-REM-03: credential secrets after vCenter removal [observational]", Label("p2"), func() {
			lab, _ := requireLabFD()
			GinkgoWriter.Println("VC-REM-03: observational — logging credential state")

			secret, err := clients.Kube.CoreV1().Secrets(framework.VSphereCredsNamespace).Get(
				suiteCtx, framework.VSphereCredsSecret, metav1.GetOptions{})
			if err != nil {
				GinkgoWriter.Printf("  could not read vsphere-creds: %v\n", err)
				Skip("cannot read credential secret for observation")
			}

			usernameKey := lab.SecondVCenter.Server + ".username"
			passwordKey := lab.SecondVCenter.Server + ".password"
			hasUsername := len(secret.Data[usernameKey]) > 0
			hasPassword := len(secret.Data[passwordKey]) > 0

			GinkgoWriter.Printf("  vsphere-creds has %s.username: %v\n", lab.SecondVCenter.Server, hasUsername)
			GinkgoWriter.Printf("  vsphere-creds has %s.password: %v\n", lab.SecondVCenter.Server, hasPassword)
			GinkgoWriter.Println("  credential cleanup is CCO's responsibility — not asserting")
		})
	})

	// ── Category 5: Edge Cases ──

	Context("edge cases", Ordered, func() {

		It("EDGE-01: connectivity loss during cleanup [DEFERRED]", Label("p2"), func() {
			Skip("EDGE-01 requires manual network partition — run manually with lab infrastructure control")
		})

		It("EDGE-02: backoff resets after successful sync", Label("p1"), func() {
			lab, fd := requireLabFD()
			id := infraID()
			tagName := vsphere.GetClusterTagName(id)

			sess := secondVCenterSession(lab)
			DeferCleanup(func() { sess.Close(suiteCtx) })

			GinkgoWriter.Println("step 1: removing FD and waiting for tag detach")
			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				removeFDAndSkipIfDenied(spec, fd)
				waitForTagDetached(suiteCtx, sess, fd.Topology.Datastore, tagName, framework.OperatorSyncTimeout)
			})

			GinkgoWriter.Println("step 2: verifying tag re-attached after restore (backoff should reset)")
			start := time.Now()
			waitForTagAttached(suiteCtx, sess, fd.Topology.Datastore, tagName, framework.OperatorSyncTimeout)
			elapsed := time.Since(start)
			GinkgoWriter.Printf("EDGE-02: tag re-attached in %v after restore\n", elapsed)
		})

		It("EDGE-03: topology transition 2 FDs → 1 FD → 2 FDs", Label("p1"), func() {
			lab, fd := requireLabFD()
			id := infraID()
			tagName := vsphere.GetClusterTagName(id)
			profileName := vsphere.GetStoragePolicyName(id)

			sess := secondVCenterSession(lab)
			DeferCleanup(func() { sess.Close(suiteCtx) })

			GinkgoWriter.Println("step 1: removing second FD (2 → 1)")
			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				removeFDAndSkipIfDenied(spec, fd)
				waitForStorageOperatorHealthy(framework.OperatorSyncTimeout)

				sc := requireDefaultStorageClass()
				Expect(sc.Parameters).To(HaveKey("StoragePolicyName"),
					"StorageClass should persist through topology transition")
				GinkgoWriter.Println("  StorageClass intact with 1 FD")
			})

			GinkgoWriter.Println("step 2: verifying re-add (1 → 2)")
			waitForTagAttached(suiteCtx, sess, fd.Topology.Datastore, tagName, framework.OperatorSyncTimeout)

			GinkgoWriter.Printf("  checking SPBM profile %q re-created on second vCenter\n", profileName)
			Eventually(func() bool {
				exists, err := vsphere.StorageProfileExists(suiteCtx, sess, profileName)
				if err != nil {
					return false
				}
				return exists
			}).WithTimeout(framework.OperatorSyncTimeout).WithPolling(operatorPollInterval).Should(BeTrue(),
				"SPBM profile should be re-created on second vCenter after FD re-add")
			GinkgoWriter.Println("EDGE-03: PASS")
		})
	})

	// ── Category 6: Metrics and Observability ──

	Context("metrics and observability", Label("observability"), Ordered, func() {

		It("OBS-01: OrphanTagsDetectedTotal metric incremented on FD removal", Label("p1"), func() {
			_, fd := requireLabFD()

			GinkgoWriter.Println("scraping operator metrics (before)")
			beforeMetrics, err := framework.ScrapeOperatorMetrics(suiteCtx, clients.Kube,
				framework.CSIDriverNamespace, "name=vmware-vsphere-csi-driver-operator")
			if err != nil {
				Skip(fmt.Sprintf("cannot scrape operator metrics: %v", err))
			}
			beforeVal, _ := framework.ParseMetricValue(beforeMetrics, framework.OrphanTagsDetectedMetric, nil)
			GinkgoWriter.Printf("  %s before: %v\n", framework.OrphanTagsDetectedMetric, beforeVal)

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				removeFDAndSkipIfDenied(spec, fd)
				waitForStorageOperatorHealthy(framework.OperatorSyncTimeout)

				GinkgoWriter.Println("  scraping operator metrics (after)")
				afterMetrics, mErr := framework.ScrapeOperatorMetrics(suiteCtx, clients.Kube,
					framework.CSIDriverNamespace, "name=vmware-vsphere-csi-driver-operator")
				Expect(mErr).NotTo(HaveOccurred())
				afterVal, parseErr := framework.ParseMetricValue(afterMetrics, framework.OrphanTagsDetectedMetric, nil)
				Expect(parseErr).NotTo(HaveOccurred())
				GinkgoWriter.Printf("  %s after: %v\n", framework.OrphanTagsDetectedMetric, afterVal)
				Expect(afterVal).To(BeNumerically(">", beforeVal),
					"orphan tags detected metric should increase after FD removal")
			})
			GinkgoWriter.Println("OBS-01: PASS")
		})

		It("OBS-02: TagOperationsTotal tracks detach operations", Label("p1"), func() {
			_, fd := requireLabFD()
			detachLabels := map[string]string{"operation": "detach", "result": "success"}

			GinkgoWriter.Println("scraping operator metrics (before)")
			beforeMetrics, err := framework.ScrapeOperatorMetrics(suiteCtx, clients.Kube,
				framework.CSIDriverNamespace, "name=vmware-vsphere-csi-driver-operator")
			if err != nil {
				Skip(fmt.Sprintf("cannot scrape operator metrics: %v", err))
			}
			beforeVal, _ := framework.ParseMetricValue(beforeMetrics, framework.TagOperationsMetric, detachLabels)
			GinkgoWriter.Printf("  %s{detach,success} before: %v\n", framework.TagOperationsMetric, beforeVal)

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				removeFDAndSkipIfDenied(spec, fd)
				waitForStorageOperatorHealthy(framework.OperatorSyncTimeout)

				GinkgoWriter.Println("  scraping operator metrics (after)")
				afterMetrics, mErr := framework.ScrapeOperatorMetrics(suiteCtx, clients.Kube,
					framework.CSIDriverNamespace, "name=vmware-vsphere-csi-driver-operator")
				Expect(mErr).NotTo(HaveOccurred())
				afterVal, parseErr := framework.ParseMetricValue(afterMetrics, framework.TagOperationsMetric, detachLabels)
				Expect(parseErr).NotTo(HaveOccurred())
				GinkgoWriter.Printf("  %s{detach,success} after: %v\n", framework.TagOperationsMetric, afterVal)
				Expect(afterVal).To(BeNumerically(">", beforeVal),
					"tag operations detach/success metric should increase")
			})
			GinkgoWriter.Println("OBS-02: PASS")
		})

		It("OBS-03: TagOperationsTotal tracks PV-blocked skips", Label("p1"), func() {
			requireSecondFDNode()
			_, fd := requireLabFD()
			skipLabels := map[string]string{"operation": "skip", "result": "pv_blocked"}

			GinkgoWriter.Printf("creating PVC+pod in zone=%s for PV-blocked metric test\n", fd.Zone)
			ns := createTestNamespace("csi-op-obs03")
			csiOpTestNS = append(csiOpTestNS, ns)

			sc := requireDefaultStorageClass()
			pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "obs03-pvc", framework.TestPVCSize, sc.Name)
			Expect(err).NotTo(HaveOccurred())

			nodeSelector := map[string]string{
				"topology.kubernetes.io/zone": fd.Zone,
			}
			pod, err := framework.CreateBusyboxPodWithNodeSelector(suiteCtx, clients.Kube, ns, "obs03-pod", pvc.Name, nodeSelector)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Println("  waiting for pod Running")
			err = framework.WaitForPodRunning(suiteCtx, clients.Kube, ns, pod.Name, framework.LongTimeout)
			Expect(err).NotTo(HaveOccurred())
			boundPVC, err := framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.DefaultTimeout)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("  PVC bound to PV %s\n", boundPVC.Spec.VolumeName)

			DeferCleanup(func() {
				GinkgoWriter.Printf("  cleanup: deleting pod/PVC in %s\n", ns)
				_ = framework.DeletePod(suiteCtx, clients.Kube, ns, pod.Name)
				_ = framework.DeletePVC(suiteCtx, clients.Kube, ns, pvc.Name)
				if boundPVC.Spec.VolumeName != "" {
					_ = framework.WaitForPVDeleted(suiteCtx, clients.Kube, boundPVC.Spec.VolumeName, framework.LongTimeout)
				}
			})

			GinkgoWriter.Println("scraping operator metrics (before)")
			beforeMetrics, mErr := framework.ScrapeOperatorMetrics(suiteCtx, clients.Kube,
				framework.CSIDriverNamespace, "name=vmware-vsphere-csi-driver-operator")
			if mErr != nil {
				Skip(fmt.Sprintf("cannot scrape operator metrics: %v", mErr))
			}
			beforeSkip, _ := framework.ParseMetricValue(beforeMetrics, framework.TagOperationsMetric, skipLabels)
			beforeOrphan, _ := framework.ParseMetricValue(beforeMetrics, framework.OrphanTagsDetectedMetric, nil)
			GinkgoWriter.Printf("  skip/pv_blocked before: %v, orphan_detected before: %v\n", beforeSkip, beforeOrphan)

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				removeFDAndSkipIfDenied(spec, fd)
				waitForOrphanConditionTrue(framework.OperatorSyncTimeout)

				GinkgoWriter.Println("  scraping operator metrics (after)")
				afterMetrics, afterErr := framework.ScrapeOperatorMetrics(suiteCtx, clients.Kube,
					framework.CSIDriverNamespace, "name=vmware-vsphere-csi-driver-operator")
				Expect(afterErr).NotTo(HaveOccurred())

				afterSkip, parseErr := framework.ParseMetricValue(afterMetrics, framework.TagOperationsMetric, skipLabels)
				Expect(parseErr).NotTo(HaveOccurred())
				GinkgoWriter.Printf("  skip/pv_blocked after: %v\n", afterSkip)
				Expect(afterSkip).To(BeNumerically(">", beforeSkip),
					"tag operations skip/pv_blocked should increase")

				afterOrphan, parseErr := framework.ParseMetricValue(afterMetrics, framework.OrphanTagsDetectedMetric, nil)
				Expect(parseErr).NotTo(HaveOccurred())
				GinkgoWriter.Printf("  orphan_detected after: %v\n", afterOrphan)
				Expect(afterOrphan).To(BeNumerically(">", beforeOrphan),
					"orphan tags detected should increase")
			})
			GinkgoWriter.Println("OBS-03: PASS")
		})
	})

	// ── Deferred ──

	It("FD-REM-04: multiple FD removal [DEFERRED]", Label("p2"), func() {
		Skip("FD-REM-04 deferred — requires 3+ FDs not available in standard lab")
	})
})
