package lab

import (
	"fmt"
	"strings"

	"github.com/jcallen/testing-day2-vcenter/pkg/labconfig"
	"github.com/jcallen/testing-day2-vcenter/pkg/vsphere"
	configv1 "github.com/openshift/api/config/v1"
	"gopkg.in/yaml.v3"
)

type vsphereCredsFile struct {
	Credentials []vsphereCredential `yaml:"credentials"`
}

type vsphereCredential struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func mergeCloudProviderConfig(existing string, infra *configv1.Infrastructure, vc labconfig.VCenterConfig) (string, error) {
	password, err := vc.PasswordValue()
	if err != nil {
		return "", err
	}

	cfg, err := vsphere.ParseCloudConfigYAML(existing)
	if err != nil {
		cfg = &vsphere.CloudConfig{
			Global:  map[string]interface{}{},
			VCenter: map[string]vsphere.VCenterConfig{},
		}
		if infra != nil && infra.Spec.PlatformSpec.VSphere != nil {
			for _, existingVC := range infra.Spec.PlatformSpec.VSphere.VCenters {
				cfg.VCenter[existingVC.Server] = vsphere.VCenterConfig{
					Server:      existingVC.Server,
					Port:        existingVC.Port,
					Datacenters: append([]string(nil), existingVC.Datacenters...),
				}
			}
		}
	}

	if cfg.Global == nil {
		cfg.Global = map[string]interface{}{}
	}
	if cfg.VCenter == nil {
		cfg.VCenter = map[string]vsphere.VCenterConfig{}
	}

	cfg.Global["user"] = vc.Username
	cfg.Global["password"] = password
	cfg.VCenter[vc.Server] = vsphere.VCenterConfig{
		Server:      vc.Server,
		Port:        vc.Port,
		Datacenters: append([]string(nil), vc.Datacenters...),
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal cloud config: %w", err)
	}
	return string(out), nil
}

func mergeCloudCredentialsSecret(data map[string][]byte, vc labconfig.VCenterConfig) (map[string][]byte, error) {
	password, err := vc.PasswordValue()
	if err != nil {
		return nil, err
	}

	out := make(map[string][]byte, len(data))
	for k, v := range data {
		out[k] = append([]byte(nil), v...)
	}

	key := pickCredsKey(out)
	creds := vsphereCredsFile{}
	if len(out[key]) > 0 {
		if err := yaml.Unmarshal(out[key], &creds); err != nil {
			return nil, fmt.Errorf("parse secret key %q: %w", key, err)
		}
	}

	replaced := false
	for i := range creds.Credentials {
		if normalizeURL(creds.Credentials[i].URL) == normalizeURL(vc.Server) {
			creds.Credentials[i].Username = vc.Username
			creds.Credentials[i].Password = password
			replaced = true
		}
	}
	if !replaced {
		creds.Credentials = append(creds.Credentials, vsphereCredential{
			URL:      vc.Server,
			Username: vc.Username,
			Password: password,
		})
	}

	encoded, err := yaml.Marshal(creds)
	if err != nil {
		return nil, fmt.Errorf("marshal vsphere creds: %w", err)
	}
	out[key] = encoded
	return out, nil
}

func pickCredsKey(data map[string][]byte) string {
	for _, candidate := range []string{"vsphere-creds.yaml", "credentials.yaml", "creds.yaml"} {
		if _, ok := data[candidate]; ok {
			return candidate
		}
	}
	for k := range data {
		if strings.HasSuffix(k, ".yaml") || strings.HasSuffix(k, ".yml") {
			return k
		}
	}
	return "vsphere-creds.yaml"
}

func normalizeURL(url string) string {
	return strings.TrimSuffix(strings.TrimSpace(strings.ToLower(url)), "/")
}
