# Test Catalog

## Feature Gate (`featuregate_test.go`) [readonly, p0, operator]

- **should expose VSphereMultiVCenterDay2 on FeatureGate/cluster** — Confirms the VSphereMultiVCenterDay2 gate appears in FeatureGate/cluster status with a version string.
- **should report gate enabled state consistently** — Cross-checks that `IsFeatureGateEnabled` returns the same result the BeforeSuite cached.

## Infrastructure xValidation (`infrastructure_validation_test.go`) [readonly, validation, p0]

### Gate enabled

- **should allow adding a second vCenter via dry-run** — Dry-run patches a second vCenter into the Infrastructure spec and expects acceptance.
- **should reject duplicate vCenter server values (N-INF-01/02)** — Submits a spec with two vCenters sharing the same server and expects the CRD uniqueness rule to reject it.
- **should reject reducing vcenters to an empty array (N-INF-03)** — Sends a raw JSON patch setting `vcenters:[]` and expects the minItems=1 rule to reject it.
- **should reject removing the vcenters field once set (N-INF-04)** — Sends a raw JSON patch setting `vcenters:null` and expects rejection.
- **should reject swapping an existing vCenter server (N-INF-05)** — Changes an existing vCenter's server address and expects the "Cannot add and remove at the same time" rule to fire.
- **should reject simultaneous add and remove of vCenters (N-INF-06/07)** — Replaces one vCenter with a new one in the same patch, which triggers the add-and-remove guard.
- **should reject more than 3 vCenters (N-INF-11)** — Adds vCenters until count exceeds 3 and expects the maxItems=3 rule to reject.
- **should reject removing a vCenter still referenced by a failure domain (N-INF-12)** — Removes a vCenter while a failure domain still references its server. Currently fails intentionally because no xValidation rule enforces this (SPLAT-2827).
- **should allow patching unrelated Infrastructure fields via dry-run (ratcheting)** — Identity-patches the current spec to confirm ratcheting allows no-op updates.

### Gate disabled

- **should reject adding a second vCenter (N-INF-09)** — With the gate off, adding a vCenter triggers the "vcenters cannot be added or removed once set" rule.
- **should reject removing the only vCenter (N-INF-10)** — With the gate off, emptying the vcenters list triggers the same immutability rule.

## ValidatingAdmissionPolicies (`vap_test.go`) [readonly, admission, p0]

### Gate enabled

- **should install vSphere failure domain VAP resources** — Verifies all three VAPs (machine, cpms, machineset) and their bindings exist on the cluster.
- **should deny removing a failure domain referenced by a Machine (N-SEQ-01)** — Removes a failure domain whose region/zone matches a Machine's labels and expects the VAP to deny.
- **should allow removing an unreferenced failure domain via dry-run** — Finds a failure domain not referenced by any Machine and confirms removal is accepted.

### Gate disabled

- **should not require vSphere VAP resources** — Confirms the Machine VAP is absent when the gate is off.

### Probe

- **records whether dry-run triggers VAP denials** — Logs whether VAP enforcement works under dry-run on this cluster.

## Cloud Config Content (`configmap_content_test.go`) [readonly, config, p0]

- **should parse managed kube-cloud-config YAML (N-CFG-01/02/03)** — Reads `openshift-config-managed/kube-cloud-config` and confirms it parses as valid cloud config YAML.
- **should parse CCM cloud-conf YAML** — Reads `openshift-cloud-controller-manager/cloud-conf` and confirms it parses as valid cloud config YAML.
- **should include all Infrastructure vCenters in managed cloud config** — Cross-checks that every vCenter in the Infrastructure CR has a corresponding entry in the managed cloud config.
- **should not contain stale vCenters in managed cloud config (N-CFG-06)** — Confirms no vCenter entries exist in the managed cloud config that aren't in the Infrastructure CR.
- **should keep insecure-flag out of per-vCenter entries when possible** — Checks that `insecure-flag` is only set globally, not duplicated per-vCenter.
- **should include source openshift-config cloud config when present (three-way parity)** — When the source ConfigMap in `openshift-config` exists, validates that managed cloud config semantically matches the Infrastructure CR.
- **should expose node network settings when configured (installer #10614)** — If the cloud config has a `nodes` section, confirms `externalNetworkSubnetCidr` or `internalNetworkSubnetCidr` is populated.

## ConfigMap Ownership (`configmap_ownership_test.go`) [readonly/mutating, config, operator, p0/p1]

- **should expose kube-cloud-config in openshift-config-managed** — Confirms the managed ConfigMap exists with the `cloud.conf` data key.
- **should keep managed ConfigMap stable over observation window** — Watches the managed ConfigMap for 60 seconds and confirms no unexpected updates (single-writer steady state).
- **should expose cloud-conf for CCM consumption** — Confirms the CCM ConfigMap exists with the `cloud.conf` data key.
- **should recreate kube-cloud-config if deleted when gate is enabled (N-OP-07)** — Deletes the managed ConfigMap and expects the config-operator to recreate it within the default timeout. Restores the original on cleanup.

## Operator Health (`operator_health_test.go`) [readonly, operator, p0]

- **should keep ClusterOperator/{name} Available and not Degraded** — For each of `cloud-controller-manager`, `config-operator`, and `machine-api`, confirms Available=True and Degraded=False.

## vsphere-problem-detector (`problem_detector_test.go`) [readonly, operator, p1]

- **should keep ClusterOperator/vsphere-problem-detector Available when installed** — If the vsphere-problem-detector ClusterOperator exists, confirms it is Available.
- **should validate GetVCenter behavior after failure domain removal when #224 merges** — Placeholder for future testing of vsphere-problem-detector#224 (N-CFG-08). Currently skipped.

## Topology Lifecycle (`topology_lifecycle_test.go`) [mutating, p0]

- **should deny removing a failure domain that still has Machines (N-SEQ-05 precheck)** — Dry-run removes a Machine-backed failure domain and expects the VAP to deny the update.
- **should deny removing a vCenter referenced by a failure domain (N-SEQ-04)** — Removes a vCenter while its failure domain remains. Currently fails intentionally because no xValidation rule enforces this (SPLAT-2827).
- **should add and remove a temporary vCenter without leaving stale cloud config (#469)** — Adds a fake vCenter, waits for cloud config reconciliation, removes it, then confirms no stale entries remain. Restores Infrastructure on cleanup.

## Real vCenter Day 2 (`real_vcenter_test.go`) [real-vcenter, p0, mutating]

- **should include configured vCenter in Infrastructure** — Confirms the lab config's second vCenter server appears in the Infrastructure CR's vcenters list.
- **should reflect configured vCenter in managed cloud config** — Parses managed cloud config and validates it includes all Infrastructure vCenters with no stale entries.
- **should pass lab verification helper** — Runs the full `lab.Verify` workflow against the live cluster.
- **should include failure domain when configured** — When lab config includes a failure domain, confirms it exists in the Infrastructure CR with the correct server reference.
