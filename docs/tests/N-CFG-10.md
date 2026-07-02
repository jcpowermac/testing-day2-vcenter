# N-CFG-10: nodes section has externalNetworkSubnetCidr or internalNetworkSubnetCidr

**File:** `test/e2e/configmap_content_test.go`
**Labels:** `readonly`, `config`, `p0`
**Component:** openshift/installer

## Summary

Verifies that when a `nodes` section is present in the managed cloud config, it contains at least one of `externalNetworkSubnetCidr` or `internalNetworkSubnetCidr`. Validates the installer enhancement from installer#10614.

## Actions

1. Parse the managed cloud config YAML
2. Skip if `nodes` section is nil (not configured on this cluster)
3. Assert at least one of the CIDR fields is non-empty

## Code

```go
It("should expose node network settings when configured (installer #10614)", func() {
    cfg, err := vsphere.ParseCloudConfigYAML(managedCloudConfigYAML())
    Expect(err).NotTo(HaveOccurred())
    if cfg.Nodes == nil {
        Skip("nodes section not configured on this cluster")
    }
    Expect(cfg.Nodes.ExternalNetworkSubnetCidr != "" ||
        cfg.Nodes.InternalNetworkSubnetCidr != "").To(BeTrue())
})
```
