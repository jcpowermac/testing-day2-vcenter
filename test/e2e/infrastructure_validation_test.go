package e2e

import (
	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	"github.com/jcallen/testing-day2-vcenter/pkg/vsphere"
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Infrastructure xValidation", Label("readonly", "validation", "p0"), func() {
	Context("when VSphereMultiVCenterDay2 is enabled", func() {
		BeforeEach(func() {
			requireGateEnabled()
		})

		It("should allow adding a second vCenter via dry-run", func() {
			infra := currentInfrastructure()
			spec := addSecondVCenterSpec(infra)
			expectPatchAllowedDryRun(spec)
		})

		It("should reject duplicate vCenter server values (N-INF-01/02)", func() {
			infra := currentInfrastructure()
			expectPatchRejected(duplicateFirstVCenterSpec(infra), "vcenters must have unique server values")
		})

		It("should reject reducing vcenters to an empty array (N-INF-03)", func() {
			infra := currentInfrastructure()
			expectPatchRejected(emptyVCentersSpec(infra), "at least 1 items")
		})

		It("should reject removing the vcenters field once set (N-INF-04)", func() {
			infra := currentInfrastructure()
			expectPatchRejected(removeVCentersFieldSpec(infra), "vcenters is required once set")
		})

		It("should reject swapping an existing vCenter server (N-INF-05)", func() {
			infra := currentInfrastructure()
			if len(framework.GetVCenters(infra)) < 2 {
				Skip("cluster has fewer than 2 vCenters")
			}
			expectPatchRejected(swapSecondVCenterServer(infra, "vcenter-swapped.example.com"), "Cannot add and remove vCenters at the same time")
		})

		It("should reject simultaneous add and remove of vCenters (N-INF-06/07)", func() {
			infra := currentInfrastructure()
			if len(framework.GetVCenters(infra)) < 2 {
				Skip("cluster has fewer than 2 vCenters")
			}
			expectPatchRejected(addAndRemoveVCenterSamePatch(infra), "Cannot add and remove vCenters at the same time")
		})

		It("should reject more than 3 vCenters (N-INF-11)", func() {
			infra := currentInfrastructure()
			expectPatchRejected(tooManyVCentersSpec(infra), "must have at most 3 items")
		})

		It("should reject removing a vCenter still referenced by a failure domain (N-INF-12)", func() {
			infra := currentInfrastructure()
			expectPatchRejected(fdReferencingRemovedVCenterSpec(infra), "Cannot add and remove vCenters at the same time")
		})

		It("should allow patching unrelated Infrastructure fields via dry-run (ratcheting)", func() {
			infra := currentInfrastructure()
			spec := vsphere.CloneInfrastructureSpec(infra.Spec)
			if spec.PlatformSpec.VSphere == nil {
				Skip("no vsphere platform spec")
			}
			expectPatchAllowedDryRun(&spec)
		})
	})

	Context("when VSphereMultiVCenterDay2 is disabled", func() {
		It("should reject adding a second vCenter (N-INF-09)", func() {
			requireGateDisabled()
			infra := currentInfrastructure()
			expectPatchRejected(addSecondVCenterSpec(infra), "vcenters cannot be added or removed once set")
		})

		It("should reject removing the only vCenter (N-INF-10)", func() {
			requireGateDisabled()
			infra := currentInfrastructure()
			if len(framework.GetVCenters(infra)) != 1 {
				Skip("cluster does not have exactly one vCenter")
			}
			expectPatchRejected(emptyVCentersSpec(infra), "vcenters cannot be added or removed once set")
		})
	})
})
