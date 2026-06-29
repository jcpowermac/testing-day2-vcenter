package framework

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// ConfigMapSnapshot captures ConfigMap data and metadata for comparison.
type ConfigMapSnapshot struct {
	Namespace       string
	Name            string
	Data            map[string]string
	Annotations     map[string]string
	ResourceVersion string
}

// Diff describes a semantic difference between two cloud config snapshots.
type Diff struct {
	Path     string
	Expected string
	Actual   string
}

// GetConfigMap fetches a ConfigMap by namespace and name.
func GetConfigMap(ctx context.Context, client kubernetes.Interface, namespace, name string) (*corev1.ConfigMap, error) {
	cm, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get configmap %s/%s: %w", namespace, name, err)
	}
	return cm, nil
}

// SnapshotConfigMap captures ConfigMap state for later comparison or restore.
func SnapshotConfigMap(ctx context.Context, client kubernetes.Interface, namespace, name string) (*ConfigMapSnapshot, error) {
	cm, err := GetConfigMap(ctx, client, namespace, name)
	if err != nil {
		return nil, err
	}

	dataCopy := make(map[string]string, len(cm.Data))
	for k, v := range cm.Data {
		dataCopy[k] = v
	}
	annotationsCopy := make(map[string]string, len(cm.Annotations))
	for k, v := range cm.Annotations {
		annotationsCopy[k] = v
	}

	return &ConfigMapSnapshot{
		Namespace:       cm.Namespace,
		Name:            cm.Name,
		Data:            dataCopy,
		Annotations:     annotationsCopy,
		ResourceVersion: cm.ResourceVersion,
	}, nil
}

// RestoreConfigMapFromSnapshot replaces ConfigMap data and annotations.
func RestoreConfigMapFromSnapshot(ctx context.Context, client kubernetes.Interface, snapshot *ConfigMapSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("configmap snapshot is nil")
	}

	cm, err := GetConfigMap(ctx, client, snapshot.Namespace, snapshot.Name)
	if err != nil {
		return err
	}

	cm.Data = snapshot.Data
	cm.Annotations = snapshot.Annotations

	_, err = client.CoreV1().ConfigMaps(snapshot.Namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("restore configmap %s/%s: %w", snapshot.Namespace, snapshot.Name, err)
	}
	return nil
}

// CompareConfigMapData compares a single data key between two snapshots.
func CompareConfigMapData(a, b *ConfigMapSnapshot, key string) []Diff {
	if a == nil || b == nil {
		return []Diff{{Path: key, Expected: "<snapshot>", Actual: "<missing snapshot>"}}
	}
	aVal := a.Data[key]
	bVal := b.Data[key]
	if aVal == bVal {
		return nil
	}
	return []Diff{{Path: key, Expected: aVal, Actual: bVal}}
}

// GetConfigMapOwner returns a best-effort managing operator identifier from annotations.
func GetConfigMapOwner(cm *corev1.ConfigMap) string {
	if cm == nil {
		return ""
	}

	candidates := []string{
		"config.openshift.io/infrastructure",
		"openshift.io/cluster-monitoring",
		"cluster.cloudconfig.operator/owning-component",
	}

	for _, key := range candidates {
		if v, ok := cm.Annotations[key]; ok && v != "" {
			return v
		}
	}

	for key, value := range cm.Annotations {
		if strings.Contains(key, "operator") || strings.Contains(key, "owner") {
			return fmt.Sprintf("%s=%s", key, value)
		}
	}
	return ""
}

// WaitForConfigMapStable polls until resourceVersion stops changing.
func WaitForConfigMapStable(ctx context.Context, client kubernetes.Interface, namespace, name string, window time.Duration) error {
	var lastVersion string
	stableSince := time.Now()

	return wait.PollUntilContextTimeout(ctx, 2*time.Second, window+30*time.Second, true, func(ctx context.Context) (bool, error) {
		cm, err := GetConfigMap(ctx, client, namespace, name)
		if err != nil {
			return false, err
		}

		if cm.ResourceVersion != lastVersion {
			lastVersion = cm.ResourceVersion
			stableSince = time.Now()
			return false, nil
		}

		return time.Since(stableSince) >= window, nil
	})
}

// SortedConfigMapKeys returns sorted keys from ConfigMap data.
func SortedConfigMapKeys(cm *corev1.ConfigMap) []string {
	if cm == nil {
		return nil
	}
	keys := make([]string, 0, len(cm.Data))
	for k := range cm.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
