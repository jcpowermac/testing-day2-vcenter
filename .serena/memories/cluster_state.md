# Test Cluster State (2026-07-01)

Cluster at `jcallen@10.38.201.171`, KUBECONFIG at `$HOME/before-installer-testing/vsphere-ipi/auth/kubeconfig`.
Cluster is rebuilt frequently — details below may be stale. Verify with `kubectl` before relying on specifics.

- Primary vCenter: `vcenter-130.ci.ibmc.devcluster.openshift.com`
- Second vCenter (lab): `vcenter-120.ci.ibmc.devcluster.openshift.com`
- `VSphereMultiVCenterDay2` feature gate: enabled
- VAPs installed and enforcing (all 3: machine, cpms, machineset)
- ClusterOperators checked by waitForClusterReady: `cloud-controller-manager`, `config-operator`, `machine-api`, `storage`

## Known product issues on this cluster
- CSI driver crash-loops when multi-vCenter config lacks `topology-categories` — the CSI driver validates this at startup
- CPMS may attempt to replace control plane nodes after Infrastructure spec change if it detects config drift — replacement fails with "template not found" if RHCOS template doesn't exist on the second vCenter
- Machine region/zone labels sync asynchronously from vCenter tags via CCM — may take time after apply