package e2e

import (
	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	"github.com/jcallen/testing-day2-vcenter/pkg/vsphere"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
			patch := []byte(`{"spec":{"platformSpec":{"vsphere":{"vcenters":[]}}}}`)
			expectRawPatchRejected(patch, "at least 1 items")
		})

		It("should reject removing the vcenters field once set (N-INF-04)", func() {
			patch := []byte(`{"spec":{"platformSpec":{"vsphere":{"vcenters":null}}}}`)
			expectRawPatchRejected(patch, "vcenters")
		})

		It("should reject swapping an existing vCenter server (N-INF-05)", Label("multi-vcenter"), func() {
			requireMultiVCenter()
			infra := currentInfrastructure()
			expectPatchRejected(swapSecondVCenterServer(infra, "vcenter-swapped.example.com"), "Cannot add and remove vCenters at the same time")
		})

		It("should reject simultaneous add and remove of vCenters (N-INF-06/07)", Label("multi-vcenter"), func() {
			requireMultiVCenter()
			infra := currentInfrastructure()
			expectPatchRejected(addAndRemoveVCenterSamePatch(infra), "Cannot add and remove vCenters at the same time")
		})

		It("should reject more than 3 vCenters (N-INF-11)", func() {
			infra := currentInfrastructure()
			expectPatchRejected(tooManyVCentersSpec(infra), "must have at most 3 items")
		})

		It("should reject removing a vCenter still referenced by a failure domain (N-INF-12)", Label("multi-vcenter"), func() {
			requireMultiVCenter()
			infra := currentInfrastructure()
			spec := fdReferencingRemovedVCenterSpec(infra)
			_, err := patchInfrastructureSpec(spec, true)
			if err == nil {
				Fail("CRD allows removing a vCenter that is still referenced by a failure domain — " +
					"no xValidation rule enforces FD.server must reference an existing vCenter entry")
			}
			Expect(framework.InfrastructurePatchError(err)).To(SatisfyAny(
				ContainSubstring("failure domain"),
				ContainSubstring("vCenter"),
				ContainSubstring("ValidatingAdmissionPolicy"),
			))
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
