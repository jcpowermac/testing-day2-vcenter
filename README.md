# testing-day2-vcenter

QE automation for **vSphere Multi-vCenter Day 2** on OpenShift (`VSphereMultiVCenterDay2`).

This repo holds planning docs and a Ginkgo/Gomega e2e suite that validates the Day 2 feature against a live vSphere cluster. Tests focus on CRD xValidation, ValidatingAdmissionPolicies, ConfigMap ownership migration (CCO → CCCMO), cloud config parity, and operator health.

## Background

The Day 2 feature spans 12 PRs across 7 repos (API, CCCMO, CCO, installer, library-go, MAO, problem detector). See the [risk assessment](./vsphere-multi-vcenter-day2-risk-assessment.md) for scope and priority.

| Doc | Purpose |
|---|---|
| [Risk assessment](./vsphere-multi-vcenter-day2-risk-assessment.md) | PR-by-pr risk and P0 matrix |
| [Gap analysis](./vsphere-multi-vcenter-day2-gap-analysis.md) | Coverage holes vs risk assessment |
| [Negative testing plan](./vsphere-multi-vcenter-day2-negative-testing.md) | QA/QE negative case catalog |
| [Ginkgo test plan](./plans/ginkgo-test-plan.md) | E2E implementation design + QE review |

## Prerequisites

- Go 1.25+
- [Ginkgo v2 CLI](https://onsi.github.io/ginkgo/) (`go install github.com/onsi/ginkgo/v2/ginkgo@v2.22.2`)
- `KUBECONFIG` for an OpenShift cluster on **vSphere**
- For most tests: `VSphereMultiVCenterDay2` enabled (e.g. TechPreview / DevPreview profile)

## Quick start

```bash
# Build and lint
make vet build

# Compile test tree (no cluster required)
make test-dry-run

# Safe default against a lab cluster
export KUBECONFIG=/path/to/kubeconfig
make test-readonly
```

## Running tests

### Makefile targets

| Target | Description |
|---|---|
| `make vet` | Run `go vet ./...` |
| `make build` | Build all packages |
| `make test-dry-run` | Compile and list specs (no cluster) |
| `make test-readonly` | Read-only + server dry-run validation |
| `make test-p0` | P0 read-only smoke |
| `make test-mutating` | Infrastructure mutations with backup/restore |

### Ginkgo label filters

```bash
ginkgo --label-filter="readonly" ./test/e2e/           # safe default
ginkgo --label-filter="p0 && readonly" ./test/e2e/    # PR smoke
ginkgo --label-filter="mutating" ./test/e2e/           # opt-in lifecycle tests
ginkgo --label-filter="p0" ./test/e2e/                 # nightly (includes mutating)
```

**Labels:** `readonly`, `mutating`, `validation`, `admission`, `operator`, `config`, `p0`, `p1`, `p2`

### CI tiers

| Tier | Filter | When |
|---|---|---|
| PR smoke | `p0 && readonly` | Every PR |
| Nightly | `p0` | Dedicated vSphere lab (includes `mutating`) |
| Manual QA | See [negative testing plan](./vsphere-multi-vcenter-day2-negative-testing.md) | Pre-release |

## Test coverage (37 specs)

| Suite | File | What it checks |
|---|---|---|
| Feature gate | `featuregate_test.go` | `VSphereMultiVCenterDay2` on `FeatureGate/cluster` |
| xValidation | `infrastructure_validation_test.go` | N-INF-* CRD rules (server-side dry-run) |
| VAP | `vap_test.go` | Failure-domain admission policies (N-SEQ-*) |
| ConfigMap ownership | `configmap_ownership_test.go` | Managed `kube-cloud-config`, steady-state writer |
| ConfigMap content | `configmap_content_test.go` | YAML parity, stale vCenter (#469), node CIDR |
| Operator health | `operator_health_test.go` | CO: cloud-controller-manager, cluster-config-operator, machine-api |
| Problem detector | `problem_detector_test.go` | CO health stub (full tests when #224 merges) |
| Topology lifecycle | `topology_lifecycle_test.go` | Mutating add/remove vCenter with restore |

## Safety model

Tests are designed to be **non-destructive by default**:

- **Server-side dry-run** (`dryRun=All`) for xValidation and most admission denial cases — nothing persisted
- **Read-only** observation for ConfigMaps, operators, feature gates, VAP existence
- **Mutating** tests (`mutating` label) patch `Infrastructure/cluster` only with `BeforeSuite` backup and `DeferCleanup` restore
- VAP denials: suite probes dry-run at startup; falls back to real patch when admission still rejects without persisting invalid state

## Project layout

```
testing-day2-vcenter/
├── pkg/
│   ├── framework/     # K8s/OpenShift clients, Infrastructure/ConfigMap/CO helpers
│   └── vsphere/       # Cloud config YAML parsing and fixture helpers
├── test/e2e/          # Ginkgo suites
├── plans/             # Implementation and QE review notes
├── Makefile
└── go.mod
```

## Out of scope for automation

These remain **manual QA** (see [negative testing plan](./vsphere-multi-vcenter-day2-negative-testing.md)):

- Operator version skew during CVO upgrade (N-OP-01/02)
- Cloud credential negative paths (N-CFG-04/05)
- Feature gate rollback after migration (N-OP-03)
- Environmental chaos / degraded API (N-ENV-*)
- Hypershift, CSI, Routes load tests

## Verification

```bash
go vet ./...
go build ./...
ginkgo --dry-run ./test/e2e/
```

Expected: 37 specs compile; live runs require a vSphere cluster and skip gracefully when preconditions are not met.
