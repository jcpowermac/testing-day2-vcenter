package vsphere

import (
	configv1 "github.com/openshift/api/config/v1"
)

// CloneInfrastructureSpec returns a deep copy of the Infrastructure spec.
func CloneInfrastructureSpec(spec configv1.InfrastructureSpec) configv1.InfrastructureSpec {
	return *spec.DeepCopy()
}

// CloneVCenter returns a deep copy of a vCenter spec entry.
func CloneVCenter(v configv1.VSpherePlatformVCenterSpec) configv1.VSpherePlatformVCenterSpec {
	return *v.DeepCopy()
}

// CloneFailureDomain returns a deep copy of a failure domain spec entry.
func CloneFailureDomain(fd configv1.VSpherePlatformFailureDomainSpec) configv1.VSpherePlatformFailureDomainSpec {
	return *fd.DeepCopy()
}

// VCenterServers returns server hostnames from vCenter entries.
func VCenterServers(vcenters []configv1.VSpherePlatformVCenterSpec) []string {
	servers := make([]string, 0, len(vcenters))
	for _, vc := range vcenters {
		servers = append(servers, vc.Server)
	}
	return servers
}

// FailureDomainKey returns region/zone tuple used by Machines and VAPs.
func FailureDomainKey(region, zone string) string {
	return region + "/" + zone
}

// FindFailureDomainByRegionZone finds a failure domain by region and zone labels.
func FindFailureDomainByRegionZone(fds []configv1.VSpherePlatformFailureDomainSpec, region, zone string) *configv1.VSpherePlatformFailureDomainSpec {
	for i := range fds {
		if fds[i].Region == region && fds[i].Zone == zone {
			return &fds[i]
		}
	}
	return nil
}

// RemoveFailureDomainByRegionZone returns a copy of failure domains with one region/zone removed.
func RemoveFailureDomainByRegionZone(fds []configv1.VSpherePlatformFailureDomainSpec, region, zone string) []configv1.VSpherePlatformFailureDomainSpec {
	out := make([]configv1.VSpherePlatformFailureDomainSpec, 0, len(fds))
	for _, fd := range fds {
		if fd.Region == region && fd.Zone == zone {
			continue
		}
		out = append(out, fd)
	}
	return out
}

// RemoveVCenterByServer returns a copy of vCenters with the given server removed.
func RemoveVCenterByServer(vcenters []configv1.VSpherePlatformVCenterSpec, server string) []configv1.VSpherePlatformVCenterSpec {
	out := make([]configv1.VSpherePlatformVCenterSpec, 0, len(vcenters))
	for _, vc := range vcenters {
		if vc.Server == server {
			continue
		}
		out = append(out, vc)
	}
	return out
}
