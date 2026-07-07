# TOPO-02: csi-provisioner has Topology feature gate and strict-topology args

**File:** `test/e2e/csi_topology_config_test.go`
**Labels:** `readonly`, `csi-operator`, `csi-topology`, `p1`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Verifies the `csi-provisioner` sidecar container in the
`vmware-vsphere-csi-driver-controller` Deployment (`openshift-cluster-csi-drivers`)
runs with `--feature-gates=Topology=true` and `--strict-topology`, which are
required for CSI topology-aware provisioning to function.

## Actions

1. Read the `csi-provisioner` container's `Args` from the controller Deployment
2. Assert `--feature-gates=Topology=true` is present
3. Assert an arg containing `--strict-topology` is present

## Code

```go
It("TOPO-02: csi-provisioner has Topology feature gate and strict-topology args", Label("p1"), func() {
    args, err := framework.GetCSIProvisionerArgs(suiteCtx, clients.Kube)
    Expect(err).NotTo(HaveOccurred())

    Expect(args).To(ContainElement("--feature-gates=Topology=true"),
        "csi-provisioner should run with the Topology feature gate enabled")
    Expect(args).To(ContainElement(ContainSubstring("--strict-topology")),
        "csi-provisioner should run with --strict-topology")
})
```
