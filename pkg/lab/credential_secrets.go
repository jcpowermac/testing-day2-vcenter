package lab

import (
	"context"
	"fmt"

	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	"github.com/jcallen/testing-day2-vcenter/pkg/labconfig"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type credentialSecretFormat int

const (
	credentialFormatPerVCenter credentialSecretFormat = iota
	credentialFormatYAML
)

type credentialSecretTarget struct {
	Namespace string
	Name      string
	Format    credentialSecretFormat
}

var vsphereCredentialSecretTargets = []credentialSecretTarget{
	{Namespace: framework.VSphereCredsNamespace, Name: framework.VSphereCredsSecret, Format: credentialFormatPerVCenter},
	{Namespace: framework.SourceConfigNamespace, Name: framework.CloudCredentialsSecret, Format: credentialFormatYAML},
	{Namespace: framework.MachineAPINamespace, Name: framework.VSphereMachineCredsSecret, Format: credentialFormatPerVCenter},
}

func backupCredentialSecrets(ctx context.Context, kube kubernetes.Interface) (map[string]corev1.Secret, error) {
	secrets := map[string]corev1.Secret{}
	for _, target := range vsphereCredentialSecretTargets {
		secret, err := kube.CoreV1().Secrets(target.Namespace).Get(ctx, target.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get %s/%s: %w", target.Namespace, target.Name, err)
		}
		secrets[objectKey(target.Namespace, target.Name)] = *secret.DeepCopy()
	}
	return secrets, nil
}

func mergeCredentialSecretData(format credentialSecretFormat, data map[string][]byte, vc labconfig.VCenterConfig) (map[string][]byte, error) {
	switch format {
	case credentialFormatYAML:
		return mergeCloudCredentialsSecret(data, vc)
	default:
		return mergeVSphereCredsSecret(data, vc)
	}
}

func updateCredentialSecrets(ctx context.Context, kube kubernetes.Interface, vc labconfig.VCenterConfig, dryRun bool) error {
	for _, target := range vsphereCredentialSecretTargets {
		secret, err := kube.CoreV1().Secrets(target.Namespace).Get(ctx, target.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("get %s/%s: %w", target.Namespace, target.Name, err)
		}

		merged, err := mergeCredentialSecretData(target.Format, secret.Data, vc)
		if err != nil {
			return fmt.Errorf("merge %s/%s: %w", target.Namespace, target.Name, err)
		}
		if dryRun {
			continue
		}

		update := secret.DeepCopy()
		update.Data = merged
		if _, err := kube.CoreV1().Secrets(update.Namespace).Update(ctx, update, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update %s/%s: %w", target.Namespace, target.Name, err)
		}
	}
	return nil
}
