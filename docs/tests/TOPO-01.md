# TOPO-01: CSI config secret topology-categories matches discovered topology keys

**File:** `test/e2e/csi_topology_config_test.go`
**Labels:** `readonly`, `csi-operator`, `csi-topology`, `p1`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Verifies the `[Labels] topology-categories` entry written into the CSI driver's
`cloud.conf` (Secret `vsphere-csi-config-secret` in `openshift-cluster-csi-drivers`)
matches the topology category names actually registered on CSINode objects
(derived via `DiscoverCSITopologyKeys`, not hardcoded strings).

## Actions

1. Discover the real CSI topology keys from CSINode objects; skip if unavailable
2. Read the CSI driver cloud config secret
3. Parse the `topology-categories` value out of the `[Labels]` section
4. Assert the parsed categories match the discovered region/zone category names

## Code

```go
It("TOPO-01: CSI config secret topology-categories matches discovered topology keys", Label("p1"), func() {
    expected := expectedInfraTopologyCategories()

    config, err := framework.GetCSIDriverConfig(suiteCtx, clients.Kube)
    Expect(err).NotTo(HaveOccurred())

    categories, ok := framework.CSIConfigTopologyCategories(config)
    Expect(ok).To(BeTrue(), "cloud.conf [Labels] section should have a topology-categories entry")
    Expect(categories).To(ConsistOf(expected))
})
```
