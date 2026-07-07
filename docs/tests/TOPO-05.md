# TOPO-05: topology tags metric reflects Infrastructure-sourced baseline

**File:** `test/e2e/csi_topology_config_test.go`
**Labels:** `readonly`, `csi-operator`, `csi-topology`, `p1`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Scrapes the `vsphere_topology_tags` metric from the CSI operator pod and
asserts the baseline values: `{source="infrastructure"}` is `2` (the operator
hardcodes `openshift-region`/`openshift-zone` when Infrastructure has more
than one failure domain), and `{source="clustercsidriver"}` is `0` since the
`ClusterCSIDriver` `topologyCategories` field is unset at baseline.

## Actions

1. Scrape the operator's `/metrics` endpoint; skip if unreachable
2. Parse `vsphere_topology_tags{source="infrastructure"}`, assert it equals 2
3. Parse `vsphere_topology_tags{source="clustercsidriver"}`, assert it equals 0

## Code

```go
It("TOPO-05: topology tags metric reflects Infrastructure-sourced baseline", Label("p1"), func() {
    requireCSITopologyKeys()

    metrics, err := csiOperatorMetrics()
    if err != nil {
        Skip(fmt.Sprintf("cannot scrape operator metrics: %v", err))
    }

    infraVal, err := framework.ParseMetricValue(metrics, framework.TopologyTagsMetric,
        map[string]string{"source": framework.TopologyTagsSourceInfra})
    Expect(err).NotTo(HaveOccurred())
    Expect(infraVal).To(BeNumerically("==", 2),
        "infrastructure-sourced topology tags should be 2 (hardcoded region+zone)")

    ccdVal, err := framework.ParseMetricValue(metrics, framework.TopologyTagsMetric,
        map[string]string{"source": framework.TopologyTagsSourceCCD})
    Expect(err).NotTo(HaveOccurred())
    Expect(ccdVal).To(BeNumerically("==", 0),
        "clustercsidriver-sourced topology tags should be 0 at baseline (ClusterCSIDriver field unset)")
})
```
