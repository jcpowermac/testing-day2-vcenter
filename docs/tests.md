# Test Catalog

## Feature Gate (`featuregate_test.go`) [readonly, p0, operator]

| Test Name | Description | Component | PR/Issue |
|---|---|---|---|
| should expose VSphereMultiVCenterDay2 on FeatureGate/cluster | Confirms the gate appears in FeatureGate/cluster status with a version string | openshift/api | |
| should report gate enabled state consistently | Cross-checks `IsFeatureGateEnabled` returns the same result the BeforeSuite cached | openshift/api | |

## Infrastructure xValidation (`infrastructure_validation_test.go`) [readonly, validation, p0]

| Test Name | Description | Component | PR/Issue |
|---|---|---|---|
| should allow adding a second vCenter via dry-run | Dry-run patches a second vCenter and expects acceptance | openshift/api | |
| should reject duplicate vCenter server values (N-INF-01/02) | Two vCenters with same server rejected by CRD uniqueness rule | openshift/api | |
| should reject reducing vcenters to an empty array (N-INF-03) | `vcenters:[]` rejected by minItems=1 rule | openshift/api | |
| should reject removing the vcenters field once set (N-INF-04) | `vcenters:null` rejected | openshift/api | |
| should reject swapping an existing vCenter server (N-INF-05) | Changing vCenter server address triggers "Cannot add and remove at the same time" | openshift/api | |
| should reject simultaneous add and remove of vCenters (N-INF-06/07) | Replacing one vCenter with another in same patch triggers add-and-remove guard | openshift/api | |
| should reject more than 3 vCenters (N-INF-11) | Exceeding 3 vCenters rejected by maxItems=3 rule | openshift/api | |
| should reject removing a vCenter still referenced by a failure domain (N-INF-12) | Removes vCenter while FD still references it. **Expected failure** — no xValidation rule enforces this | openshift/api | SPLAT-2827 |
| should allow patching unrelated Infrastructure fields via dry-run (ratcheting) | Identity-patches current spec to confirm ratcheting allows no-op updates | openshift/api | |
| should reject adding a second vCenter (N-INF-09) | Gate-off: adding a vCenter triggers immutability rule | openshift/api | |
| should reject removing the only vCenter (N-INF-10) | Gate-off: emptying vcenters list triggers immutability rule | openshift/api | |

## ValidatingAdmissionPolicies (`vap_test.go`) [readonly/mutating, admission, p0]

| Test Name | Description | Component | PR/Issue |
|---|---|---|---|
| should install vSphere failure domain VAP resources | Verifies all 3 VAPs (machine, cpms, machineset) and their bindings exist | cluster-config-operator | |
| should deny removing a failure domain referenced by a Machine (N-SEQ-01) | Removes FD matching Machine region/zone labels, expects VAP denial | cluster-config-operator | |
| should deny removing a failure domain referenced by a CPMS (N-SEQ-02) | Removes FD matching CPMS FD name reference, expects VAP denial | cluster-config-operator | |
| should deny removing a failure domain referenced by a MachineSet (N-SEQ-03) | Creates 1-replica MachineSet, waits for Machine, tests VAP denial, cleans up | cluster-config-operator | |
| should allow removing an unreferenced failure domain via dry-run | Dry-run probes each FD to find one the API accepts for removal | cluster-config-operator | |
| should not require vSphere VAP resources | Gate-off: confirms Machine VAP is absent | cluster-config-operator | |
| records whether dry-run triggers VAP denials | Logs whether VAP enforcement works under dry-run | cluster-config-operator | |

## Cloud Config Content (`configmap_content_test.go`) [readonly, config, p0]

| Test Name | Description | Component | PR/Issue |
|---|---|---|---|
| should parse managed kube-cloud-config YAML (N-CFG-01/02/03) | Parses `openshift-config-managed/kube-cloud-config` as valid cloud config YAML | cluster-config-operator | |
| should parse CCM cloud-conf YAML | Parses `openshift-cloud-controller-manager/cloud-conf` as valid cloud config YAML | cloud-controller-manager | |
| should include all Infrastructure vCenters in managed cloud config | Every Infrastructure vCenter has a corresponding managed cloud config entry | cluster-config-operator | |
| should not contain stale vCenters in managed cloud config (N-CFG-06) | No cloud config vCenter entries exist that aren't in Infrastructure CR | cluster-config-operator | |
| should keep insecure-flag out of per-vCenter entries when possible | `insecure-flag` only set globally, not duplicated per-vCenter | cluster-config-operator | |
| should include source openshift-config cloud config when present (three-way parity) | Managed cloud config semantically matches Infrastructure CR and source ConfigMap | cluster-config-operator | |
| should expose node network settings when configured (installer #10614) | `nodes` section has `externalNetworkSubnetCidr` or `internalNetworkSubnetCidr` | openshift/installer | installer#10614 |

## ConfigMap Ownership (`configmap_ownership_test.go`) [readonly/mutating, config, operator, p0/p1]

| Test Name | Description | Component | PR/Issue |
|---|---|---|---|
| should expose kube-cloud-config in openshift-config-managed | Managed ConfigMap exists with `cloud.conf` data key | cluster-config-operator | |
| should keep managed ConfigMap stable over observation window | Watches managed ConfigMap 60s, confirms no unexpected updates | cluster-config-operator | |
| should expose cloud-conf for CCM consumption | CCM ConfigMap exists with `cloud.conf` data key | cloud-controller-manager | |
| should recreate kube-cloud-config if deleted when gate is enabled (N-OP-07) | Deletes managed ConfigMap, expects config-operator to recreate it | cluster-config-operator | |

## Credential Propagation (`credentials_test.go`) [readonly, integration, p0]

| Test Name | Description | Component | PR/Issue |
|---|---|---|---|
| should have credential secrets for all Infrastructure vCenters | All 4 consumer secrets have key entries for every Infrastructure vCenter | cloud-credential-operator | |
| should have {namespace}/{name} with entries for every vCenter | Per-secret check, skips if secret doesn't exist | cloud-credential-operator | |

## Machine Integration (`machine_integration_test.go`) [readonly, integration, p0]

| Test Name | Description | Component | PR/Issue |
|---|---|---|---|
| should have all worker Machines in a healthy phase | Every non-deleting Machine is Running or Provisioned | machine-api-operator | |
| should label every Machine with region and zone | Every Machine has non-empty region/zone labels | cloud-controller-manager | |
| should map every Machine to a valid Infrastructure failure domain | Machine region/zone labels match Infrastructure FDs | cloud-controller-manager | |
| should have Machine providerSpec workspace matching Infrastructure topology | Machine workspace datacenter matches labeled FD topology | machine-api-operator | |

## CPMS Integration (`cpms_integration_test.go`) [readonly, integration, p0]

| Test Name | Description | Component | PR/Issue |
|---|---|---|---|
| should reference failure domain names that exist in Infrastructure | Every CPMS FD name exists in Infrastructure CR | machine-api-operator | |
| should have CPMS failure domains covering all Infrastructure FDs | Logs Infrastructure FDs not referenced by CPMS (informational) | machine-api-operator | |

## MachineSet Integration (`machineset_integration_test.go`) [readonly, integration, p0]

| Test Name | Description | Component | PR/Issue |
|---|---|---|---|
| should have providerSpec workspace matching an Infrastructure FD topology | MachineSet workspace datacenter maps to known Infrastructure FD | machine-api-operator | |
| should have MachineSet template labels matching Infrastructure failure domains | Region/zone labels on MachineSet templates correspond to existing FDs | machine-api-operator | |

## CSI Driver Integration (`csi_integration_test.go`) [readonly, integration, p1]

| Test Name | Description | Component | PR/Issue |
|---|---|---|---|
| should have CSI driver credential secret with entries for all vCenters | `openshift-cluster-csi-drivers/vmware-vsphere-cloud-credentials` has keys for every vCenter | vmware-vsphere-csi-driver-operator | |
| should have CSI driver pods running | CSI driver controller pods are in Running phase | vmware-vsphere-csi-driver-operator | |
| should have managed cloud config listing all vCenter datacenters for CSI | Multi-vCenter cloud config includes all vCenters for CSI | vmware-vsphere-csi-driver-operator | |

## Operator Health (`operator_health_test.go`) [readonly, operator, p0/p1]

| Test Name | Description | Component | PR/Issue |
|---|---|---|---|
| should keep ClusterOperator/{name} Available and not Degraded | CCM, config-operator, machine-api are Available=True, Degraded=False | multiple | |
| should not have CCM pods in a crash loop | CCM pods have fewer than 5 restarts | cloud-controller-manager | |
| should not have MAO pods in a crash loop | MAO pods have fewer than 5 restarts | machine-api-operator | |
| should not have CSI driver pods in a crash loop | CSI driver controller pods have fewer than 5 restarts | vmware-vsphere-csi-driver-operator | |

## vsphere-problem-detector (`problem_detector_test.go`) [readonly, operator, p1]

| Test Name | Description | Component | PR/Issue |
|---|---|---|---|
| should have vsphere-problem-detector-operator deployment available | Deployment in `openshift-cluster-storage-operator` has >=1 available replica | vsphere-problem-detector | |
| should not have vsphere-problem-detector pods in a crash loop | VPD pods have fewer than 5 restarts | vsphere-problem-detector | |
| should validate GetVCenter behavior after failure domain removal when #224 merges | Placeholder for N-CFG-08. Currently skipped | vsphere-problem-detector | vsphere-problem-detector#224 |

## Topology Lifecycle (`topology_lifecycle_test.go`) [mutating, p0/p1]

| Test Name | Description | Component | PR/Issue |
|---|---|---|---|
| should deny removing a failure domain that still has Machines (N-SEQ-05 precheck) | Dry-run removes Machine-backed FD, expects VAP denial | cluster-config-operator | |
| should deny removing a vCenter referenced by a failure domain (N-SEQ-04) | Removes vCenter while FD remains. **Expected failure** — no xValidation rule | openshift/api | SPLAT-2827 |
| should deny removing an FD referenced by a scaled MachineSet | Creates 1-replica MachineSet, waits for Machine, tests VAP denial, cleans up | cluster-config-operator | |
| should add and remove a temporary vCenter without leaving stale cloud config (#469) | Adds fake vCenter, waits for reconciliation, removes it, confirms no stale entries | cluster-config-operator | cluster-config-operator#469 |

## Real vCenter Day 2 (`real_vcenter_test.go`) [real-vcenter, p0, mutating]

| Test Name | Description | Component | PR/Issue |
|---|---|---|---|
| should include configured vCenter in Infrastructure | Lab config's second vCenter appears in Infrastructure vcenters list | openshift/api | |
| should reflect configured vCenter in managed cloud config | Managed cloud config includes all Infrastructure vCenters, no stale entries | cluster-config-operator | |
| should pass lab verification helper | Runs full `lab.Verify` workflow against live cluster | multiple | |
| should include failure domain when configured | Lab FD exists in Infrastructure with correct server reference | openshift/api | |
| should have credential secrets updated with second vCenter entries | All 4 consumer secrets have entries for second vCenter | cloud-credential-operator | |
| should keep all operators healthy after Day 2 add | CCM, config-operator, machine-api are Available after real vCenter add | multiple | |
| should have CCM cloud config reflecting second vCenter | CCM cloud config includes second vCenter server | cloud-controller-manager | |
