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
	ClusterCSIDriverName     = "csi.vsphere.vmware.com"
	StorageOperatorName      = "storage"

	MCONamespace        = "openshift-machine-config-operator"
	CoreOSBootImagesCM  = "coreos-bootimages"

	TestPVCSize         = "1Gi"
	TestNamespacePrefix = "e2e-csi-storage"
	BusyboxImage        = "registry.k8s.io/e2e-test-images/busybox:1.36.1"

	DefaultTimeout = 5 * time.Minute
	DefaultPolling = 10 * time.Second
	ShortTimeout   = 30 * time.Second
	LongTimeout    = 15 * time.Minute
)
