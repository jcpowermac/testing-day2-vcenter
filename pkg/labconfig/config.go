package labconfig

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigPath = "config/lab.yaml"
	DefaultStateDir   = ".lab-state"
)

// LabConfig describes the second (or additional) vCenter to add for Day 2 testing.
type LabConfig struct {
	StateDir string `yaml:"stateDir,omitempty"`

	SecondVCenter VCenterConfig `yaml:"secondVCenter"`

	// FailureDomain is optional. When set, a failure domain is added pointing at secondVCenter.
	FailureDomain *FailureDomainConfig `yaml:"failureDomain,omitempty"`
}

// VCenterConfig is connection info for one vCenter.
type VCenterConfig struct {
	Server      string   `yaml:"server"`
	Port        int32    `yaml:"port,omitempty"`
	Datacenters []string `yaml:"datacenters"`
	Username    string   `yaml:"username"`
	Password    string   `yaml:"password,omitempty"`
	PasswordFile string  `yaml:"passwordFile,omitempty"`
}

// FailureDomainConfig mirrors Infrastructure failure domain fields needed for Day 2.
type FailureDomainConfig struct {
	Name   string         `yaml:"name"`
	Region string         `yaml:"region"`
	Zone   string         `yaml:"zone"`
	Topology TopologyConfig `yaml:"topology"`
}

// TopologyConfig is vSphere topology for a failure domain.
type TopologyConfig struct {
	ComputeCluster string   `yaml:"computeCluster"`
	Datacenter     string   `yaml:"datacenter"`
	Datastore      string   `yaml:"datastore"`
	Networks       []string `yaml:"networks"`
	ResourcePool   string   `yaml:"resourcePool"`
	Template       string   `yaml:"template,omitempty"`
}

// Load reads and validates lab config from path.
func Load(path string) (*LabConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	cfg := &LabConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.StateDir == "" {
		cfg.StateDir = DefaultStateDir
	}
	return cfg, nil
}

// LoadFromEnv loads config from E2E_LAB_CONFIG or CONFIG env, then default path.
func LoadFromEnv() (*LabConfig, string, error) {
	path := os.Getenv("E2E_LAB_CONFIG")
	if path == "" {
		path = os.Getenv("CONFIG")
	}
	if path == "" {
		path = DefaultConfigPath
	}
	if _, err := os.Stat(path); err != nil {
		return nil, path, fmt.Errorf("lab config not found at %q (copy config/lab.yaml.example to config/lab.yaml)", path)
	}
	cfg, err := Load(path)
	return cfg, path, err
}

// Validate checks required fields.
func (c *LabConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if err := c.SecondVCenter.Validate(); err != nil {
		return fmt.Errorf("secondVCenter: %w", err)
	}
	if c.FailureDomain != nil {
		if err := c.FailureDomain.Validate(c.SecondVCenter.Server); err != nil {
			return fmt.Errorf("failureDomain: %w", err)
		}
	}
	return nil
}

// Validate checks vCenter fields.
func (v *VCenterConfig) Validate() error {
	if v == nil {
		return fmt.Errorf("is nil")
	}
	v.Server = strings.TrimSpace(v.Server)
	if v.Server == "" {
		return fmt.Errorf("server is required")
	}
	if len(v.Datacenters) == 0 {
		return fmt.Errorf("at least one datacenter is required")
	}
	if strings.TrimSpace(v.Username) == "" {
		return fmt.Errorf("username is required")
	}
	password, err := v.PasswordValue()
	if err != nil {
		return err
	}
	if password == "" {
		return fmt.Errorf("password or passwordFile is required")
	}
	if v.Port == 0 {
		v.Port = 443
	}
	return nil
}

// PasswordValue returns password from inline value or file.
func (v *VCenterConfig) PasswordValue() (string, error) {
	if v.Password != "" {
		return v.Password, nil
	}
	if v.PasswordFile == "" {
		return "", nil
	}
	data, err := os.ReadFile(v.PasswordFile)
	if err != nil {
		return "", fmt.Errorf("read passwordFile: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// Validate checks failure domain fields.
func (f *FailureDomainConfig) Validate(vcenterServer string) error {
	if strings.TrimSpace(f.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(f.Region) == "" {
		return fmt.Errorf("region is required")
	}
	if strings.TrimSpace(f.Zone) == "" {
		return fmt.Errorf("zone is required")
	}
	if strings.TrimSpace(f.Topology.Datacenter) == "" {
		return fmt.Errorf("topology.datacenter is required")
	}
	if strings.TrimSpace(f.Topology.ComputeCluster) == "" {
		return fmt.Errorf("topology.computeCluster is required")
	}
	if strings.TrimSpace(f.Topology.Datastore) == "" {
		return fmt.Errorf("topology.datastore is required")
	}
	if len(f.Topology.Networks) == 0 {
		return fmt.Errorf("topology.networks requires at least one entry")
	}
	if strings.TrimSpace(f.Topology.ResourcePool) == "" {
		return fmt.Errorf("topology.resourcePool is required")
	}
	_ = vcenterServer
	return nil
}
