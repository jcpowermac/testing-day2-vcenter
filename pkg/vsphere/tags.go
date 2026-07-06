package vsphere

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vapi/tags"
	"k8s.io/klog/v2"
)

// GetClusterTagCategoryName returns the tag category name used by OpenShift
// for the given cluster infrastructure ID.
func GetClusterTagCategoryName(infraID string) string {
	return "openshift-" + infraID
}

// GetClusterTagName returns the tag name used by OpenShift for the given
// cluster infrastructure ID. The tag name is the infra ID itself.
func GetClusterTagName(infraID string) string {
	return infraID
}

// GetStoragePolicyName returns the SPBM storage policy name used by OpenShift
// for the given cluster infrastructure ID.
func GetStoragePolicyName(infraID string) string {
	return "openshift-storage-policy-" + infraID
}

// FindTagCategoryByName searches for a tag category with the given name.
// Returns nil, nil if no category with that name exists.
func FindTagCategoryByName(ctx context.Context, s *Session, name string) (*tags.Category, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("searching for tag category", "name", name)

	categories, err := s.TagManager.GetCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing tag categories: %w", err)
	}

	for i := range categories {
		if categories[i].Name == name {
			return &categories[i], nil
		}
	}

	return nil, nil
}

// FindTagByName searches for a tag with the given name within a category.
// Returns nil, nil if no tag with that name exists in the category.
func FindTagByName(ctx context.Context, s *Session, categoryID, name string) (*tags.Tag, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("searching for tag", "categoryID", categoryID, "name", name)

	tagList, err := s.TagManager.GetTagsForCategory(ctx, categoryID)
	if err != nil {
		return nil, fmt.Errorf("listing tags for category %s: %w", categoryID, err)
	}

	for i := range tagList {
		if tagList[i].Name == name {
			return &tagList[i], nil
		}
	}

	return nil, nil
}

// IsDatastoreTagged checks whether the datastore at the given inventory path
// has a tag with the specified name attached.
func IsDatastoreTagged(ctx context.Context, s *Session, datastorePath string, tagName string) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("checking datastore tag", "datastore", datastorePath, "tag", tagName)

	ds, err := s.Finder.Datastore(ctx, datastorePath)
	if err != nil {
		return false, fmt.Errorf("finding datastore %q: %w", datastorePath, err)
	}

	ref := ds.Reference()
	attachedTags, err := s.TagManager.GetAttachedTags(ctx, ref)
	if err != nil {
		return false, fmt.Errorf("getting attached tags for datastore %q: %w", datastorePath, err)
	}

	for _, t := range attachedTags {
		if t.Name == tagName {
			return true, nil
		}
	}

	return false, nil
}

// ListTaggedDatastoreNames returns the names of all datastores that have the
// given tag attached. Datastores whose names cannot be resolved are logged
// and skipped.
func ListTaggedDatastoreNames(ctx context.Context, s *Session, tagID string) ([]string, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("listing datastores for tag", "tagID", tagID)

	refs, err := s.TagManager.ListAttachedObjects(ctx, tagID)
	if err != nil {
		return nil, fmt.Errorf("listing objects attached to tag %s: %w", tagID, err)
	}

	var names []string
	for _, ref := range refs {
		mor := ref.Reference()
		if mor.Type != "Datastore" {
			continue
		}

		ds := object.NewDatastore(s.Client.Client, mor)
		name, err := ds.ObjectName(ctx)
		if err != nil {
			log.V(1).Info("failed to resolve datastore name, skipping",
				"mor", mor.Value, "err", err)
			continue
		}

		names = append(names, name)
	}

	return names, nil
}
