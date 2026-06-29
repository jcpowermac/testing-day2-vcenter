package labconfig_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jcallen/testing-day2-vcenter/pkg/labconfig"
)

func TestLoadValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lab.yaml")
	content := `
secondVCenter:
  server: vc2.example.com
  datacenters: [DC2]
  username: user@example.com
  password: secret
failureDomain:
  name: fd2
  region: r2
  zone: z2
  topology:
    computeCluster: /DC2/host/c1
    datacenter: DC2
    datastore: /DC2/datastore/ds1
    networks: [net1]
    resourcePool: /DC2/host/c1/Resources
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := labconfig.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SecondVCenter.Server != "vc2.example.com" {
		t.Fatalf("server = %q", cfg.SecondVCenter.Server)
	}
}

func TestLoadRequiresServer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lab.yaml")
	if err := os.WriteFile(path, []byte("secondVCenter:\n  datacenters: [DC2]\n  username: u\n  password: p\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := labconfig.Load(path); err == nil {
		t.Fatal("expected validation error")
	}
}
