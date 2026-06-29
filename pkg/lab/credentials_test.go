package lab

import (
	"strings"
	"testing"

	"github.com/jcallen/testing-day2-vcenter/pkg/labconfig"
)

func TestMergeVSphereCredsSecret(t *testing.T) {
	vc := labconfig.VCenterConfig{
		Server:   "vcenter2.example.com",
		Username: "admin@vsphere.local",
		Password: "secret",
	}

	merged, err := mergeVSphereCredsSecret(map[string][]byte{
		"vcenter1.example.com.username": []byte("user1"),
		"vcenter1.example.com.password": []byte("pass1"),
	}, vc)
	if err != nil {
		t.Fatalf("mergeVSphereCredsSecret: %v", err)
	}

	if got := string(merged["vcenter1.example.com.username"]); got != "user1" {
		t.Fatalf("existing username changed: %q", got)
	}
	if got := string(merged["vcenter2.example.com.username"]); got != vc.Username {
		t.Fatalf("new username = %q, want %q", got, vc.Username)
	}
	if got := string(merged["vcenter2.example.com.password"]); got != vc.Password {
		t.Fatalf("new password = %q, want %q", got, vc.Password)
	}
}

func TestMergeCloudProviderConfigPreservesSecretRef(t *testing.T) {
	existing := `global:
  secretName: vsphere-creds
  secretNamespace: kube-system
  insecureFlag: true
vcenter:
  vcenter1.example.com:
    server: vcenter1.example.com
    port: 443
    datacenters:
      - DC1
`
	vc := labconfig.VCenterConfig{
		Server:      "vcenter2.example.com",
		Port:        443,
		Datacenters: []string{"DC2"},
		Username:    "admin@vsphere.local",
		Password:    "secret",
	}

	merged, err := mergeCloudProviderConfig(existing, nil, vc)
	if err != nil {
		t.Fatalf("mergeCloudProviderConfig: %v", err)
	}
	if strings.Contains(merged, "password: secret") {
		t.Fatalf("did not expect inline password in secret-ref config:\n%s", merged)
	}
	if !strings.Contains(merged, "vcenter2.example.com") {
		t.Fatalf("expected new vCenter in config:\n%s", merged)
	}
}

func TestMergeCloudProviderConfigInlineCredentials(t *testing.T) {
	existing := `global:
  user: admin
  password: old
vcenter:
  vcenter1.example.com:
    server: vcenter1.example.com
`
	vc := labconfig.VCenterConfig{
		Server:      "vcenter2.example.com",
		Port:        443,
		Datacenters: []string{"DC2"},
		Username:    "admin@vsphere.local",
		Password:    "secret",
	}

	merged, err := mergeCloudProviderConfig(existing, nil, vc)
	if err != nil {
		t.Fatalf("mergeCloudProviderConfig: %v", err)
	}
	if !strings.Contains(merged, "password: secret") {
		t.Fatalf("expected inline password update:\n%s", merged)
	}
}

func TestCloudConfigUsesSecretRef(t *testing.T) {
	if !cloudConfigUsesSecretRef(map[string]interface{}{"secretName": "vsphere-creds"}) {
		t.Fatal("expected secret ref detection")
	}
	if cloudConfigUsesSecretRef(map[string]interface{}{"user": "admin"}) {
		t.Fatal("did not expect secret ref for inline creds")
	}
}
