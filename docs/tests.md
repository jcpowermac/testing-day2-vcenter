# Test Catalog

> **Note:** Tests guarded with `requireMultiVCenter()` skip on single-vCenter clusters.
> Run `make apply-lab` to add the second vCenter before expecting full coverage.
> Tests labeled `real-vcenter` additionally require `E2E_LAB_CONFIG` to be set.

## Feature Gate (`featuregate_test.go`) [readonly, p0, operator]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| [N-GATE-01](tests/N-GATE-01.md) | Gate appears in FeatureGate/cluster status with a version string | openshift/api | api#2783, CCO#481, CCO#489 |
| [N-GATE-02](tests/N-GATE-02.md) | `IsFeatureGateEnabled` matches BeforeSuite cached result | openshift/api | api#2783 |

## Infrastructure xValidation (`infrastructure_validation_test.go`) [readonly, validation, p0]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| [N-INF-00](tests/N-INF-00.md) | Dry-run adding a second vCenter is accepted | openshift/api | api#2784 |
| [N-INF-01/02](tests/N-INF-01-02.md) | Duplicate vCenter server values rejected by CRD uniqueness rule | openshift/api | api#2784 |
| [N-INF-03](tests/N-INF-03.md) | `vcenters:[]` rejected by minItems=1 rule | openshift/api | api#2784 |
| [N-INF-04](tests/N-INF-04.md) | `vcenters:null` rejected | openshift/api | api#2784 |
| [N-INF-05](tests/N-INF-05.md) | Swapping existing vCenter server triggers "Cannot add and remove at the same time" | openshift/api | api#2784 |
| [N-INF-06/07](tests/N-INF-06-07.md) | Replacing one vCenter with another in same patch triggers add-and-remove guard | openshift/api | api#2784 |
| [N-INF-08](tests/N-INF-08.md) | Ratcheting: identity-patch of current spec accepted (no-op update) | openshift/api | api#2784 |
| [N-INF-09](tests/N-INF-09.md) | Gate-off: adding a vCenter triggers immutability rule | openshift/api | api#2784 |
| [N-INF-10](tests/N-INF-10.md) | Gate-off: emptying vcenters list triggers immutability rule | openshift/api | api#2784 |
| [N-INF-11](tests/N-INF-11.md) | Exceeding 3 vCenters rejected by maxItems=3 rule | openshift/api | api#2784 |
| [N-INF-12](tests/N-INF-12.md) | Removing vCenter still referenced by FD. **Expected failure** — no xValidation rule | openshift/api | api#2784, SPLAT-2827 |

## ValidatingAdmissionPolicies (`vap_test.go`) [readonly/mutating, admission, p0]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| [N-SEQ-00](tests/N-SEQ-00.md) | All 3 VAPs (machine, cpms, machineset) and bindings exist on cluster | cluster-config-operator | MAO#1510 |
| [N-SEQ-01](tests/N-SEQ-01.md) | Removing FD matching Machine region/zone labels denied by VAP (real patch, `mutating`) | cluster-config-operator | MAO#1510 |
| [N-SEQ-02](tests/N-SEQ-02.md) | Removing FD matching CPMS FD name reference denied by VAP (real patch, `mutating`) | cluster-config-operator | MAO#1510 |
| [N-SEQ-03](tests/N-SEQ-03.md) | Create 1-replica MachineSet, wait for Machine, removing its FD denied by VAP | cluster-config-operator | MAO#1510 |
| [N-SEQ-06](tests/N-SEQ-06.md) | Dry-run probes each FD to find one the API accepts for removal | cluster-config-operator | MAO#1510 |
| [N-SEQ-07](tests/N-SEQ-07.md) | Gate-off: Machine VAP is absent | cluster-config-operator | MAO#1510 |

## Cloud Config Content (`configmap_content_test.go`) [readonly, config, p0]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| [N-CFG-01/02/03](tests/N-CFG-01-02-03.md) | `openshift-config-managed/kube-cloud-config` parses as valid cloud config YAML | cluster-config-operator | CCMO#442, lib-go#2175, installer#10529 |
| [N-CFG-04](tests/N-CFG-04.md) | `openshift-cloud-controller-manager/cloud-conf` parses as valid cloud config YAML | cloud-controller-manager | CCMO#442, installer#10529 |
| [N-CFG-05](tests/N-CFG-05.md) | Every Infrastructure vCenter has a corresponding managed cloud config entry | cluster-config-operator | CCMO#442 |
| [N-CFG-06](tests/N-CFG-06.md) | No stale vCenter entries in managed cloud config | cluster-config-operator | CCMO#469 |
| [N-CFG-07](tests/N-CFG-07.md) | `insecure-flag` only set globally, not duplicated per-vCenter | cluster-config-operator | lib-go#2195 |
| [N-CFG-09](tests/N-CFG-09.md) | Managed cloud config semantically matches Infrastructure CR and source ConfigMap | cluster-config-operator | CCMO#442, lib-go#2175 |
| [N-CFG-10](tests/N-CFG-10.md) | `nodes` section has `externalNetworkSubnetCidr` or `internalNetworkSubnetCidr` | openshift/installer | installer#10614, lib-go#2195 |

## ConfigMap Ownership (`configmap_ownership_test.go`) [readonly/mutating, config, operator, p0/p1]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| [N-CM-01](tests/N-CM-01.md) | Managed ConfigMap exists with `cloud.conf` data key | cluster-config-operator | CCMO#442, CCO#481 |
| [N-CM-02](tests/N-CM-02.md) | Managed ConfigMap stable over 60s observation window (single-writer) | cluster-config-operator | CCMO#442, CCO#481 |
| [N-CM-03](tests/N-CM-03.md) | CCM ConfigMap exists with `cloud.conf` data key | cloud-controller-manager | CCMO#442 |
| [N-OP-07](tests/N-OP-07.md) | Deleting managed ConfigMap triggers config-operator recreation | cluster-config-operator | CCO#481, CCO#489 |

## Credential Propagation (`credentials_test.go`) [readonly, integration, p0]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| [N-CRED-01](tests/N-CRED-01.md) | All 4 consumer secrets have key entries for every Infrastructure vCenter | cloud-credential-operator | |
| [N-CRED-02](tests/N-CRED-02.md) | Per-secret check for each consumer (skips if secret missing) | cloud-credential-operator | |

## Machine Integration (`machine_integration_test.go`) [readonly, integration, p0]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| [N-MACH-01](tests/N-MACH-01.md) | Every non-deleting Machine is Running or Provisioned | machine-api-operator | |
| [N-MACH-02](tests/N-MACH-02.md) | Every Machine has non-empty region/zone labels | cloud-controller-manager | CCMO#442 |
| [N-MACH-03](tests/N-MACH-03.md) | Machine region/zone labels match an Infrastructure failure domain | cloud-controller-manager | CCMO#442, api#2784 |
| [N-MACH-04](tests/N-MACH-04.md) | Machine workspace datacenter matches labeled FD topology | machine-api-operator | api#2784 |

## CPMS Integration (`cpms_integration_test.go`) [readonly, integration, p0]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| [N-CPMS-01](tests/N-CPMS-01.md) | Every CPMS FD name exists in Infrastructure CR | machine-api-operator | MAO#1510, api#2784 |
| [N-CPMS-02](tests/N-CPMS-02.md) | Logs Infrastructure FDs not referenced by CPMS (informational) | machine-api-operator | MAO#1510 |

## MachineSet Integration (`machineset_integration_test.go`) [readonly, integration, p0]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| [N-MS-01](tests/N-MS-01.md) | MachineSet workspace datacenter maps to known Infrastructure FD | machine-api-operator | api#2784 |
| [N-MS-02](tests/N-MS-02.md) | Region/zone labels on MachineSet templates match existing FDs | machine-api-operator | MAO#1510 |

## CSI Driver Integration (`csi_integration_test.go`) [readonly, integration, p1]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| [N-CSI-01](tests/N-CSI-INT-01.md) | CSI credential secret has keys for every Infrastructure vCenter | vmware-vsphere-csi-driver-operator | |
| [N-CSI-02](tests/N-CSI-INT-02.md) | CSI driver controller pods are Running | vmware-vsphere-csi-driver-operator | |
| [N-CSI-03](tests/N-CSI-INT-03.md) | Multi-vCenter cloud config includes all vCenters for CSI | vmware-vsphere-csi-driver-operator | CCMO#442 |

## CSI Storage Provisioning (`csi_storage_test.go`) [readonly/mutating, storage, p0-p2]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| [N-CSI-01](tests/N-CSI-STOR-01.md) | ClusterCSIDriver Available and not Degraded | vmware-vsphere-csi-driver-operator | |
| [N-CSI-10](tests/N-CSI-10.md) | Default StorageClass backed by vSphere CSI with WaitForFirstConsumer | vmware-vsphere-csi-driver-operator | |
| [N-CSI-11](tests/N-CSI-11.md) | StorageClass topology plumbing connected to Infrastructure FDs | vmware-vsphere-csi-driver-operator | |
| [N-CSI-02](tests/N-CSI-STOR-02.md) | CSI driver pods healthy with current Infrastructure topology | vmware-vsphere-csi-driver-operator | |
| [N-CSI-03](tests/N-CSI-STOR-03.md) | CSI credential secret reflecting all vCenters | vmware-vsphere-csi-driver-operator | |
| [N-CSI-04](tests/N-CSI-04.md) | Provision and bind PVC in existing failure domain | vmware-vsphere-csi-driver-operator | |
| [N-CSI-07](tests/N-CSI-07.md) | PV deleted when PVC deleted with reclaimPolicy Delete | vmware-vsphere-csi-driver-operator | |
| [N-CSI-05](tests/N-CSI-05.md) | Provision PV in new failure domain with correct topology labels. **Expected failure** — storage policy not propagated to second vCenter | vmware-vsphere-csi-driver-operator | |
| [N-CSI-06](tests/N-CSI-06.md) | Provision PVC with explicit topology constraint in correct FD. **Expected failure** — storage policy gap | vmware-vsphere-csi-driver-operator | |
| [N-CSI-08](tests/N-CSI-08.md) | Probe FD removal behavior when PVs exist in that FD | vmware-vsphere-csi-driver-operator, openshift/api | api#2784, MAO#1510 |
| [N-CSI-09](tests/N-CSI-09.md) | vCenter removal blocked by xValidation while PVs present (PV irrelevant) | vmware-vsphere-csi-driver-operator, openshift/api | api#2784 |

## Operator Health (`operator_health_test.go`) [readonly, operator, p0/p1]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| [N-OP-01](tests/N-OP-01.md) | CCM, config-operator, machine-api are Available=True, Degraded=False | multiple | CCMO#442, CCO#481, MAO#1510 |
| [N-OP-02](tests/N-OP-02.md) | CCM pods have fewer than 5 restarts | cloud-controller-manager | CCMO#442 |
| [N-OP-03](tests/N-OP-03.md) | MAO pods have fewer than 5 restarts | machine-api-operator | MAO#1510 |
| [N-OP-04](tests/N-OP-04.md) | CSI driver controller pods have fewer than 5 restarts | vmware-vsphere-csi-driver-operator | |

## vsphere-problem-detector (`problem_detector_test.go`) [readonly, operator, p1]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| [N-VPD-01](tests/N-VPD-01.md) | VPD deployment in `openshift-cluster-storage-operator` has >=1 available replica | vsphere-problem-detector | VPD#224 |
| [N-VPD-02](tests/N-VPD-02.md) | VPD pods have fewer than 5 restarts | vsphere-problem-detector | VPD#224 |
| [N-CFG-08](tests/N-CFG-08.md) | Validate GetVCenter after FD removal. Currently skipped pending upstream | vsphere-problem-detector | VPD#224 |

## Topology Lifecycle (`topology_lifecycle_test.go`) [mutating, p0/p1]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| [N-SEQ-05](tests/N-SEQ-05.md) | Dry-run removing Machine-backed FD denied by VAP (precheck) | cluster-config-operator | MAO#1510 |
| [N-SEQ-04](tests/N-SEQ-04.md) | Removing vCenter while FD remains. **Expected failure** — no xValidation rule | openshift/api | api#2784, SPLAT-2827 |
| [N-TOPO-01](tests/N-TOPO-01.md) | Create 1-replica MachineSet, removing its FD denied by VAP, scale down and cleanup | cluster-config-operator | MAO#1510 |
| [N-TOPO-02](tests/N-TOPO-02.md) | Add fake vCenter, wait for reconciliation, remove it, confirm no stale config | cluster-config-operator | CCMO#469 |

## Real vCenter Day 2 (`real_vcenter_test.go`) [real-vcenter, p0, mutating]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| [N-RVC-01](tests/N-RVC-01.md) | Lab config's second vCenter appears in Infrastructure vcenters list | openshift/api | api#2784 |
| [N-RVC-02](tests/N-RVC-02.md) | Managed cloud config includes all Infrastructure vCenters, no stale entries | cluster-config-operator | CCMO#442, CCMO#469 |
| [N-RVC-03](tests/N-RVC-03.md) | Full `lab.Verify` workflow passes against live cluster | multiple | api#2784, CCMO#442 |
| [N-RVC-04](tests/N-RVC-04.md) | Lab FD exists in Infrastructure with correct server reference | openshift/api | api#2784 |
| [N-RVC-05](tests/N-RVC-05.md) | All 4 credential secrets have entries for second vCenter | cloud-credential-operator | |
| [N-RVC-06](tests/N-RVC-06.md) | CCM, config-operator, machine-api are Available after Day 2 add | multiple | CCMO#442, CCO#481, MAO#1510 |
| [N-RVC-07](tests/N-RVC-07.md) | CCM cloud config includes second vCenter server | cloud-controller-manager | CCMO#442 |

## CSI Operator FD Lifecycle (`csi_operator_lifecycle_test.go`) [mutating, csi-operator, p0-p2]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| [FD-ADD-01](tests/FD-ADD-01.md) | Operator tags new FD's datastore with cluster tag | vmware-vsphere-csi-driver-operator | csi-op#348 |
| [FD-ADD-02](tests/FD-ADD-02.md) | SPBM profile exists on second vCenter after FD add | vmware-vsphere-csi-driver-operator | csi-op#348 |
| [FD-ADD-03](tests/FD-ADD-03.md) | Operator conditions healthy after FD add | vmware-vsphere-csi-driver-operator | csi-op#348 |
| [FD-ADD-04](tests/FD-ADD-04.md) | CSI driver config includes second vCenter | vmware-vsphere-csi-driver-operator | csi-op#348 |
| [FD-REM-01](tests/FD-REM-01.md) | Orphan tag detached after FD removal, re-attached after restore | vmware-vsphere-csi-driver-operator | csi-op#348 |
| [FD-REM-02](tests/FD-REM-02.md) | StorageClass and SPBM profile survive FD removal | vmware-vsphere-csi-driver-operator | csi-op#348 |
| [FD-REM-03](tests/FD-REM-03.md) | Operator reconciles within backoff window | vmware-vsphere-csi-driver-operator | csi-op#348 |
| [PV-SAFE-01](tests/PV-SAFE-01.md) | Orphan tag blocked when PVs exist on datastore | vmware-vsphere-csi-driver-operator | csi-op#348 |
| [PV-SAFE-02](tests/PV-SAFE-02.md) | Orphan cleanup proceeds after PVs deleted | vmware-vsphere-csi-driver-operator | csi-op#348 |
| [PV-SAFE-03](tests/PV-SAFE-03.md) | Force cleanup annotation overrides PV safety | vmware-vsphere-csi-driver-operator | csi-op#348 |
| [VC-REM-01](tests/VC-REM-01.md) | Complete vCenter removal lifecycle (FD→tag→SPBM→vCenter) | vmware-vsphere-csi-driver-operator | csi-op#348 |
| [VC-REM-02](tests/VC-REM-02.md) | CSI driver config updated after vCenter removal | vmware-vsphere-csi-driver-operator | csi-op#348 |
| [VC-REM-03](tests/VC-REM-03.md) | Credential secrets after vCenter removal [observational] | vmware-vsphere-csi-driver-operator | csi-op#348 |
| [EDGE-02](tests/EDGE-02.md) | Backoff resets after successful sync | vmware-vsphere-csi-driver-operator | csi-op#348 |
| [EDGE-03](tests/EDGE-03.md) | Topology transition 2 FDs → 1 FD → 2 FDs | vmware-vsphere-csi-driver-operator | csi-op#348 |
| [OBS-01](tests/OBS-01.md) | OrphanTagsDetectedTotal metric incremented on FD removal | vmware-vsphere-csi-driver-operator | csi-op#348 |
| [OBS-02](tests/OBS-02.md) | TagOperationsTotal tracks detach operations | vmware-vsphere-csi-driver-operator | csi-op#348 |
| [OBS-03](tests/OBS-03.md) | TagOperationsTotal tracks PV-blocked skips | vmware-vsphere-csi-driver-operator | csi-op#348 |

## CSI Topology Configuration (`csi_topology_config_test.go`) [readonly/mutating, csi-operator, csi-topology, p1]

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| [TOPO-01](tests/TOPO-01.md) | CSI config secret `topology-categories` matches discovered CSI topology keys | vmware-vsphere-csi-driver-operator | |
| [TOPO-02](tests/TOPO-02.md) | `csi-provisioner` container has `--feature-gates=Topology=true` and `--strict-topology` | vmware-vsphere-csi-driver-operator | |
| [TOPO-03](tests/TOPO-03.md) | Internal feature states ConfigMap has `improved-volume-topology` = `"true"` | vmware-vsphere-csi-driver-operator | |
| [TOPO-04](tests/TOPO-04.md) | CSINode topology keys match discovered categories | vmware-vsphere-csi-driver-operator | |
| [TOPO-05](tests/TOPO-05.md) | `vsphere_topology_tags` metric baseline: `infrastructure`=2, `clustercsidriver`=0 | vmware-vsphere-csi-driver-operator | |
| [TOPO-06](tests/TOPO-06.md) | ClusterCSIDriver `topologyCategories` updates metric without overriding Infrastructure precedence (`mutating`) | vmware-vsphere-csi-driver-operator | |

## CSI Synthetic Orphan Tags (`csi_orphan_tag_test.go`) [mutating, csi-operator, csi-orphan, p0/p1]

> Requires `E2E_LAB_CONFIG` with `orphanTest.datastore` set (or an auto-discoverable
> non-FD datastore), and requires `make apply-lab` to have run so the cluster tag/category
> exist on the second vCenter. Scoped to the second vCenter only.

| Test ID | Description | Component | PR/Issue |
|---|---|---|---|
| [SYNTH-01](tests/SYNTH-01.md) | Synthetic orphan tag detected and detached without PVs | vmware-vsphere-csi-driver-operator | |
| [SYNTH-02](tests/SYNTH-02.md) | Orphan cleanup latency within `OperatorSyncTimeout` | vmware-vsphere-csi-driver-operator | |
| [SYNTH-04](tests/SYNTH-04.md) | `OrphanTagsDetectedTotal` metric increments | vmware-vsphere-csi-driver-operator | |
| [SYNTH-05](tests/SYNTH-05.md) | `TagOperationsTotal` detach/success metric increments | vmware-vsphere-csi-driver-operator | |
| [SYNTH-09](tests/SYNTH-09.md) | Orphan cleanup causes no side-effect damage (operator healthy, StorageClass intact) | vmware-vsphere-csi-driver-operator | |
| [SYNTH-10](tests/SYNTH-10.md) | Operator handles repeated orphans without getting stuck | vmware-vsphere-csi-driver-operator | |

PV-blocked synthetic orphan tests (SYNTH-06/07/08) and the equivalent metric
test (OBS-03 analog) are deferred — see `plans/new-csi-operator-test-topology-config.md`.

## PR Reference

| Short Name | Full Reference | JIRA | Title |
|---|---|---|---|
| api#2783 | [openshift/api#2783](https://github.com/openshift/api/pull/2783) | SPLAT-2664 | Added new feature gate VSphereMultiVCenterDay2 |
| api#2784 | [openshift/api#2784](https://github.com/openshift/api/pull/2784) | SPLAT-2649 | Added vSphere Day 2 logic to CRDs |
| CCMO#442 | [openshift/cluster-cloud-controller-manager-operator#442](https://github.com/openshift/cluster-cloud-controller-manager-operator/pull/442) | SPLAT-2651 | Added support to manage kube-cloud-config for vSphere |
| CCMO#469 | [openshift/cluster-cloud-controller-manager-operator#469](https://github.com/openshift/cluster-cloud-controller-manager-operator/pull/469) | SPLAT-2792 | Fixed issue where old vCenters not removed from new cloud config |
| CCO#481 | [openshift/cluster-config-operator#481](https://github.com/openshift/cluster-config-operator/pull/481) | SPLAT-2717 | Migrate vSphere sync of kube-cloud-config to 3CMO |
| CCO#489 | [openshift/cluster-config-operator#489](https://github.com/openshift/cluster-config-operator/pull/489) | SPLAT-2747 | Updated kube cloud config controller to react to feature gate updates |
| installer#10529 | [openshift/installer#10529](https://github.com/openshift/installer/pull/10529) | SPLAT-2710 | Added vSphere day 2 support |
| installer#10614 | [openshift/installer#10614](https://github.com/openshift/installer/pull/10614) | SPLAT-2795 | Enhanced vSphere cloud config to include node network cidr information |
| lib-go#2175 | [openshift/library-go#2175](https://github.com/openshift/library-go/pull/2175) | SPLAT-2651 | Added vSphere cloud config modules from 3CMO |
| lib-go#2195 | [openshift/library-go#2195](https://github.com/openshift/library-go/pull/2195) | SPLAT-2651 | Changed Node struct to not be pointer in CPIConfig |
| MAO#1510 | [openshift/machine-api-operator#1510](https://github.com/openshift/machine-api-operator/pull/1510) | SPLAT-2790 | Added new VAP for vSphere infra validation |
| VPD#224 | [openshift/vsphere-problem-detector#224](https://github.com/openshift/vsphere-problem-detector/pull/224) | OCPBUGS-87906 | Fixed GetVCenter to handle removed FDs better |
| csi-op#348 | [openshift/vmware-vsphere-csi-driver-operator#348](https://github.com/openshift/vmware-vsphere-csi-driver-operator/pull/348) | | FD lifecycle: orphan tag cleanup, SPBM profile management, PV-safe detach |
