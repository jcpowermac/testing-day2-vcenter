# TOPO-03: internal feature states configmap has improved-volume-topology enabled

**File:** `test/e2e/csi_topology_config_test.go`
**Labels:** `readonly`, `csi-operator`, `csi-topology`, `p1`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Verifies the ConfigMap `internal-feature-states.csi.vsphere.vmware.com`
(`openshift-cluster-csi-drivers`) has data key `improved-volume-topology` set
to `"true"`, which the vSphere CSI driver reads at startup to enable improved
topology-aware volume provisioning.

## Actions

1. Read the internal feature states ConfigMap
2. Assert `data["improved-volume-topology"] == "true"`

## Code

```go
It("TOPO-03: internal feature states configmap has improved-volume-topology enabled", Label("p1"), func() {
    data, err := framework.GetFeatureConfigMapData(suiteCtx, clients.Kube)
    Expect(err).NotTo(HaveOccurred())

    Expect(data).To(HaveKeyWithValue("improved-volume-topology", "true"))
})
```
