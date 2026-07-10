package framework

import "time"

const (
	InfrastructureName          = "cluster"
	FeatureGateName             = "cluster"
	VSphereMultiVCenterDay2Gate = "VSphereMultiVCenterDay2"

	ManagedConfigNamespace = "openshift-config-managed"
	ManagedConfigName      = "kube-cloud-config"
	SourceConfigNamespace  = "openshift-config"
	SourceConfigName       = "cloud-provider-config"
	CloudCredentialsSecret = "cloud-credentials"
	VSphereCredsNamespace  = "kube-system"
	VSphereCredsSecret     = "vsphere-creds"
	VSphereMachineCredsSecret = "vsphere-cloud-credentials"
	CCMConfigNamespace     = "openshift-cloud-controller-manager"
	CCMConfigName          = "cloud-conf"
	MachineAPINamespace    = "openshift-machine-api"

	CloudConfigDataKey        = "cloud.conf"
	SourceCloudConfigDataKey  = "config"

	VAPMachineFailureDomainName    = "vsphere-failure-domain-in-use-by-machine"
	VAPCPMSFailureDomainName       = "vsphere-failure-domain-in-use-by-cpms"
	VAPMachineSetFailureDomainName = "vsphere-failure-domain-in-use-by-machineset"

	MachineRegionLabel = "machine.openshift.io/region"
	MachineZoneLabel   = "machine.openshift.io/zone"

	CSIDriverNamespace       = "openshift-cluster-csi-drivers"
	CSIDriverControllerLabel = "app=vmware-vsphere-csi-driver-controller"
	CSIDriverNodeLabel       = "app=vmware-vsphere-csi-driver-node"
	CSICredentialSecretName  = "vmware-vsphere-cloud-credentials"
	CSITopologyKeyPrefix = "topology.csi.vmware.com/"
	CSIDriverConfigSecretName = "vsphere-csi-config-secret"
	ClusterCSIDriverName     = "csi.vsphere.vmware.com"
	StorageOperatorName      = "storage"

	MCONamespace        = "openshift-machine-config-operator"
	CoreOSBootImagesCM  = "coreos-bootimages"

	TestPVCSize         = "1Gi"
	TestNamespacePrefix = "e2e-csi-storage"
	BusyboxImage        = "registry.k8s.io/e2e-test-images/busybox:1.36.1-1"

	// CSI operator lifecycle constants (from vmware-vsphere-csi-driver-operator PR #348, commit acb68c32)
	OrphanCleanupPendingCondition = "VMwareVSphereDriverStorageClassControllerOrphanCleanupPending"
	ForceOrphanCleanupAnnotation  = "csi.vsphere.vmware.com/force-orphan-cleanup"
	TagOperationsMetric           = "vsphere_csi_tag_operations_total"
	OrphanTagsDetectedMetric      = "vsphere_csi_orphan_tags_detected_total"

	// CSI topology configuration constants (csi_topology_config_test.go)
	CSIControllerDeployment = "vmware-vsphere-csi-driver-controller"
	CSIProvisionerContainer = "csi-provisioner"
	FeatureStatesConfigMap  = "internal-feature-states.csi.vsphere.vmware.com"
	TopologyTagsMetric      = "vsphere_topology_tags"
	TopologyTagsSourceInfra = "infrastructure"
	TopologyTagsSourceCCD   = "clustercsidriver"

DefaultTimeout = 5 * time.Minute
	DefaultPolling = 10 * time.Second
	ShortTimeout   = 30 * time.Second
	LongTimeout    = 15 * time.Minute
	OperatorSyncTimeout = 12 * time.Minute

	PerfTimeout = 60 * time.Minute
)
