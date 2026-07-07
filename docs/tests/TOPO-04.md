# TOPO-04: CSINode topology keys match discovered categories

**File:** `test/e2e/csi_topology_config_test.go`
**Labels:** `readonly`, `csi-operator`, `csi-topology`, `p1`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Verifies the deduplicated set of `topology.csi.vmware.com/`-prefixed keys
across all CSINode objects is exactly the region/zone pair discovered by
`DiscoverCSITopologyKeys` — i.e. no stray or extra topology categories are
registered on any node.

## Actions

1. Discover the real CSI topology keys; skip if unavailable
2. List all CSINode objects and collect their vSphere CSI driver topology keys
3. Assert the collected set consists exactly of the discovered region and zone keys

## Code

```go
It("TOPO-04: CSINode topology keys match discovered categories", Label("p1"), func() {
    topoKeys := requireCSITopologyKeys()

    keys, err := framework.GetCSINodeTopologyKeys(suiteCtx, clients.Kube)
    Expect(err).NotTo(HaveOccurred())

    Expect(keys).To(ConsistOf(topoKeys.Region, topoKeys.Zone))
})
```
