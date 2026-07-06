package vsphere

import (
	"context"
	"fmt"
	"strings"

	"github.com/vmware/govmomi/pbm"
	"k8s.io/klog/v2"
)

// StorageProfileExists checks whether a storage profile with the given name
// exists on the vCenter that the session is connected to.
func StorageProfileExists(ctx context.Context, s *Session, profileName string) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("checking storage profile existence", "profile", profileName)

	client, err := pbm.NewClient(ctx, s.Client.Client)
	if err != nil {
		return false, fmt.Errorf("creating PBM client: %w", err)
	}

	id, err := client.ProfileIDByName(ctx, profileName)
	if err != nil {
		if strings.Contains(err.Error(), "no pbm profile found with name") {
			log.V(2).Info("storage profile not found", "profile", profileName)
			return false, nil
		}
		return false, fmt.Errorf("looking up storage profile %q: %w", profileName, err)
	}

	log.V(2).Info("storage profile found", "profile", profileName, "id", id)
	return true, nil
}
