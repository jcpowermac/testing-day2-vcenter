# N-CSI-11: StorageClass topology plumbing connected to Infrastructure FDs

**File:** `test/e2e/csi_storage_test.go`
**Labels:** `readonly`, `storage`, `p2`
**Component:** vmware-vsphere-csi-driver-operator

## Summary

Checks that CSI topology labels on StorageClass `allowedTopologies` (or on Nodes when no explicit topology is set) map to Infrastructure failure domains. Validates the topology plumbing is connected end-to-end.

## Actions

1. Get default StorageClass and CSI topology keys
2. If StorageClass has `allowedTopologies`, verify each label expression matches an Infrastructure FD
3. If no `allowedTopologies`, check Node labels for CSI topology keys matching Infrastructure FDs

## Code

```go
It("should have StorageClass topology plumbing connected to Infrastructure FDs (N-CSI-11)", func() {
    sc := requireDefaultStorageClass()
    topoKeys := requireCSITopologyKeys()
    infra := currentInfrastructure()
    fds := framework.GetFailureDomains(infra)

    if len(sc.AllowedTopologies) > 0 {
        for _, term := range sc.AllowedTopologies {
            for _, expr := range term.MatchLabelExpressions {
                if expr.Key == topoKeys.Region || expr.Key == topoKeys.Zone {
                    matched := false
                    for _, fd := range fds {
                        if (expr.Key == topoKeys.Region && containsString(expr.Values, fd.Region)) ||
                            (expr.Key == topoKeys.Zone && containsString(expr.Values, fd.Zone)) {
                            matched = true
                            break
                        }
                    }
                    Expect(matched).To(BeTrue())
                }
            }
        }
    } else {
        // Fall back to checking Node labels
        nodes, err := clients.Kube.CoreV1().Nodes().List(suiteCtx, metav1.ListOptions{})
        Expect(err).NotTo(HaveOccurred())
        found := false
        for _, node := range nodes.Items {
            region := node.Labels[topoKeys.Region]
            zone := node.Labels[topoKeys.Zone]
            if region != "" && zone != "" {
                for _, fd := range fds {
                    if fd.Region == region && fd.Zone == zone {
                        found = true
                        break
                    }
                }
            }
            if found { break }
        }
        Expect(found).To(BeTrue())
    }
})
```
