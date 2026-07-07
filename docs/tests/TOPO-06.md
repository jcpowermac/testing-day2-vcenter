# TOPO-06: ClusterCSIDriver topologyCategories updates metric without overriding Infrastructure

**File:** `test/e2e/csi_topology_config_test.go`
**Labels:** `mutating`, `csi-operator`, `csi-topology`, `p1`

**Component:** vmware-vsphere-csi-driver-operator

## Summary

Sets `spec.driverConfig.vSphere.topologyCategories: ["custom-zone"]` on the
`ClusterCSIDriver` and verifies the `vsphere_topology_tags{source="clustercsidriver"}`
metric updates to `1` — but the CSI config secret's `topology-categories`
value is unaffected, still reflecting the Infrastructure-derived categories.
This exercises the operator's documented precedence model: Infrastructure
wins whenever more than one failure domain exists, so `GetTopologyCategories()`
never picks up the ClusterCSIDriver field on this cluster; only the
independently-updated metric proves the field was read at all. The original
`topologyCategories` value is restored in `DeferCleanup`.

## Actions

1. Read current `ClusterCSIDriver` and remember `spec.driverConfig.vSphere.topologyCategories`
2. Register cleanup to restore that value
3. Patch `topologyCategories` to `["custom-zone"]`
4. Poll `vsphere_topology_tags{source="clustercsidriver"}` until it becomes `1`
5. Read the CSI config secret and assert `topology-categories` still matches the
   Infrastructure-derived categories (precedence unaffected)

## Code

```go
It("TOPO-06: ClusterCSIDriver topologyCategories updates metric without overriding Infrastructure", Label("p1"), func() {
    expected := expectedInfraTopologyCategories()

    ccd, err := clients.Operator.OperatorV1().ClusterCSIDrivers().Get(suiteCtx, framework.ClusterCSIDriverName, metav1.GetOptions{})
    Expect(err).NotTo(HaveOccurred())
    var originalCategories []string
    if ccd.Spec.DriverConfig.VSphere != nil {
        originalCategories = ccd.Spec.DriverConfig.VSphere.TopologyCategories
    }

    DeferCleanup(func() {
        Expect(framework.SetClusterCSIDriverTopologyCategories(suiteCtx, clients.Operator, originalCategories)).To(Succeed())
    })

    Expect(framework.SetClusterCSIDriverTopologyCategories(suiteCtx, clients.Operator, []string{"custom-zone"})).To(Succeed())

    Eventually(func() (float64, error) {
        metrics, mErr := csiOperatorMetrics()
        if mErr != nil {
            return 0, mErr
        }
        return framework.ParseMetricValue(metrics, framework.TopologyTagsMetric,
            map[string]string{"source": framework.TopologyTagsSourceCCD})
    }).WithTimeout(framework.DefaultTimeout).WithPolling(framework.DefaultPolling).Should(
        BeNumerically("==", 1), "clustercsidriver-sourced topology tags metric should become 1")

    config, err := framework.GetCSIDriverConfig(suiteCtx, clients.Kube)
    Expect(err).NotTo(HaveOccurred())
    categories, ok := framework.CSIConfigTopologyCategories(config)
    Expect(ok).To(BeTrue())
    Expect(categories).To(ConsistOf(expected),
        "Infrastructure topology categories should take precedence over ClusterCSIDriver's since >1 FDs exist")
})
```
