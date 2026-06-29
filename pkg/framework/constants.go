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

	DefaultTimeout = 5 * time.Minute
	DefaultPolling = 10 * time.Second
	ShortTimeout   = 30 * time.Second
	LongTimeout    = 15 * time.Minute
)
