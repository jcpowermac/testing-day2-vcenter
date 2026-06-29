# Test Catalog

## Feature Gate (`featuregate_test.go`) [readonly, p0, operator]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| N-GATE-01 | Gate appears in FeatureGate/cluster status with a version string | openshift/api | |
| N-GATE-02 | `IsFeatureGateEnabled` matches BeforeSuite cached result | openshift/api | |

## Infrastructure xValidation (`infrastructure_validation_test.go`) [readonly, validation, p0]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| N-INF-00 | Dry-run adding a second vCenter is accepted | openshift/api | |
| N-INF-01/02 | Duplicate vCenter server values rejected by CRD uniqueness rule | openshift/api | |
| N-INF-03 | `vcenters:[]` rejected by minItems=1 rule | openshift/api | |
| N-INF-04 | `vcenters:null` rejected | openshift/api | |
| N-INF-05 | Swapping existing vCenter server triggers "Cannot add and remove at the same time" | openshift/api | |
| N-INF-06/07 | Replacing one vCenter with another in same patch triggers add-and-remove guard | openshift/api | |
| N-INF-08 | Ratcheting: identity-patch of current spec accepted (no-op update) | openshift/api | |
| N-INF-09 | Gate-off: adding a vCenter triggers immutability rule | openshift/api | |
| N-INF-10 | Gate-off: emptying vcenters list triggers immutability rule | openshift/api | |
| N-INF-11 | Exceeding 3 vCenters rejected by maxItems=3 rule | openshift/api | |
| N-INF-12 | Removing vCenter still referenced by FD. **Expected failure** — no xValidation rule | openshift/api | SPLAT-2827 |

## ValidatingAdmissionPolicies (`vap_test.go`) [readonly/mutating, admission, p0]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| N-SEQ-00 | All 3 VAPs (machine, cpms, machineset) and bindings exist on cluster | cluster-config-operator | |
| N-SEQ-01 | Removing FD matching Machine region/zone labels denied by VAP | cluster-config-operator | |
| N-SEQ-02 | Removing FD matching CPMS FD name reference denied by VAP | cluster-config-operator | |
| N-SEQ-03 | Create 1-replica MachineSet, wait for Machine, removing its FD denied by VAP | cluster-config-operator | |
| N-SEQ-06 | Dry-run probes each FD to find one the API accepts for removal | cluster-config-operator | |
| N-SEQ-07 | Gate-off: Machine VAP is absent | cluster-config-operator | |
| N-SEQ-PROBE | Logs whether VAP enforcement works under dry-run on this cluster | cluster-config-operator | |

## Cloud Config Content (`configmap_content_test.go`) [readonly, config, p0]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| N-CFG-01/02/03 | `openshift-config-managed/kube-cloud-config` parses as valid cloud config YAML | cluster-config-operator | |
| N-CFG-04 | `openshift-cloud-controller-manager/cloud-conf` parses as valid cloud config YAML | cloud-controller-manager | |
| N-CFG-05 | Every Infrastructure vCenter has a corresponding managed cloud config entry | cluster-config-operator | |
| N-CFG-06 | No stale vCenter entries in managed cloud config | cluster-config-operator | |
| N-CFG-07 | `insecure-flag` only set globally, not duplicated per-vCenter | cluster-config-operator | |
| N-CFG-09 | Managed cloud config semantically matches Infrastructure CR and source ConfigMap | cluster-config-operator | |
| N-CFG-10 | `nodes` section has `externalNetworkSubnetCidr` or `internalNetworkSubnetCidr` | openshift/installer | installer#10614 |

## ConfigMap Ownership (`configmap_ownership_test.go`) [readonly/mutating, config, operator, p0/p1]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| N-CM-01 | Managed ConfigMap exists with `cloud.conf` data key | cluster-config-operator | |
| N-CM-02 | Managed ConfigMap stable over 60s observation window (single-writer) | cluster-config-operator | |
| N-CM-03 | CCM ConfigMap exists with `cloud.conf` data key | cloud-controller-manager | |
| N-OP-07 | Deleting managed ConfigMap triggers config-operator recreation | cluster-config-operator | |

## Credential Propagation (`credentials_test.go`) [readonly, integration, p0]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| N-CRED-01 | All 4 consumer secrets have key entries for every Infrastructure vCenter | cloud-credential-operator | |
| N-CRED-02 | Per-secret check for each consumer (skips if secret missing) | cloud-credential-operator | |

## Machine Integration (`machine_integration_test.go`) [readonly, integration, p0]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| N-MACH-01 | Every non-deleting Machine is Running or Provisioned | machine-api-operator | |
| N-MACH-02 | Every Machine has non-empty region/zone labels | cloud-controller-manager | |
| N-MACH-03 | Machine region/zone labels match an Infrastructure failure domain | cloud-controller-manager | |
| N-MACH-04 | Machine workspace datacenter matches labeled FD topology | machine-api-operator | |

## CPMS Integration (`cpms_integration_test.go`) [readonly, integration, p0]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| N-CPMS-01 | Every CPMS FD name exists in Infrastructure CR | machine-api-operator | |
| N-CPMS-02 | Logs Infrastructure FDs not referenced by CPMS (informational) | machine-api-operator | |

## MachineSet Integration (`machineset_integration_test.go`) [readonly, integration, p0]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| N-MS-01 | MachineSet workspace datacenter maps to known Infrastructure FD | machine-api-operator | |
| N-MS-02 | Region/zone labels on MachineSet templates match existing FDs | machine-api-operator | |

## CSI Driver Integration (`csi_integration_test.go`) [readonly, integration, p1]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| N-CSI-01 | CSI credential secret has keys for every Infrastructure vCenter | vmware-vsphere-csi-driver-operator | |
| N-CSI-02 | CSI driver controller pods are Running | vmware-vsphere-csi-driver-operator | |
| N-CSI-03 | Multi-vCenter cloud config includes all vCenters for CSI | vmware-vsphere-csi-driver-operator | |

## Operator Health (`operator_health_test.go`) [readonly, operator, p0/p1]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| N-OP-01 | CCM, config-operator, machine-api are Available=True, Degraded=False | multiple | |
| N-OP-02 | CCM pods have fewer than 5 restarts | cloud-controller-manager | |
| N-OP-03 | MAO pods have fewer than 5 restarts | machine-api-operator | |
| N-OP-04 | CSI driver controller pods have fewer than 5 restarts | vmware-vsphere-csi-driver-operator | |

## vsphere-problem-detector (`problem_detector_test.go`) [readonly, operator, p1]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| N-VPD-01 | VPD deployment in `openshift-cluster-storage-operator` has >=1 available replica | vsphere-problem-detector | |
| N-VPD-02 | VPD pods have fewer than 5 restarts | vsphere-problem-detector | |
| N-CFG-08 | Validate GetVCenter after FD removal. Currently skipped pending upstream | vsphere-problem-detector | vsphere-problem-detector#224 |

## Topology Lifecycle (`topology_lifecycle_test.go`) [mutating, p0/p1]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| N-SEQ-05 | Dry-run removing Machine-backed FD denied by VAP (precheck) | cluster-config-operator | |
| N-SEQ-04 | Removing vCenter while FD remains. **Expected failure** — no xValidation rule | openshift/api | SPLAT-2827 |
| N-TOPO-01 | Create 1-replica MachineSet, removing its FD denied by VAP, scale down and cleanup | cluster-config-operator | |
| N-TOPO-02 | Add fake vCenter, wait for reconciliation, remove it, confirm no stale config | cluster-config-operator | cluster-config-operator#469 |

## Real vCenter Day 2 (`real_vcenter_test.go`) [real-vcenter, p0, mutating]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| N-RVC-01 | Lab config's second vCenter appears in Infrastructure vcenters list | openshift/api | |
| N-RVC-02 | Managed cloud config includes all Infrastructure vCenters, no stale entries | cluster-config-operator | |
| N-RVC-03 | Full `lab.Verify` workflow passes against live cluster | multiple | |
| N-RVC-04 | Lab FD exists in Infrastructure with correct server reference | openshift/api | |
| N-RVC-05 | All 4 credential secrets have entries for second vCenter | cloud-credential-operator | |
| N-RVC-06 | CCM, config-operator, machine-api are Available after Day 2 add | multiple | |
| N-RVC-07 | CCM cloud config includes second vCenter server | cloud-controller-manager | |
