# Running Tests — Cluster Configuration Requirements

## Quick Reference

| Make Target | What Runs | Cluster Requirements | Mutates Cluster? |
|---|---|---|---|
| `make test-readonly` | 54 specs | KUBECONFIG, vSphere platform, gate enabled | No |
| `make test-p0` | P0 readonly subset | Same as readonly | No |
| `make test-mutating` | 5 specs | Same + backup/restore permission | Yes (reverts) |
| `make test-real` | 7 specs | Same + `config/lab.yaml` with real second vCenter already applied | Reads only |
| `make day2-lab` | apply → test-real → restore | Same + `config/lab.yaml` + second vCenter reachable | Yes (reverts) |

## Remote Execution

Tests run on a remote bastion host with cluster access:

```bash
# 1. Sync code
rsync -az --delete --exclude='.git' ./ jcallen@10.38.201.171:~/Development/testing-day2-vcenter/

# 2. Run (always use export + make -C)
ssh jcallen@10.38.201.171 'export KUBECONFIG=$HOME/before-installer-testing/vsphere-ipi/auth/kubeconfig && make -C ~/Development/testing-day2-vcenter test-readonly'
```

## Per-Test Configuration Details

### Always Required

- `KUBECONFIG` pointing to a vSphere-platform OpenShift cluster
- Cluster must have `Infrastructure/cluster` with `spec.platformSpec.type: VSphere`

### Feature Gate Tests

| Test | Requires |
|---|---|
| Gate-enabled tests (majority) | `VSphereMultiVCenterDay2` enabled (GA in 4.18+, always on) |
| N-INF-09, N-INF-10, VAP gate-disabled | `VSphereMultiVCenterDay2` **disabled** — requires a separate cluster or pre-4.18 build |

Gate-disabled tests will always skip on GA clusters where the gate cannot be turned off.

### VAP Tests

| Test | Requires |
|---|---|
| VAP existence | Gate enabled — checks all 3 VAPs installed |
| N-SEQ-01 (Machine VAP) | At least one Machine with `machine.openshift.io/region` and `machine.openshift.io/zone` labels matching an Infrastructure failure domain |
| N-SEQ-02 (CPMS VAP) | A CPMS with failure domain names referencing Infrastructure FDs |
| N-SEQ-03 (MachineSet VAP) | Mutating — creates a 1-replica MachineSet with region/zone labels, waits for Machine to provision, then tests VAP denial. Needs at least one existing MachineSet to clone providerSpec from |
| Scaled MachineSet VAP (mutating) | Same as N-SEQ-03 — creates a 1-replica MachineSet, waits for Machine, tests VAP denial |

### Infrastructure xValidation Tests

All require gate enabled and at least one vCenter in `Infrastructure/cluster`. The cluster currently has 2 vCenters and 2 failure domains, which satisfies all xValidation tests.

### Credential Propagation Tests

Checks these 4 secrets exist and have `<server>.username` / `<server>.password` keys for every Infrastructure vCenter:

| Secret | Namespace |
|---|---|
| `vsphere-creds` | `kube-system` |
| `vsphere-cloud-credentials` | `openshift-machine-api` |
| `vsphere-cloud-credentials` | `openshift-cloud-controller-manager` |
| `vmware-vsphere-cloud-credentials` | `openshift-cluster-csi-drivers` |

If any secret is missing, that per-secret spec skips (not a failure).

### Machine/CPMS/MachineSet Integration Tests

| Test | Requires |
|---|---|
| Machine health/labels | At least one non-deleting Machine |
| Machine FD mapping | Machines with region/zone labels + Infrastructure failure domains |
| CPMS FD names | A ControlPlaneMachineSet in `openshift-machine-api` |
| MachineSet workspace | At least one MachineSet with providerSpec |
| MachineSet labels | MachineSet with region/zone template labels (same issue as N-SEQ-03) |

### Operator Health Tests

| Test | Requires |
|---|---|
| CO Available/Degraded | ClusterOperators `cloud-controller-manager`, `config-operator`, `machine-api` |
| CCM crash loop | Pods with `k8s-app=vsphere-cloud-controller-manager` in `openshift-cloud-controller-manager` |
| MAO crash loop | Pods with `k8s-app=machine-api-operator` in `openshift-machine-api` |
| CSI crash loop | Pods with `app=vmware-vsphere-csi-driver-controller` in `openshift-cluster-csi-drivers` |

### CSI Integration Tests

| Test | Requires |
|---|---|
| CSI credential secret | `vmware-vsphere-cloud-credentials` in `openshift-cluster-csi-drivers` |
| CSI pods running | CSI driver controller pods present |
| Multi-vCenter cloud config | 2+ vCenters in Infrastructure |

### Cloud Config Tests

Require `openshift-config-managed/kube-cloud-config` and `openshift-cloud-controller-manager/cloud-conf` ConfigMaps to exist.

### ConfigMap Recreation (mutating)

Deletes `openshift-config-managed/kube-cloud-config` and waits for `config-operator` to recreate it. Requires cluster-admin and gate enabled.

### Real vCenter Tests

Require `config/lab.yaml` (or `E2E_LAB_CONFIG` env var) pointing to a valid lab config with a real second vCenter that has already been applied to the cluster via `make apply-lab`.

### vsphere-problem-detector Tests

Require the `vsphere-problem-detector-operator` deployment in `openshift-cluster-storage-operator` namespace. VPD runs under the `storage` ClusterOperator, not as its own CO.

## Expected Skip Summary (Current Cluster)

On this OCP 5.0 cluster with gate enabled, 2 vCenters, and pre-multi-FD MachineSets:

| Skipped Test | Reason | Fix |
|---|---|---|
| VAP gate-disabled | Gate is always on in GA | Needs pre-4.18 cluster |
| N-INF-09 gate-disabled | Gate is always on in GA | Needs pre-4.18 cluster |
| N-INF-10 gate-disabled | Gate is always on in GA | Needs pre-4.18 cluster |
| VPD #224 | Hardcoded skip (upstream PR) | Wait for merge |

## Expected Failures

| Failed Test | Reason | Tracker |
|---|---|---|
| N-INF-12 / N-SEQ-04 | No xValidation rule prevents removing a vCenter still referenced by a failure domain | SPLAT-2827 |
