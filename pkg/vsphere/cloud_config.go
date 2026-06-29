package vsphere

import (
	"fmt"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	"gopkg.in/yaml.v3"
)

// CloudConfig represents the subset of vSphere cloud provider YAML used in tests.
type CloudConfig struct {
	Global  map[string]interface{}            `yaml:"global,omitempty"`
	Labels  map[string]string                 `yaml:"labels,omitempty"`
	VCenter map[string]VCenterConfig          `yaml:"vcenter,omitempty"`
	Nodes   *NodesConfig                      `yaml:"nodes,omitempty"`
	Extra   map[string]interface{}            `yaml:",inline"`
}

// VCenterConfig holds per-vCenter settings in cloud config YAML.
type VCenterConfig struct {
	Server     string   `yaml:"server,omitempty"`
	Port       int32    `yaml:"port,omitempty"`
	Datacenters []string `yaml:"datacenters,omitempty"`
	Insecure   bool     `yaml:"insecure-flag,omitempty"`
}

// NodesConfig holds node network settings when present.
type NodesConfig struct {
	ExternalNetworkSubnetCidr string `yaml:"externalNetworkSubnetCidr,omitempty"`
	InternalNetworkSubnetCidr string `yaml:"internalNetworkSubnetCidr,omitempty"`
}

// ParseCloudConfigYAML parses vSphere cloud provider config YAML/INI-like YAML content.
func ParseCloudConfigYAML(data string) (*CloudConfig, error) {
	if strings.TrimSpace(data) == "" {
		return nil, fmt.Errorf("cloud config is empty")
	}

	cfg := &CloudConfig{}
	if err := yaml.Unmarshal([]byte(data), cfg); err != nil {
		return nil, fmt.Errorf("parse cloud config yaml: %w", err)
	}
	return cfg, nil
}

// VCenterServersFromConfig returns server names from parsed cloud config.
func VCenterServersFromConfig(cfg *CloudConfig) []string {
	if cfg == nil || len(cfg.VCenter) == 0 {
		return nil
	}
	servers := make([]string, 0, len(cfg.VCenter))
	for name, vc := range cfg.VCenter {
		if vc.Server != "" {
			servers = append(servers, vc.Server)
			continue
		}
		servers = append(servers, name)
	}
	return servers
}

// AssertInfrastructureVCentersPresent verifies all Infrastructure vCenter servers appear in config.
func AssertInfrastructureVCentersPresent(infra *configv1.Infrastructure, cfg *CloudConfig) error {
	if infra == nil || infra.Spec.PlatformSpec.VSphere == nil {
		return fmt.Errorf("infrastructure has no vsphere platform spec")
	}
	if cfg == nil {
		return fmt.Errorf("cloud config is nil")
	}

	configServers := VCenterServersFromConfig(cfg)
	configSet := make(map[string]struct{}, len(configServers))
	for _, s := range configServers {
		configSet[s] = struct{}{}
	}
	for _, vc := range infra.Spec.PlatformSpec.VSphere.VCenters {
		if _, ok := configSet[vc.Server]; !ok {
			// Also accept vCenter keyed by server hostname in map key.
			if _, keyed := cfg.VCenter[vc.Server]; !keyed {
				return fmt.Errorf("vCenter %q missing from cloud config", vc.Server)
			}
		}
	}
	return nil
}

// AssertNoStaleVCenters fails if config contains servers not in Infrastructure spec.
func AssertNoStaleVCenters(infra *configv1.Infrastructure, cfg *CloudConfig) error {
	if infra == nil || infra.Spec.PlatformSpec.VSphere == nil {
		return fmt.Errorf("infrastructure has no vsphere platform spec")
	}
	if cfg == nil {
		return fmt.Errorf("cloud config is nil")
	}

	allowed := make(map[string]struct{})
	for _, vc := range infra.Spec.PlatformSpec.VSphere.VCenters {
		allowed[vc.Server] = struct{}{}
	}

	for key, vc := range cfg.VCenter {
		server := vc.Server
		if server == "" {
			server = key
		}
		if _, ok := allowed[server]; !ok {
			return fmt.Errorf("stale vCenter %q present in cloud config", server)
		}
	}
	return nil
}

// GlobalInsecureOnly returns true when insecure-flag is only set globally, not per vCenter.
func GlobalInsecureOnly(cfg *CloudConfig) bool {
	if cfg == nil {
		return true
	}
	for _, vc := range cfg.VCenter {
		if vc.Insecure {
			return false
		}
	}
	return true
}
