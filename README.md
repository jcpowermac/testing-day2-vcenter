# testing-day2-vcenter

QE automation for **vSphere Multi-vCenter Day 2** on OpenShift (`VSphereMultiVCenterDay2`).

This repo holds planning docs and tooling to test Day 2 against a **real second vCenter** with minimal manual steps.

## Quick start (real vCenter)

```bash
# 1. Copy and edit lab config (gitignored — contains credentials)
cp config/lab.yaml.example config/lab.yaml

# 2. Point at your cluster
export KUBECONFIG=/path/to/kubeconfig

# 3. Apply Day 2 changes (backs up cluster to .lab-state/ first)
make apply-lab

# 4. Run real-vCenter verification tests
make test-real

# 5. Restore original cluster state
make restore-lab
```

Or run the full loop:

```bash
make day2-lab CONFIG=config/lab.yaml
```

## Lab config (`config/lab.yaml`)

Fill in your real second vCenter:

```yaml
secondVCenter:
  server: vcenter2.lab.example.com
  port: 443
  datacenters:
    - DC2
  username: admin@vsphere.local
  passwordFile: /run/secrets/vcenter2-password   # or inline password

failureDomain:   # optional but recommended
  name: fd-vcenter2
  region: openshift-region-vc2
  zone: openshift-zone-vc2
  topology:
    computeCluster: /DC2/host/cluster1
    datacenter: DC2
    datastore: /DC2/datastore/default
    networks:
      - VM Network
    resourcePool: /DC2/host/cluster1/Resources
```

### What `make apply-lab` does

1. Backs up to `.lab-state/`:
   - `Infrastructure/cluster`
   - `openshift-config/cloud-provider-config`
   - Credential secrets that exist on vSphere clusters (typically `kube-system/vsphere-creds`, optionally `openshift-machine-api/vsphere-cloud-credentials`)
2. Merges credentials for the new vCenter into those secrets and `cloud-provider-config`
3. Patches `Infrastructure/cluster` to append the vCenter (and optional failure domain)
4. Waits for `cloud-controller-manager`, `config-operator`, and `machine-api` to recover

### CLI (same as Makefile)

```bash
go run ./cmd/day2-vcenter apply -config config/lab.yaml
go run ./cmd/day2-vcenter verify -config config/lab.yaml
go run ./cmd/day2-vcenter restore -config config/lab.yaml
go run ./cmd/day2-vcenter apply -config config/lab.yaml -dry-run   # validate only
```

## Prerequisites

- Go 1.25+
- Ginkgo CLI: `go install github.com/onsi/ginkgo/v2/ginkgo@v2.22.2`
- OpenShift vSphere cluster with **`VSphereMultiVCenterDay2` enabled**
- Cluster admin `KUBECONFIG`
- Real second vCenter reachable from the cluster

## Test tiers

| Command | Purpose |
|---|---|
| `make test-readonly` | CRD xValidation, VAP dry-run, ConfigMap checks (no lab config needed) |
| `make test-p0` | P0 read-only smoke |
| `make test-real` | Verify real vCenter after `make apply-lab` (requires `config/lab.yaml`) |
| `make test-mutating` | Synthetic add/remove test (skipped when lab config exists) |
| `make day2-lab` | apply → test-real → restore |

Set `CONFIG=/path/to/lab.yaml` or `E2E_LAB_CONFIG` to override the default config path.

## Project layout

```
config/lab.yaml.example   # template — copy to config/lab.yaml
cmd/day2-vcenter/         # apply / restore / verify CLI
pkg/lab/                  # cluster apply + backup logic
pkg/labconfig/            # lab YAML loader
pkg/framework/            # K8s/OpenShift helpers
test/e2e/                 # Ginkgo suites
```

## Safety

- `apply-lab` saves backups before any change; `restore-lab` reverts them
- `config/lab.yaml` and `.lab-state/` are **gitignored**
- Read-only e2e tests use server dry-run and never persist invalid patches

## Related docs

- [Test catalog](./docs/tests.md) — one-line description of every test spec
- [Coverage gap plan](./plans/coverage-gap-plan.md) — PR-to-test coverage matrix and implementation plan
- [Risk assessment](./vsphere-multi-vcenter-day2-risk-assessment.md)
- [Gap analysis](./vsphere-multi-vcenter-day2-gap-analysis.md)
- [Negative testing plan](./vsphere-multi-vcenter-day2-negative-testing.md)
- [Ginkgo test plan](./plans/ginkgo-test-plan.md)

## Verification (no cluster)

```bash
make vet build test-dry-run
```
