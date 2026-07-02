# N-VPD-01: VPD deployment has >= 1 available replica

**File:** `test/e2e/problem_detector_test.go`
**Labels:** `readonly`, `operator`, `p1`
**Component:** vsphere-problem-detector

## Summary

Verifies the vsphere-problem-detector-operator Deployment in `openshift-cluster-storage-operator` has at least one available replica.

## Actions

1. GET the Deployment; skip if not found
2. Assert `status.availableReplicas >= 1`

## Code

```go
It("should have vsphere-problem-detector-operator deployment available", func() {
    deploy, err := clients.Kube.AppsV1().Deployments(vpdNamespace).Get(
        suiteCtx, vpdDeployment, metav1.GetOptions{})
    if err != nil {
        Skip(fmt.Sprintf("vsphere-problem-detector deployment not found: %v", err))
    }
    Expect(deploy.Status.AvailableReplicas).To(BeNumerically(">=", 1))
})
```
