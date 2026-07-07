package vsphere

import (
	"context"
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
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

// resolveTagIDByName finds a tag's ID by name, searching across all categories.
func resolveTagIDByName(ctx context.Context, s *Session, tagName string) (string, error) {
	tagList, err := s.TagManager.GetTags(ctx)
	if err != nil {
		return "", fmt.Errorf("listing tags: %w", err)
	}
	for i := range tagList {
		if tagList[i].Name == tagName {
			return tagList[i].ID, nil
		}
	}
	return "", fmt.Errorf("tag %q not found", tagName)
}

// AttachTagToDatastore attaches the named tag to the datastore at the given
// inventory path. Idempotent — attaching an already-attached tag is a no-op.
func AttachTagToDatastore(ctx context.Context, s *Session, datastorePath, tagName string) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("attaching tag to datastore", "datastore", datastorePath, "tag", tagName)

	ds, err := s.Finder.Datastore(ctx, datastorePath)
	if err != nil {
		return fmt.Errorf("finding datastore %q: %w", datastorePath, err)
	}

	tagID, err := resolveTagIDByName(ctx, s, tagName)
	if err != nil {
		return err
	}

	if err := s.TagManager.AttachTag(ctx, tagID, ds.Reference()); err != nil {
		return fmt.Errorf("attaching tag %q to datastore %q: %w", tagName, datastorePath, err)
	}
	return nil
}

// DetachTagFromDatastore detaches the named tag from the datastore at the given
// inventory path. Idempotent — detaching an already-detached tag is a no-op.
func DetachTagFromDatastore(ctx context.Context, s *Session, datastorePath, tagName string) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("detaching tag from datastore", "datastore", datastorePath, "tag", tagName)

	ds, err := s.Finder.Datastore(ctx, datastorePath)
	if err != nil {
		return fmt.Errorf("finding datastore %q: %w", datastorePath, err)
	}

	tagID, err := resolveTagIDByName(ctx, s, tagName)
	if err != nil {
		return err
	}

	if err := s.TagManager.DetachTag(ctx, tagID, ds.Reference()); err != nil {
		return fmt.Errorf("detaching tag %q from datastore %q: %w", tagName, datastorePath, err)
	}
	return nil
}

// FindNonFDDatastore finds a datastore in the session's datacenter that is not
// referenced by any of the given failure domains' Topology.Datastore path.
// Returns ("", false, nil) if every datastore in the datacenter belongs to a
// failure domain. The chosen datastore is logged for operator visibility,
// since attaching a synthetic orphan tag to the wrong datastore has blast
// radius.
func FindNonFDDatastore(ctx context.Context, s *Session, fds []configv1.VSpherePlatformFailureDomainSpec) (string, bool, error) {
	log := klog.FromContext(ctx)

	fdPaths := map[string]bool{}
	for _, fd := range fds {
		fdPaths[fd.Topology.Datastore] = true
	}

	datastores, err := s.Finder.DatastoreList(ctx, "*")
	if err != nil {
		return "", false, fmt.Errorf("listing datastores in datacenter %q: %w", s.Datacenter, err)
	}

	for _, ds := range datastores {
		path := ds.InventoryPath
		if fdPaths[path] {
			continue
		}
		log.Info("selected non-FD datastore for synthetic orphan tag testing", "datastore", path)
		return path, true, nil
	}

	log.Info("no non-FD datastore found in datacenter", "datacenter", s.Datacenter)
	return "", false, nil
}
