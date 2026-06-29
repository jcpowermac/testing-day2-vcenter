package framework

import (
	"context"
	"encoding/json"
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const infrastructureFieldManager = "testing-day2-vcenter-e2e"

// GetInfrastructure returns the cluster Infrastructure CR.
func GetInfrastructure(ctx context.Context, client configclient.Interface) (*configv1.Infrastructure, error) {
	infra, err := client.ConfigV1().Infrastructures().Get(ctx, InfrastructureName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get infrastructure: %w", err)
	}
	return infra, nil
}

// BackupInfrastructure deep-copies the current Infrastructure CR.
func BackupInfrastructure(ctx context.Context, client configclient.Interface) (*configv1.Infrastructure, error) {
	infra, err := GetInfrastructure(ctx, client)
	if err != nil {
		return nil, err
	}
	return infra.DeepCopy(), nil
}

// RestoreInfrastructure replaces spec and status with the backup copy.
func RestoreInfrastructure(ctx context.Context, client configclient.Interface, backup *configv1.Infrastructure) error {
	if backup == nil {
		return fmt.Errorf("backup infrastructure is nil")
	}

	current, err := GetInfrastructure(ctx, client)
	if err != nil {
		return err
	}

	restored := current.DeepCopy()
	restored.Spec = *backup.Spec.DeepCopy()
	restored.Status = *backup.Status.DeepCopy()

	_, err = client.ConfigV1().Infrastructures().Update(ctx, restored, metav1.UpdateOptions{
		FieldManager: infrastructureFieldManager,
	})
	if err != nil {
		return fmt.Errorf("restore infrastructure: %w", err)
	}
	return nil
}

// PatchInfrastructure applies a merge patch to Infrastructure/cluster.
// When dryRun is true the API server validates without persisting changes.
func PatchInfrastructure(ctx context.Context, client configclient.Interface, patch []byte, dryRun bool) (*configv1.Infrastructure, error) {
	opts := metav1.PatchOptions{FieldManager: infrastructureFieldManager}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}

	infra, err := client.ConfigV1().Infrastructures().Patch(
		ctx,
		InfrastructureName,
		types.MergePatchType,
		patch,
		opts,
	)
	if err != nil {
		return nil, err
	}
	return infra, nil
}

// ReplaceInfrastructureSpec replaces the entire spec via merge patch.
func ReplaceInfrastructureSpec(ctx context.Context, client configclient.Interface, spec *configv1.InfrastructureSpec, dryRun bool) (*configv1.Infrastructure, error) {
	payload := map[string]interface{}{
		"spec": spec,
	}
	patch, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal infrastructure spec patch: %w", err)
	}
	return PatchInfrastructure(ctx, client, patch, dryRun)
}

// IsVSphereCluster reports whether the cluster platform is VSphere.
func IsVSphereCluster(infra *configv1.Infrastructure) bool {
	if infra == nil {
		return false
	}
	if infra.Status.PlatformStatus != nil {
		return infra.Status.PlatformStatus.Type == configv1.VSpherePlatformType
	}
	return infra.Spec.PlatformSpec.Type == configv1.VSpherePlatformType
}

// GetVCenters returns vCenter entries from the Infrastructure spec.
func GetVCenters(infra *configv1.Infrastructure) []configv1.VSpherePlatformVCenterSpec {
	if infra == nil || infra.Spec.PlatformSpec.VSphere == nil {
		return nil
	}
	return infra.Spec.PlatformSpec.VSphere.VCenters
}

// GetFailureDomains returns failure domain entries from the Infrastructure spec.
func GetFailureDomains(infra *configv1.Infrastructure) []configv1.VSpherePlatformFailureDomainSpec {
	if infra == nil || infra.Spec.PlatformSpec.VSphere == nil {
		return nil
	}
	return infra.Spec.PlatformSpec.VSphere.FailureDomains
}

// InfrastructurePatchError returns a human-readable error for patch failures.
func InfrastructurePatchError(err error) string {
	if err == nil {
		return ""
	}
	if status, ok := err.(*apierrors.StatusError); ok {
		return status.ErrStatus.Message
	}
	return err.Error()
}
