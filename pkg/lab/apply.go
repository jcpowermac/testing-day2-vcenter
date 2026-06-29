package lab

import (
	"context"
	"fmt"
	"time"

	"github.com/jcallen/testing-day2-vcenter/pkg/framework"
	"github.com/jcallen/testing-day2-vcenter/pkg/labconfig"
	"github.com/jcallen/testing-day2-vcenter/pkg/vsphere"
	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplyOptions controls apply behavior.
type ApplyOptions struct {
	DryRun bool
}

// ApplyResult summarizes what changed.
type ApplyResult struct {
	AddedVCenter      string
	AddedFailureDomain string
	StateDir          string
}

// Apply adds the configured vCenter (and optional failure domain) to the cluster.
func Apply(ctx context.Context, clients *framework.Clients, cfg *labconfig.LabConfig, opts ApplyOptions) (*ApplyResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("lab config is nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	infra, err := framework.GetInfrastructure(ctx, clients.Config)
	if err != nil {
		return nil, err
	}
	if !framework.IsVSphereCluster(infra) {
		return nil, fmt.Errorf("cluster platform is not VSphere")
	}

	enabled, err := framework.IsFeatureGateEnabled(ctx, clients.Config, framework.VSphereMultiVCenterDay2Gate)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, fmt.Errorf("VSphereMultiVCenterDay2 feature gate is not enabled")
	}

	if vsphereHasServer(infra, cfg.SecondVCenter.Server) {
		return nil, fmt.Errorf("vCenter %q is already in Infrastructure", cfg.SecondVCenter.Server)
	}
	if len(framework.GetVCenters(infra)) >= 3 {
		return nil, fmt.Errorf("Infrastructure already has 3 vCenters (OpenShift maximum)")
	}

	if !opts.DryRun && !HasState(cfg.StateDir) {
		state, err := captureState(ctx, clients, infra)
		if err != nil {
			return nil, fmt.Errorf("backup cluster state: %w", err)
		}
		if err := SaveState(cfg.StateDir, state); err != nil {
			return nil, err
		}
	}

	if err := updateCredentials(ctx, clients, infra, cfg, opts.DryRun); err != nil {
		return nil, fmt.Errorf("update credentials: %w", err)
	}

	newSpec, fdName, err := buildInfrastructureSpec(infra, cfg)
	if err != nil {
		return nil, err
	}

	if opts.DryRun {
		_, err = framework.ReplaceInfrastructureSpec(ctx, clients.Config, newSpec, true)
		if err != nil {
			return nil, fmt.Errorf("dry-run infrastructure patch: %w", err)
		}
	} else {
		_, err = framework.ReplaceInfrastructureSpec(ctx, clients.Config, newSpec, false)
		if err != nil {
			return nil, fmt.Errorf("apply infrastructure patch: %w", err)
		}
		if err := waitForOperators(ctx, clients); err != nil {
			return nil, err
		}
	}

	return &ApplyResult{
		AddedVCenter:       cfg.SecondVCenter.Server,
		AddedFailureDomain: fdName,
		StateDir:           cfg.StateDir,
	}, nil
}

// Restore reverts cluster objects from saved state.
func Restore(ctx context.Context, clients *framework.Clients, stateDir string) error {
	state, err := LoadState(stateDir)
	if err != nil {
		return err
	}

	if state.Infrastructure != nil {
		if err := framework.RestoreInfrastructure(ctx, clients.Config, state.Infrastructure); err != nil {
			return fmt.Errorf("restore infrastructure: %w", err)
		}
	}

	for key, cm := range state.ConfigMaps {
		ns, name, err := splitKey(key)
		if err != nil {
			return err
		}
		current, getErr := framework.GetConfigMap(ctx, clients.Kube, ns, name)
		if getErr != nil {
			return fmt.Errorf("get configmap %s/%s: %w", ns, name, getErr)
		}
		restored := current.DeepCopy()
		restored.Data = cm.Data
		restored.BinaryData = cm.BinaryData
		if _, err := clients.Kube.CoreV1().ConfigMaps(ns).Update(ctx, restored, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("restore configmap %s/%s: %w", ns, name, err)
		}
	}

	for key, secret := range state.Secrets {
		ns, name, err := splitKey(key)
		if err != nil {
			return err
		}
		current, getErr := clients.Kube.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
		if getErr != nil {
			return fmt.Errorf("get secret %s/%s: %w", ns, name, getErr)
		}
		restored := current.DeepCopy()
		restored.Data = secret.Data
		restored.StringData = nil
		if _, err := clients.Kube.CoreV1().Secrets(ns).Update(ctx, restored, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("restore secret %s/%s: %w", ns, name, err)
		}
	}

	return waitForOperators(ctx, clients)
}

// Verify checks operators and cloud config include the configured vCenter.
func Verify(ctx context.Context, clients *framework.Clients, cfg *labconfig.LabConfig) error {
	if err := framework.CheckOperatorsNotDegraded(ctx, clients.Config, []string{
		"cloud-controller-manager",
		"cluster-config-operator",
		"machine-api",
	}); err != nil {
		return err
	}

	infra, err := framework.GetInfrastructure(ctx, clients.Config)
	if err != nil {
		return err
	}
	if !vsphereHasServer(infra, cfg.SecondVCenter.Server) {
		return fmt.Errorf("Infrastructure does not include vCenter %q", cfg.SecondVCenter.Server)
	}

	cm, err := framework.GetConfigMap(ctx, clients.Kube, framework.ManagedConfigNamespace, framework.ManagedConfigName)
	if err != nil {
		return err
	}
	parsed, err := vsphere.ParseCloudConfigYAML(cm.Data[framework.CloudConfigDataKey])
	if err != nil {
		return err
	}
	return vsphere.AssertInfrastructureVCentersPresent(infra, parsed)
}

func captureState(ctx context.Context, clients *framework.Clients, infra *configv1.Infrastructure) (*ClusterState, error) {
	state := &ClusterState{
		Infrastructure: infra.DeepCopy(),
		ConfigMaps:     map[string]corev1.ConfigMap{},
		Secrets:        map[string]corev1.Secret{},
	}

	cm, err := framework.GetConfigMap(ctx, clients.Kube, framework.SourceConfigNamespace, framework.SourceConfigName)
	if err != nil {
		return nil, fmt.Errorf("get %s/%s: %w", framework.SourceConfigNamespace, framework.SourceConfigName, err)
	}
	state.ConfigMaps[objectKey(framework.SourceConfigNamespace, framework.SourceConfigName)] = *cm.DeepCopy()

	secrets, err := backupCredentialSecrets(ctx, clients.Kube)
	if err != nil {
		return nil, err
	}
	state.Secrets = secrets
	return state, nil
}

func updateCredentials(ctx context.Context, clients *framework.Clients, infra *configv1.Infrastructure, cfg *labconfig.LabConfig, dryRun bool) error {
	cm, err := framework.GetConfigMap(ctx, clients.Kube, framework.SourceConfigNamespace, framework.SourceConfigName)
	if err != nil {
		return err
	}
	mergedConfig, err := mergeCloudProviderConfig(cm.Data[framework.CloudConfigDataKey], infra, cfg.SecondVCenter)
	if err != nil {
		return err
	}

	if dryRun {
		return updateCredentialSecrets(ctx, clients.Kube, cfg.SecondVCenter, true)
	}

	cmUpdate := cm.DeepCopy()
	if cmUpdate.Data == nil {
		cmUpdate.Data = map[string]string{}
	}
	cmUpdate.Data[framework.CloudConfigDataKey] = mergedConfig
	if _, err := clients.Kube.CoreV1().ConfigMaps(cmUpdate.Namespace).Update(ctx, cmUpdate, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("update cloud-provider-config: %w", err)
	}

	return updateCredentialSecrets(ctx, clients.Kube, cfg.SecondVCenter, false)
}

func buildInfrastructureSpec(infra *configv1.Infrastructure, cfg *labconfig.LabConfig) (*configv1.InfrastructureSpec, string, error) {
	spec := vsphere.CloneInfrastructureSpec(infra.Spec)
	if spec.PlatformSpec.VSphere == nil {
		return nil, "", fmt.Errorf("infrastructure has no vsphere platform spec")
	}

	entry := configv1.VSpherePlatformVCenterSpec{
		Server:      cfg.SecondVCenter.Server,
		Port:        cfg.SecondVCenter.Port,
		Datacenters: append([]string(nil), cfg.SecondVCenter.Datacenters...),
	}
	spec.PlatformSpec.VSphere.VCenters = append(spec.PlatformSpec.VSphere.VCenters, entry)

	fdName := ""
	if cfg.FailureDomain != nil {
		fd := configv1.VSpherePlatformFailureDomainSpec{
			Name:   cfg.FailureDomain.Name,
			Region: cfg.FailureDomain.Region,
			Zone:   cfg.FailureDomain.Zone,
			Server: cfg.SecondVCenter.Server,
			Topology: configv1.VSpherePlatformTopology{
				ComputeCluster: cfg.FailureDomain.Topology.ComputeCluster,
				Datacenter:     cfg.FailureDomain.Topology.Datacenter,
				Datastore:      cfg.FailureDomain.Topology.Datastore,
				Networks:       append([]string(nil), cfg.FailureDomain.Topology.Networks...),
				ResourcePool:   cfg.FailureDomain.Topology.ResourcePool,
			},
		}
		spec.PlatformSpec.VSphere.FailureDomains = append(spec.PlatformSpec.VSphere.FailureDomains, fd)
		fdName = fd.Name
	}

	return &spec, fdName, nil
}

func vsphereHasServer(infra *configv1.Infrastructure, server string) bool {
	for _, vc := range framework.GetVCenters(infra) {
		if vc.Server == server {
			return true
		}
	}
	return false
}

func waitForOperators(ctx context.Context, clients *framework.Clients) error {
	for _, name := range []string{"cloud-controller-manager", "cluster-config-operator", "machine-api"} {
		if err := framework.WaitForClusterOperatorAvailable(ctx, clients.Config, name, framework.DefaultTimeout); err != nil {
			return fmt.Errorf("wait for clusteroperator %q: %w", name, err)
		}
	}
	// Allow CCCMO to reconcile cloud config after Infrastructure change.
	time.Sleep(15 * time.Second)
	return nil
}

func objectKey(namespace, name string) string {
	return namespace + "/" + name
}

func splitKey(key string) (namespace, name string, err error) {
	for i := 0; i < len(key); i++ {
		if key[i] == '/' {
			return key[:i], key[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("invalid object key %q", key)
}
