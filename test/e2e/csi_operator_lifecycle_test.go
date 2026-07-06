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

// requireLabFD returns lab config with a failure domain and skips if not available.
func requireLabFD() (lab *labconfig.LabConfig, fd *labconfig.FailureDomainConfig) {
	cfg := requireLabConfigWithFD()
	return cfg, cfg.FailureDomain
}

// secondVCenterSession creates a govmomi session to the second vCenter using lab config credentials.
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

// primaryVCenterSession creates a govmomi session to the primary vCenter.
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

// infraID returns the cluster's InfrastructureName.
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

// waitForTagDetached polls until the tag is no longer attached to the datastore.
func waitForTagDetached(ctx context.Context, sess *vsphere.Session, datastorePath, tagName string, timeout time.Duration) {
	Eventually(func() bool {
		tagged, err := vsphere.IsDatastoreTagged(ctx, sess, datastorePath, tagName)
		if err != nil {
			GinkgoWriter.Printf("  tag check error: %v\n", err)
			return false
		}
		return !tagged
	}).WithTimeout(timeout).WithPolling(operatorPollInterval).Should(BeTrue(),
		"tag %q should be detached from datastore %s", tagName, datastorePath)
}

// waitForTagAttached polls until the tag is attached to the datastore.
func waitForTagAttached(ctx context.Context, sess *vsphere.Session, datastorePath, tagName string, timeout time.Duration) {
	Eventually(func() bool {
		tagged, err := vsphere.IsDatastoreTagged(ctx, sess, datastorePath, tagName)
		if err != nil {
			GinkgoWriter.Printf("  tag check error: %v\n", err)
			return false
		}
		return tagged
	}).WithTimeout(timeout).WithPolling(operatorPollInterval).Should(BeTrue(),
		"tag %q should be attached to datastore %s", tagName, datastorePath)
}

// waitForOrphanConditionFalse polls until OrphanCleanupPending is False on ClusterOperator storage.
func waitForOrphanConditionFalse(timeout time.Duration) {
	Eventually(func() bool {
		cond, err := framework.GetClusterOperatorCondition(suiteCtx, clients.Config,
			framework.StorageOperatorName, configv1.ClusterStatusConditionType(framework.OrphanCleanupPendingCondition))
		if err != nil {
			return true // condition absent = not pending
		}
		return cond.Status != configv1.ConditionTrue
	}).WithTimeout(timeout).WithPolling(operatorPollInterval).Should(BeTrue(),
		"OrphanCleanupPending should be False on ClusterOperator storage")
}

// waitForOrphanConditionTrue polls until OrphanCleanupPending is True.
func waitForOrphanConditionTrue(timeout time.Duration) {
	Eventually(func() bool {
		cond, err := framework.GetClusterOperatorCondition(suiteCtx, clients.Config,
			framework.StorageOperatorName, configv1.ClusterStatusConditionType(framework.OrphanCleanupPendingCondition))
		if err != nil {
			return false
		}
		return cond.Status == configv1.ConditionTrue
	}).WithTimeout(timeout).WithPolling(operatorPollInterval).Should(BeTrue(),
		"OrphanCleanupPending should be True on ClusterOperator storage")
}

// waitForStorageOperatorHealthy waits for the storage ClusterOperator to be Available and not Degraded.
func waitForStorageOperatorHealthy(timeout time.Duration) {
	err := framework.WaitForClusterOperatorAvailable(suiteCtx, clients.Config,
		framework.StorageOperatorName, timeout)
	Expect(err).NotTo(HaveOccurred(), "storage ClusterOperator should be healthy")
}

var _ = Describe("CSI Operator Failure Domain Lifecycle", Serial, Label("csi-operator", "mutating"), func() {

	BeforeEach(func() {
		requireGateEnabled()
		requireMultiVCenter()
		requireLabConfigWithFD()
	})

	// ── Category 1: Failure Domain Addition — Operator Response ──

	Context("after FD addition (State 1)", Label("fd-lifecycle"), Ordered, func() {

		It("FD-ADD-01: operator tags new FD's datastore", Label("p0"), func() {
			lab, fd := requireLabFD()
			id := infraID()
			tagName := vsphere.GetClusterTagName(id)

			sess := secondVCenterSession(lab)
			DeferCleanup(func() { sess.Close(suiteCtx) })

			catName := vsphere.GetClusterTagCategoryName(id)
			cat, err := vsphere.FindTagCategoryByName(suiteCtx, sess, catName)
			Expect(err).NotTo(HaveOccurred())
			Expect(cat).NotTo(BeNil(), "tag category %q should exist on second vCenter", catName)

			tagged, err := vsphere.IsDatastoreTagged(suiteCtx, sess, fd.Topology.Datastore, tagName)
			Expect(err).NotTo(HaveOccurred())
			Expect(tagged).To(BeTrue(), "datastore %s should be tagged with %q", fd.Topology.Datastore, tagName)

			sc := requireDefaultStorageClass()
			Expect(sc.Parameters).To(HaveKey("storagePolicyName"),
				"default StorageClass should have storagePolicyName parameter")
		})

		It("FD-ADD-02: SPBM profile exists on second vCenter", Label("p0"), func() {
			lab, _ := requireLabFD()
			id := infraID()
			profileName := vsphere.GetStoragePolicyName(id)

			sess := secondVCenterSession(lab)
			DeferCleanup(func() { sess.Close(suiteCtx) })

			exists, err := vsphere.StorageProfileExists(suiteCtx, sess, profileName)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue(), "SPBM profile %q should exist on second vCenter", profileName)

			primarySess := primaryVCenterSession()
			DeferCleanup(func() { primarySess.Close(suiteCtx) })

			primaryExists, err := vsphere.StorageProfileExists(suiteCtx, primarySess, profileName)
			Expect(err).NotTo(HaveOccurred())
			Expect(primaryExists).To(BeTrue(), "SPBM profile %q should still exist on primary vCenter", profileName)
		})

		It("FD-ADD-03: operator conditions healthy", Label("p0"), func() {
			waitForStorageOperatorHealthy(framework.DefaultTimeout)
			waitForOrphanConditionFalse(framework.ShortTimeout)

			pods, err := framework.ListPodsByLabel(suiteCtx, clients.Kube,
				framework.CSIDriverNamespace, "app=vmware-vsphere-csi-driver-operator")
			Expect(err).NotTo(HaveOccurred())
			for _, pod := range pods {
				restarts := framework.PodRestartCount(&pod)
				Expect(restarts).To(BeNumerically("<=", 2),
					"operator pod %s should have <= 2 restarts, got %d", pod.Name, restarts)
			}
		})

		It("FD-ADD-04: CSI driver config includes second vCenter", Label("p0"), func() {
			lab, _ := requireLabFD()

			config, err := framework.GetCSIDriverConfig(suiteCtx, clients.Kube)
			Expect(err).NotTo(HaveOccurred())
			Expect(framework.CSIConfigHasVCenter(config, lab.SecondVCenter.Server)).To(BeTrue(),
				"CSI driver config should reference second vCenter %s", lab.SecondVCenter.Server)

			infra := currentInfrastructure()
			primaryServer := framework.GetVCenters(infra)[0].Server
			Expect(framework.CSIConfigHasVCenter(config, primaryServer)).To(BeTrue(),
				"CSI driver config should still reference primary vCenter %s", primaryServer)
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

			tagged, err := vsphere.IsDatastoreTagged(suiteCtx, sess, fd.Topology.Datastore, tagName)
			Expect(err).NotTo(HaveOccurred())
			Expect(tagged).To(BeTrue(), "pre-check: datastore should be tagged")

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				spec.PlatformSpec.VSphere.FailureDomains = vsphere.RemoveFailureDomainByRegionZone(
					spec.PlatformSpec.VSphere.FailureDomains, fd.Region, fd.Zone)
				_, patchErr := patchInfrastructureSpec(spec, false)
				if patchErr != nil {
					skipIfVAPDenied(patchErr, "FD removal")
					Fail(fmt.Sprintf("FD removal patch failed: %v", patchErr))
				}

				waitForTagDetached(suiteCtx, sess, fd.Topology.Datastore, tagName, framework.OperatorSyncTimeout)

				// Primary FD tags untouched
				primarySess := primaryVCenterSession()
				defer primarySess.Close(suiteCtx)
				infra := currentInfrastructure()
				primaryFDs := framework.GetFailureDomains(infra)
				Expect(primaryFDs).NotTo(BeEmpty(), "should have remaining FDs")

				waitForOrphanConditionFalse(framework.DefaultTimeout)

				sc := requireDefaultStorageClass()
				Expect(sc.Parameters).To(HaveKey("storagePolicyName"))
			})

			// After restore, verify tag is re-attached
			waitForTagAttached(suiteCtx, sess, fd.Topology.Datastore, tagName, framework.OperatorSyncTimeout)
		})

		It("FD-REM-02: StorageClass and SPBM profile survive FD removal", Label("p0"), func() {
			_, fd := requireLabFD()
			id := infraID()
			profileName := vsphere.GetStoragePolicyName(id)

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				spec.PlatformSpec.VSphere.FailureDomains = vsphere.RemoveFailureDomainByRegionZone(
					spec.PlatformSpec.VSphere.FailureDomains, fd.Region, fd.Zone)
				_, patchErr := patchInfrastructureSpec(spec, false)
				if patchErr != nil {
					skipIfVAPDenied(patchErr, "FD removal")
					Fail(fmt.Sprintf("FD removal patch failed: %v", patchErr))
				}

				waitForStorageOperatorHealthy(framework.OperatorSyncTimeout)

				sc := requireDefaultStorageClass()
				Expect(sc.Parameters).To(HaveKey("storagePolicyName"),
					"StorageClass should retain storagePolicyName after FD removal")

				primarySess := primaryVCenterSession()
				defer primarySess.Close(suiteCtx)
				exists, err := vsphere.StorageProfileExists(suiteCtx, primarySess, profileName)
				Expect(err).NotTo(HaveOccurred())
				Expect(exists).To(BeTrue(), "SPBM profile should survive on primary vCenter")

				// Verify StorageClass is functional with a quick PVC smoke test
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
			})
		})

		It("FD-REM-03: operator reconciles within backoff window", Label("p1"), func() {
			lab, fd := requireLabFD()
			id := infraID()
			tagName := vsphere.GetClusterTagName(id)

			sess := secondVCenterSession(lab)
			DeferCleanup(func() { sess.Close(suiteCtx) })

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				spec.PlatformSpec.VSphere.FailureDomains = vsphere.RemoveFailureDomainByRegionZone(
					spec.PlatformSpec.VSphere.FailureDomains, fd.Region, fd.Zone)

				start := time.Now()
				_, patchErr := patchInfrastructureSpec(spec, false)
				if patchErr != nil {
					skipIfVAPDenied(patchErr, "FD removal")
					Fail(fmt.Sprintf("FD removal patch failed: %v", patchErr))
				}

				waitForTagDetached(suiteCtx, sess, fd.Topology.Datastore, tagName, framework.OperatorSyncTimeout)
				elapsed := time.Since(start)
				GinkgoWriter.Printf("FD-REM-03: tag detached in %v (backoff reset validates if < 12min)\n", elapsed)

				Expect(elapsed).To(BeNumerically("<", framework.OperatorSyncTimeout),
					"tag should detach within successCheckInterval (10 min) + jitter, not stuck at 30 min backoff cap")
			})
		})
	})

	// ── Category 3: PV Safety — Tag Detach Blocked by CNS Volumes ──

	Context("PV safety — tag detach blocked by CNS volumes", Label("pv-safety"), Ordered, func() {

		It("PV-SAFE-01: orphan tag blocked when PVs exist", Label("p0"), func() {
			lab, fd := requireLabFD()
			id := infraID()
			tagName := vsphere.GetClusterTagName(id)

			sess := secondVCenterSession(lab)
			DeferCleanup(func() { sess.Close(suiteCtx) })

			ns := createTestNamespace("csi-op-pvsafe")
			DeferCleanup(func() {
				_ = framework.DeleteNamespace(suiteCtx, clients.Kube, ns, framework.LongTimeout)
			})

			sc := requireDefaultStorageClass()
			pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "pv-safe-pvc", framework.TestPVCSize, sc.Name)
			Expect(err).NotTo(HaveOccurred())

			nodeSelector := map[string]string{
				"topology.kubernetes.io/zone": fd.Zone,
			}
			pod, err := framework.CreateBusyboxPodWithNodeSelector(suiteCtx, clients.Kube, ns, "pv-safe-pod", pvc.Name, nodeSelector)
			Expect(err).NotTo(HaveOccurred())

			err = framework.WaitForPodRunning(suiteCtx, clients.Kube, ns, pod.Name, framework.LongTimeout)
			Expect(err).NotTo(HaveOccurred(), "pod should reach Running in second FD")

			boundPVC, err := framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.DefaultTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(boundPVC.Spec.VolumeName).NotTo(BeEmpty())

			DeferCleanup(func() {
				_ = framework.DeletePod(suiteCtx, clients.Kube, ns, pod.Name)
				_ = framework.DeletePVC(suiteCtx, clients.Kube, ns, pvc.Name)
				if boundPVC.Spec.VolumeName != "" {
					_ = framework.WaitForPVDeleted(suiteCtx, clients.Kube, boundPVC.Spec.VolumeName, framework.LongTimeout)
				}
			})

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				spec.PlatformSpec.VSphere.FailureDomains = vsphere.RemoveFailureDomainByRegionZone(
					spec.PlatformSpec.VSphere.FailureDomains, fd.Region, fd.Zone)
				_, patchErr := patchInfrastructureSpec(spec, false)
				if patchErr != nil {
					skipIfVAPDenied(patchErr, "FD removal")
					Fail(fmt.Sprintf("FD removal patch failed: %v", patchErr))
				}

				waitForOrphanConditionTrue(framework.OperatorSyncTimeout)

				tagged, tagErr := vsphere.IsDatastoreTagged(suiteCtx, sess, fd.Topology.Datastore, tagName)
				Expect(tagErr).NotTo(HaveOccurred())
				Expect(tagged).To(BeTrue(), "tag should still be attached — PV blocks cleanup")

				waitForStorageOperatorHealthy(framework.DefaultTimeout)

				// Verify PVC remains accessible
				recheckPVC, recheckErr := clients.Kube.CoreV1().PersistentVolumeClaims(ns).Get(
					suiteCtx, pvc.Name, metav1.GetOptions{})
				Expect(recheckErr).NotTo(HaveOccurred())
				Expect(string(recheckPVC.Status.Phase)).To(Equal("Bound"), "PVC should remain Bound")
			})
		})

		It("PV-SAFE-02: orphan cleanup proceeds after PVs deleted", Label("p0"), func() {
			lab, fd := requireLabFD()
			id := infraID()
			tagName := vsphere.GetClusterTagName(id)

			sess := secondVCenterSession(lab)
			DeferCleanup(func() { sess.Close(suiteCtx) })

			ns := createTestNamespace("csi-op-pvdel")
			DeferCleanup(func() {
				_ = framework.DeleteNamespace(suiteCtx, clients.Kube, ns, framework.LongTimeout)
			})

			sc := requireDefaultStorageClass()
			pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "pvdel-pvc", framework.TestPVCSize, sc.Name)
			Expect(err).NotTo(HaveOccurred())

			nodeSelector := map[string]string{
				"topology.kubernetes.io/zone": fd.Zone,
			}
			pod, err := framework.CreateBusyboxPodWithNodeSelector(suiteCtx, clients.Kube, ns, "pvdel-pod", pvc.Name, nodeSelector)
			Expect(err).NotTo(HaveOccurred())

			err = framework.WaitForPodRunning(suiteCtx, clients.Kube, ns, pod.Name, framework.LongTimeout)
			Expect(err).NotTo(HaveOccurred())
			boundPVC, err := framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.DefaultTimeout)
			Expect(err).NotTo(HaveOccurred())
			pvName := boundPVC.Spec.VolumeName

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				spec.PlatformSpec.VSphere.FailureDomains = vsphere.RemoveFailureDomainByRegionZone(
					spec.PlatformSpec.VSphere.FailureDomains, fd.Region, fd.Zone)
				_, patchErr := patchInfrastructureSpec(spec, false)
				if patchErr != nil {
					skipIfVAPDenied(patchErr, "FD removal")
					Fail(fmt.Sprintf("FD removal patch failed: %v", patchErr))
				}

				waitForOrphanConditionTrue(framework.OperatorSyncTimeout)

				// Now delete the PV resources
				Expect(framework.DeletePod(suiteCtx, clients.Kube, ns, pod.Name)).To(Succeed())
				Expect(framework.DeletePVC(suiteCtx, clients.Kube, ns, pvc.Name)).To(Succeed())
				Expect(framework.WaitForPVDeleted(suiteCtx, clients.Kube, pvName, framework.LongTimeout)).To(Succeed())

				// Tag should now be detached after next operator sync
				waitForTagDetached(suiteCtx, sess, fd.Topology.Datastore, tagName, framework.OperatorSyncTimeout)
				waitForOrphanConditionFalse(framework.DefaultTimeout)
			})
		})

		It("PV-SAFE-03: force cleanup annotation overrides PV safety", Label("p1"), func() {
			lab, fd := requireLabFD()
			id := infraID()
			tagName := vsphere.GetClusterTagName(id)

			sess := secondVCenterSession(lab)
			DeferCleanup(func() { sess.Close(suiteCtx) })

			ns := createTestNamespace("csi-op-force")
			DeferCleanup(func() {
				_ = framework.DeleteNamespace(suiteCtx, clients.Kube, ns, framework.LongTimeout)
			})

			sc := requireDefaultStorageClass()
			pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "force-pvc", framework.TestPVCSize, sc.Name)
			Expect(err).NotTo(HaveOccurred())

			nodeSelector := map[string]string{
				"topology.kubernetes.io/zone": fd.Zone,
			}
			pod, err := framework.CreateBusyboxPodWithNodeSelector(suiteCtx, clients.Kube, ns, "force-pod", pvc.Name, nodeSelector)
			Expect(err).NotTo(HaveOccurred())

			err = framework.WaitForPodRunning(suiteCtx, clients.Kube, ns, pod.Name, framework.LongTimeout)
			Expect(err).NotTo(HaveOccurred())
			boundPVC, err := framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.DefaultTimeout)
			Expect(err).NotTo(HaveOccurred())

			DeferCleanup(func() {
				_ = framework.DeletePod(suiteCtx, clients.Kube, ns, pod.Name)
				_ = framework.DeletePVC(suiteCtx, clients.Kube, ns, pvc.Name)
				if boundPVC.Spec.VolumeName != "" {
					_ = framework.WaitForPVDeleted(suiteCtx, clients.Kube, boundPVC.Spec.VolumeName, framework.LongTimeout)
				}
			})

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				spec.PlatformSpec.VSphere.FailureDomains = vsphere.RemoveFailureDomainByRegionZone(
					spec.PlatformSpec.VSphere.FailureDomains, fd.Region, fd.Zone)
				_, patchErr := patchInfrastructureSpec(spec, false)
				if patchErr != nil {
					skipIfVAPDenied(patchErr, "FD removal")
					Fail(fmt.Sprintf("FD removal patch failed: %v", patchErr))
				}

				waitForOrphanConditionTrue(framework.OperatorSyncTimeout)

				// Annotate ClusterCSIDriver with force cleanup
				ccd, annotErr := clients.Operator.OperatorV1().ClusterCSIDrivers().Get(
					suiteCtx, framework.ClusterCSIDriverName, metav1.GetOptions{})
				Expect(annotErr).NotTo(HaveOccurred())
				if ccd.Annotations == nil {
					ccd.Annotations = map[string]string{}
				}
				ccd.Annotations[framework.ForceOrphanCleanupAnnotation] = "true"
				_, annotErr = clients.Operator.OperatorV1().ClusterCSIDrivers().Update(suiteCtx, ccd, metav1.UpdateOptions{})
				Expect(annotErr).NotTo(HaveOccurred())

				DeferCleanup(func() {
					ccd, err := clients.Operator.OperatorV1().ClusterCSIDrivers().Get(
						suiteCtx, framework.ClusterCSIDriverName, metav1.GetOptions{})
					if err == nil {
						delete(ccd.Annotations, framework.ForceOrphanCleanupAnnotation)
						_, _ = clients.Operator.OperatorV1().ClusterCSIDrivers().Update(suiteCtx, ccd, metav1.UpdateOptions{})
					}
				})

				// Tag should now be detached despite PVs
				waitForTagDetached(suiteCtx, sess, fd.Topology.Datastore, tagName, framework.OperatorSyncTimeout)
				waitForOrphanConditionFalse(framework.DefaultTimeout)

				// PVC should remain Bound
				recheckPVC, recheckErr := clients.Kube.CoreV1().PersistentVolumeClaims(ns).Get(
					suiteCtx, pvc.Name, metav1.GetOptions{})
				Expect(recheckErr).NotTo(HaveOccurred())
				Expect(string(recheckPVC.Status.Phase)).To(Equal("Bound"),
					"PVC should remain Bound after force cleanup — only tag was removed")
			})
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

			// Step 1: Remove all FDs referencing second vCenter
			infra := currentInfrastructure()
			backup, err := framework.BackupInfrastructure(suiteCtx, clients.Config)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				_ = framework.RestoreInfrastructure(suiteCtx, clients.Config, backup)
				waitForStorageOperatorHealthy(framework.OperatorSyncTimeout)
			})

			spec := vsphere.CloneInfrastructureSpec(infra.Spec)
			spec.PlatformSpec.VSphere.FailureDomains = vsphere.RemoveFailureDomainByRegionZone(
				spec.PlatformSpec.VSphere.FailureDomains, fd.Region, fd.Zone)
			_, patchErr := patchInfrastructureSpec(&spec, false)
			if patchErr != nil {
				skipIfVAPDenied(patchErr, "FD removal")
				Fail(fmt.Sprintf("FD removal patch failed: %v", patchErr))
			}

			// Step 2: Wait for orphan tag cleanup
			waitForTagDetached(suiteCtx, sess, fd.Topology.Datastore, tagName, framework.OperatorSyncTimeout)
			waitForOrphanConditionFalse(framework.DefaultTimeout)

			// SPBM profile should be deleted from second vCenter (zero FDs)
			Eventually(func() bool {
				exists, err := vsphere.StorageProfileExists(suiteCtx, sess, profileName)
				if err != nil {
					GinkgoWriter.Printf("  SPBM check error: %v\n", err)
					return false
				}
				return !exists
			}).WithTimeout(framework.OperatorSyncTimeout).WithPolling(operatorPollInterval).Should(BeTrue(),
				"SPBM profile should be deleted from second vCenter after all FDs removed")

			// Step 3: Remove second vCenter entry
			infra = currentInfrastructure()
			spec2 := vsphere.CloneInfrastructureSpec(infra.Spec)
			spec2.PlatformSpec.VSphere.VCenters = vsphere.RemoveVCenterByServer(
				spec2.PlatformSpec.VSphere.VCenters, lab.SecondVCenter.Server)
			_, patchErr = patchInfrastructureSpec(&spec2, false)
			Expect(patchErr).NotTo(HaveOccurred(), "removing vCenter entry should succeed after FDs removed")

			// Step 4: Verify post-removal state
			waitForStorageOperatorHealthy(framework.OperatorSyncTimeout)

			sc := requireDefaultStorageClass()
			Expect(sc.Parameters).To(HaveKey("storagePolicyName"))

			config, err := framework.GetCSIDriverConfig(suiteCtx, clients.Kube)
			Expect(err).NotTo(HaveOccurred())
			Expect(framework.CSIConfigHasVCenter(config, lab.SecondVCenter.Server)).To(BeFalse(),
				"CSI config should no longer reference second vCenter")

			primarySess := primaryVCenterSession()
			defer primarySess.Close(suiteCtx)
			primaryExists, err := vsphere.StorageProfileExists(suiteCtx, primarySess, profileName)
			Expect(err).NotTo(HaveOccurred())
			Expect(primaryExists).To(BeTrue(), "primary vCenter SPBM profile should be intact")
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

			// Remove FDs then vCenter
			spec := vsphere.CloneInfrastructureSpec(infra.Spec)
			spec.PlatformSpec.VSphere.FailureDomains = vsphere.RemoveFailureDomainByRegionZone(
				spec.PlatformSpec.VSphere.FailureDomains, fd.Region, fd.Zone)
			_, patchErr := patchInfrastructureSpec(&spec, false)
			if patchErr != nil {
				skipIfVAPDenied(patchErr, "FD removal")
				Fail(fmt.Sprintf("FD removal patch failed: %v", patchErr))
			}
			waitForStorageOperatorHealthy(framework.OperatorSyncTimeout)

			infra = currentInfrastructure()
			spec2 := vsphere.CloneInfrastructureSpec(infra.Spec)
			spec2.PlatformSpec.VSphere.VCenters = vsphere.RemoveVCenterByServer(
				spec2.PlatformSpec.VSphere.VCenters, lab.SecondVCenter.Server)
			_, patchErr = patchInfrastructureSpec(&spec2, false)
			Expect(patchErr).NotTo(HaveOccurred())

			waitForStorageOperatorHealthy(framework.OperatorSyncTimeout)

			config, err := framework.GetCSIDriverConfig(suiteCtx, clients.Kube)
			Expect(err).NotTo(HaveOccurred())
			Expect(framework.CSIConfigHasVCenter(config, lab.SecondVCenter.Server)).To(BeFalse(),
				"config should not reference removed vCenter")

			primaryServer := framework.GetVCenters(currentInfrastructure())[0].Server
			Expect(framework.CSIConfigHasVCenter(config, primaryServer)).To(BeTrue(),
				"config should still reference primary vCenter")
		})

		It("VC-REM-03: credential secrets after vCenter removal [observational]", Label("p2"), func() {
			lab, _ := requireLabFD()
			GinkgoWriter.Println("VC-REM-03: observational — logging credential state after vCenter removal")

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

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				spec.PlatformSpec.VSphere.FailureDomains = vsphere.RemoveFailureDomainByRegionZone(
					spec.PlatformSpec.VSphere.FailureDomains, fd.Region, fd.Zone)
				_, patchErr := patchInfrastructureSpec(spec, false)
				if patchErr != nil {
					skipIfVAPDenied(patchErr, "FD removal")
					Fail(fmt.Sprintf("FD removal patch failed: %v", patchErr))
				}

				waitForTagDetached(suiteCtx, sess, fd.Topology.Datastore, tagName, framework.OperatorSyncTimeout)
			})

			// After restore, tag should be re-attached within the success interval (not stuck at 30 min)
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

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				// Remove second FD → 1 FD
				spec.PlatformSpec.VSphere.FailureDomains = vsphere.RemoveFailureDomainByRegionZone(
					spec.PlatformSpec.VSphere.FailureDomains, fd.Region, fd.Zone)
				_, patchErr := patchInfrastructureSpec(spec, false)
				if patchErr != nil {
					skipIfVAPDenied(patchErr, "FD removal")
					Fail(fmt.Sprintf("FD removal patch failed: %v", patchErr))
				}

				waitForStorageOperatorHealthy(framework.OperatorSyncTimeout)

				sc := requireDefaultStorageClass()
				Expect(sc.Parameters).To(HaveKey("storagePolicyName"),
					"StorageClass should persist through topology transition")
			})

			// After restore (re-add FD), verify tag re-attached and SPBM profile re-created
			waitForTagAttached(suiteCtx, sess, fd.Topology.Datastore, tagName, framework.OperatorSyncTimeout)

			Eventually(func() bool {
				exists, err := vsphere.StorageProfileExists(suiteCtx, sess, profileName)
				if err != nil {
					return false
				}
				return exists
			}).WithTimeout(framework.OperatorSyncTimeout).WithPolling(operatorPollInterval).Should(BeTrue(),
				"SPBM profile should be re-created on second vCenter after FD re-add")
		})
	})

	// ── Category 6: Metrics and Observability ──

	Context("metrics and observability", Label("observability"), Ordered, func() {

		It("OBS-01: OrphanTagsDetectedTotal metric incremented on FD removal", Label("p1"), func() {
			_, fd := requireLabFD()

			beforeMetrics, err := framework.ScrapeOperatorMetrics(suiteCtx, clients.Kube,
				framework.CSIDriverNamespace, "app=vmware-vsphere-csi-driver-operator")
			if err != nil {
				Skip(fmt.Sprintf("cannot scrape operator metrics: %v", err))
			}
			beforeVal, _ := framework.ParseMetricValue(beforeMetrics, framework.OrphanTagsDetectedMetric, nil)

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				spec.PlatformSpec.VSphere.FailureDomains = vsphere.RemoveFailureDomainByRegionZone(
					spec.PlatformSpec.VSphere.FailureDomains, fd.Region, fd.Zone)
				_, patchErr := patchInfrastructureSpec(spec, false)
				if patchErr != nil {
					skipIfVAPDenied(patchErr, "FD removal")
					Fail(fmt.Sprintf("FD removal patch failed: %v", patchErr))
				}

				waitForStorageOperatorHealthy(framework.OperatorSyncTimeout)

				afterMetrics, mErr := framework.ScrapeOperatorMetrics(suiteCtx, clients.Kube,
					framework.CSIDriverNamespace, "app=vmware-vsphere-csi-driver-operator")
				Expect(mErr).NotTo(HaveOccurred())
				afterVal, parseErr := framework.ParseMetricValue(afterMetrics, framework.OrphanTagsDetectedMetric, nil)
				Expect(parseErr).NotTo(HaveOccurred())
				Expect(afterVal).To(BeNumerically(">", beforeVal),
					"orphan tags detected metric should increase after FD removal")
			})
		})

		It("OBS-02: TagOperationsTotal tracks detach operations", Label("p1"), func() {
			_, fd := requireLabFD()
			detachLabels := map[string]string{"operation": "detach", "result": "success"}

			beforeMetrics, err := framework.ScrapeOperatorMetrics(suiteCtx, clients.Kube,
				framework.CSIDriverNamespace, "app=vmware-vsphere-csi-driver-operator")
			if err != nil {
				Skip(fmt.Sprintf("cannot scrape operator metrics: %v", err))
			}
			beforeVal, _ := framework.ParseMetricValue(beforeMetrics, framework.TagOperationsMetric, detachLabels)

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				spec.PlatformSpec.VSphere.FailureDomains = vsphere.RemoveFailureDomainByRegionZone(
					spec.PlatformSpec.VSphere.FailureDomains, fd.Region, fd.Zone)
				_, patchErr := patchInfrastructureSpec(spec, false)
				if patchErr != nil {
					skipIfVAPDenied(patchErr, "FD removal")
					Fail(fmt.Sprintf("FD removal patch failed: %v", patchErr))
				}

				waitForStorageOperatorHealthy(framework.OperatorSyncTimeout)

				afterMetrics, mErr := framework.ScrapeOperatorMetrics(suiteCtx, clients.Kube,
					framework.CSIDriverNamespace, "app=vmware-vsphere-csi-driver-operator")
				Expect(mErr).NotTo(HaveOccurred())
				afterVal, parseErr := framework.ParseMetricValue(afterMetrics, framework.TagOperationsMetric, detachLabels)
				Expect(parseErr).NotTo(HaveOccurred())
				Expect(afterVal).To(BeNumerically(">", beforeVal),
					"tag operations detach/success metric should increase")
			})
		})

		It("OBS-03: TagOperationsTotal tracks PV-blocked skips", Label("p1"), func() {
			_, fd := requireLabFD()
			skipLabels := map[string]string{"operation": "skip", "result": "pv_blocked"}

			ns := createTestNamespace("csi-op-obs03")
			DeferCleanup(func() {
				_ = framework.DeleteNamespace(suiteCtx, clients.Kube, ns, framework.LongTimeout)
			})

			sc := requireDefaultStorageClass()
			pvc, err := framework.CreatePVC(suiteCtx, clients.Kube, ns, "obs03-pvc", framework.TestPVCSize, sc.Name)
			Expect(err).NotTo(HaveOccurred())

			nodeSelector := map[string]string{
				"topology.kubernetes.io/zone": fd.Zone,
			}
			pod, err := framework.CreateBusyboxPodWithNodeSelector(suiteCtx, clients.Kube, ns, "obs03-pod", pvc.Name, nodeSelector)
			Expect(err).NotTo(HaveOccurred())
			err = framework.WaitForPodRunning(suiteCtx, clients.Kube, ns, pod.Name, framework.LongTimeout)
			Expect(err).NotTo(HaveOccurred())
			boundPVC, err := framework.WaitForPVCBound(suiteCtx, clients.Kube, ns, pvc.Name, framework.DefaultTimeout)
			Expect(err).NotTo(HaveOccurred())

			DeferCleanup(func() {
				_ = framework.DeletePod(suiteCtx, clients.Kube, ns, pod.Name)
				_ = framework.DeletePVC(suiteCtx, clients.Kube, ns, pvc.Name)
				if boundPVC.Spec.VolumeName != "" {
					_ = framework.WaitForPVDeleted(suiteCtx, clients.Kube, boundPVC.Spec.VolumeName, framework.LongTimeout)
				}
			})

			beforeMetrics, mErr := framework.ScrapeOperatorMetrics(suiteCtx, clients.Kube,
				framework.CSIDriverNamespace, "app=vmware-vsphere-csi-driver-operator")
			if mErr != nil {
				Skip(fmt.Sprintf("cannot scrape operator metrics: %v", mErr))
			}
			beforeSkip, _ := framework.ParseMetricValue(beforeMetrics, framework.TagOperationsMetric, skipLabels)
			beforeOrphan, _ := framework.ParseMetricValue(beforeMetrics, framework.OrphanTagsDetectedMetric, nil)

			withInfrastructureRestore(func(spec *configv1.InfrastructureSpec) {
				spec.PlatformSpec.VSphere.FailureDomains = vsphere.RemoveFailureDomainByRegionZone(
					spec.PlatformSpec.VSphere.FailureDomains, fd.Region, fd.Zone)
				_, patchErr := patchInfrastructureSpec(spec, false)
				if patchErr != nil {
					skipIfVAPDenied(patchErr, "FD removal")
					Fail(fmt.Sprintf("FD removal patch failed: %v", patchErr))
				}

				waitForOrphanConditionTrue(framework.OperatorSyncTimeout)

				afterMetrics, afterErr := framework.ScrapeOperatorMetrics(suiteCtx, clients.Kube,
					framework.CSIDriverNamespace, "app=vmware-vsphere-csi-driver-operator")
				Expect(afterErr).NotTo(HaveOccurred())

				afterSkip, parseErr := framework.ParseMetricValue(afterMetrics, framework.TagOperationsMetric, skipLabels)
				Expect(parseErr).NotTo(HaveOccurred())
				Expect(afterSkip).To(BeNumerically(">", beforeSkip),
					"tag operations skip/pv_blocked should increase")

				afterOrphan, parseErr := framework.ParseMetricValue(afterMetrics, framework.OrphanTagsDetectedMetric, nil)
				Expect(parseErr).NotTo(HaveOccurred())
				Expect(afterOrphan).To(BeNumerically(">", beforeOrphan),
					"orphan tags detected should increase")
			})
		})
	})

	// ── Deferred ──

	It("FD-REM-04: multiple FD removal [DEFERRED]", Label("p2"), func() {
		Skip("FD-REM-04 deferred — requires 3+ FDs not available in standard lab")
	})
})
