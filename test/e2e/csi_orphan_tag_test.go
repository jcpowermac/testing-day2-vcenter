package e2e

import (
	"fmt"
	"time"

	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	"github.com/jcallen/testing-day2-vcenter/pkg/vsphere"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Shared state for the synthetic orphan tag suite, set up once in BeforeAll.
var (
	orphanSess      *vsphere.Session
	orphanDatastore string
	orphanTagName   string
)

var _ = Describe("CSI Synthetic Orphan Tags", Serial, Ordered, Label("csi-operator", "csi-orphan", "mutating"), func() {

	BeforeAll(func() {
		lab := requireLabConfigWithFD()
		requireGateEnabled()

		id := infraID()
		orphanTagName = vsphere.GetClusterTagName(id)

		GinkgoWriter.Printf("connecting to second vCenter %s\n", lab.SecondVCenter.Server)
		orphanSess = secondVCenterSession(lab)

		catName := vsphere.GetClusterTagCategoryName(id)
		GinkgoWriter.Printf("looking up cluster tag category %q on second vCenter\n", catName)
		cat, err := vsphere.FindTagCategoryByName(suiteCtx, orphanSess, catName)
		Expect(err).NotTo(HaveOccurred())
		Expect(cat).NotTo(BeNil(),
			"tag category %q should exist on second vCenter — run make apply-lab first", catName)

		GinkgoWriter.Printf("looking up cluster tag %q in category %q\n", orphanTagName, catName)
		tag, err := vsphere.FindTagByName(suiteCtx, orphanSess, cat.ID, orphanTagName)
		Expect(err).NotTo(HaveOccurred())
		Expect(tag).NotTo(BeNil(),
			"tag %q should exist in category %q — run make apply-lab first", orphanTagName, catName)

		infra := currentInfrastructure()
		fds := framework.GetFailureDomains(infra)

		if lab.OrphanTest != nil && lab.OrphanTest.Datastore != "" {
			orphanDatastore = lab.OrphanTest.Datastore
			GinkgoWriter.Printf("using configured orphanTest.datastore=%s\n", orphanDatastore)
		} else {
			GinkgoWriter.Println("orphanTest.datastore not set in lab config — auto-discovering a non-FD datastore")
			ds, found, err := vsphere.FindNonFDDatastore(suiteCtx, orphanSess, fds)
			Expect(err).NotTo(HaveOccurred())
			if !found {
				Skip("no non-FD datastore found on second vCenter for synthetic orphan tag tests — set orphanTest.datastore in lab config")
			}
			orphanDatastore = ds
			GinkgoWriter.Printf("auto-discovered non-FD datastore=%s (verify this is safe to tag!)\n", orphanDatastore)
		}

		GinkgoWriter.Printf("pre-check: verifying %s is not already tagged\n", orphanDatastore)
		tagged, err := vsphere.IsDatastoreTagged(suiteCtx, orphanSess, orphanDatastore, orphanTagName)
		Expect(err).NotTo(HaveOccurred())
		Expect(tagged).To(BeFalse(),
			"test datastore %s should not already be tagged with %q — clean up manually before running", orphanDatastore, orphanTagName)
	})

	AfterAll(func() {
		if orphanSess == nil {
			return
		}
		GinkgoWriter.Println("AfterAll: safety-net detach of synthetic orphan tag, if still attached")
		if tagged, err := vsphere.IsDatastoreTagged(suiteCtx, orphanSess, orphanDatastore, orphanTagName); err == nil && tagged {
			_ = vsphere.DetachTagFromDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)
		}
		orphanSess.Close(suiteCtx)
	})

	It("SYNTH-01: synthetic orphan tag is detected and detached without PVs", Label("p0"), func() {
		GinkgoWriter.Printf("attaching tag %q to non-FD datastore %s\n", orphanTagName, orphanDatastore)
		Expect(vsphere.AttachTagToDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)).To(Succeed())
		DeferCleanup(func() {
			_ = vsphere.DetachTagFromDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)
		})

		waitForTagDetached(suiteCtx, orphanSess, orphanDatastore, orphanTagName, framework.OperatorSyncTimeout)

		GinkgoWriter.Println("  verifying OrphanCleanupPending stayed False — no PVs block this detach")
		waitForOrphanConditionFalse(framework.DefaultTimeout)
		waitForStorageOperatorHealthy(framework.DefaultTimeout)
		GinkgoWriter.Println("SYNTH-01: PASS")
	})

	It("SYNTH-02: orphan cleanup latency is within OperatorSyncTimeout", Label("p1"), func() {
		Expect(vsphere.AttachTagToDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)).To(Succeed())
		DeferCleanup(func() {
			_ = vsphere.DetachTagFromDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)
		})

		start := time.Now()
		waitForTagDetached(suiteCtx, orphanSess, orphanDatastore, orphanTagName, framework.OperatorSyncTimeout)
		elapsed := time.Since(start)
		GinkgoWriter.Printf("SYNTH-02: orphan tag detached in %v\n", elapsed)
		Expect(elapsed).To(BeNumerically("<", framework.OperatorSyncTimeout))
	})

	It("SYNTH-04: orphan tags detected metric increments", Label("p1"), func() {
		beforeMetrics, err := csiOperatorMetrics()
		if err != nil {
			Skip(fmt.Sprintf("cannot scrape operator metrics: %v", err))
		}
		beforeVal, _ := framework.ParseMetricValue(beforeMetrics, framework.OrphanTagsDetectedMetric, nil)
		GinkgoWriter.Printf("  %s before: %v\n", framework.OrphanTagsDetectedMetric, beforeVal)

		Expect(vsphere.AttachTagToDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)).To(Succeed())
		DeferCleanup(func() {
			_ = vsphere.DetachTagFromDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)
		})

		waitForTagDetached(suiteCtx, orphanSess, orphanDatastore, orphanTagName, framework.OperatorSyncTimeout)

		afterMetrics, err := csiOperatorMetrics()
		Expect(err).NotTo(HaveOccurred())
		afterVal, err := framework.ParseMetricValue(afterMetrics, framework.OrphanTagsDetectedMetric, nil)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  %s after: %v\n", framework.OrphanTagsDetectedMetric, afterVal)
		Expect(afterVal).To(BeNumerically(">", beforeVal),
			"orphan tags detected metric should increase after synthetic orphan tag detach")
		GinkgoWriter.Println("SYNTH-04: PASS")
	})

	It("SYNTH-05: tag operations detach/success metric increments", Label("p1"), func() {
		detachLabels := map[string]string{"operation": "detach", "result": "success"}

		beforeMetrics, err := csiOperatorMetrics()
		if err != nil {
			Skip(fmt.Sprintf("cannot scrape operator metrics: %v", err))
		}
		beforeVal, _ := framework.ParseMetricValue(beforeMetrics, framework.TagOperationsMetric, detachLabels)
		GinkgoWriter.Printf("  %s{detach,success} before: %v\n", framework.TagOperationsMetric, beforeVal)

		Expect(vsphere.AttachTagToDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)).To(Succeed())
		DeferCleanup(func() {
			_ = vsphere.DetachTagFromDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)
		})

		waitForTagDetached(suiteCtx, orphanSess, orphanDatastore, orphanTagName, framework.OperatorSyncTimeout)

		afterMetrics, err := csiOperatorMetrics()
		Expect(err).NotTo(HaveOccurred())
		afterVal, err := framework.ParseMetricValue(afterMetrics, framework.TagOperationsMetric, detachLabels)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  %s{detach,success} after: %v\n", framework.TagOperationsMetric, afterVal)
		Expect(afterVal).To(BeNumerically(">", beforeVal),
			"tag operations detach/success metric should increase")
		GinkgoWriter.Println("SYNTH-05: PASS")
	})

	It("SYNTH-09: orphan cleanup causes no side-effect damage", Label("p0"), func() {
		Expect(vsphere.AttachTagToDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)).To(Succeed())
		DeferCleanup(func() {
			_ = vsphere.DetachTagFromDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)
		})

		waitForTagDetached(suiteCtx, orphanSess, orphanDatastore, orphanTagName, framework.OperatorSyncTimeout)

		waitForStorageOperatorHealthy(framework.DefaultTimeout)
		waitForOrphanConditionFalse(framework.DefaultTimeout)

		sc := requireDefaultStorageClass()
		Expect(sc.Parameters).To(HaveKey("StoragePolicyName"),
			"default StorageClass should retain StoragePolicyName after synthetic orphan cleanup")
		GinkgoWriter.Println("SYNTH-09: PASS")
	})

	It("SYNTH-10: operator handles repeated orphans without getting stuck", Label("p1"), NodeTimeout(25*time.Minute), func() {
		GinkgoWriter.Println("  first attach/detach cycle")
		Expect(vsphere.AttachTagToDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)).To(Succeed())
		DeferCleanup(func() {
			_ = vsphere.DetachTagFromDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)
		})
		waitForTagDetached(suiteCtx, orphanSess, orphanDatastore, orphanTagName, framework.OperatorSyncTimeout)

		GinkgoWriter.Println("  second attach/detach cycle")
		Expect(vsphere.AttachTagToDatastore(suiteCtx, orphanSess, orphanDatastore, orphanTagName)).To(Succeed())

		start := time.Now()
		waitForTagDetached(suiteCtx, orphanSess, orphanDatastore, orphanTagName, framework.OperatorSyncTimeout)
		elapsed := time.Since(start)
		GinkgoWriter.Printf("SYNTH-10: second detach completed in %v\n", elapsed)
		Expect(elapsed).To(BeNumerically("<", framework.OperatorSyncTimeout),
			"repeated orphan should be cleaned up within OperatorSyncTimeout, not stuck at backoff cap")
		GinkgoWriter.Println("SYNTH-10: PASS")
	})
})
