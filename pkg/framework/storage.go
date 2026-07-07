package framework

import (
	"context"
	"fmt"
	"strings"
	"time"

	operatorv1 "github.com/openshift/api/operator/v1"
	operatorclient "github.com/openshift/client-go/operator/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// CSITopologyKeys holds the discovered CSI topology label keys for a cluster.
type CSITopologyKeys struct {
	Region string
	Zone   string
}

// DiscoverCSITopologyKeys reads CSINode objects to find the actual topology keys
// registered by the vSphere CSI driver. The keys depend on vCenter tag category
// names (e.g. "openshift-region" → "topology.csi.vmware.com/openshift-region").
func DiscoverCSITopologyKeys(ctx context.Context, client kubernetes.Interface) (*CSITopologyKeys, error) {
	csiNodes, err := client.StorageV1().CSINodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list CSINodes: %w", err)
	}

	for _, csiNode := range csiNodes.Items {
		for _, driver := range csiNode.Spec.Drivers {
			if driver.Name != ClusterCSIDriverName {
				continue
			}
			keys := &CSITopologyKeys{}
			for _, key := range driver.TopologyKeys {
				if !strings.HasPrefix(key, CSITopologyKeyPrefix) {
					continue
				}
				suffix := strings.TrimPrefix(key, CSITopologyKeyPrefix)
				if strings.Contains(suffix, "region") {
					keys.Region = key
				} else if strings.Contains(suffix, "zone") {
					keys.Zone = key
				}
			}
			if keys.Region != "" && keys.Zone != "" {
				return keys, nil
			}
		}
	}
	return nil, fmt.Errorf("no CSINode has vSphere CSI topology keys with prefix %s", CSITopologyKeyPrefix)
}

// CreateTestNamespace creates a namespace with a generated suffix for test isolation.
// It waits for the SCC admission controller to annotate the namespace before returning.
func CreateTestNamespace(ctx context.Context, client kubernetes.Interface, prefix string) (string, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: prefix + "-",
			Labels: map[string]string{
				"e2e-test": "csi-storage",
			},
		},
	}
	created, err := client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("create test namespace: %w", err)
	}
	err = wait.PollUntilContextTimeout(ctx, time.Second, ShortTimeout, true, func(ctx context.Context) (bool, error) {
		got, err := client.CoreV1().Namespaces().Get(ctx, created.Name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		_, ok := got.Annotations["openshift.io/sa.scc.uid-range"]
		return ok, nil
	})
	if err != nil {
		return "", fmt.Errorf("namespace %s not annotated with SCC uid-range: %w", created.Name, err)
	}
	return created.Name, nil
}

// DeleteNamespace deletes a namespace and waits for termination.
func DeleteNamespace(ctx context.Context, client kubernetes.Interface, name string, timeout time.Duration) error {
	err := client.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete namespace %s: %w", name, err)
	}
	lastLog := time.Now()
	return wait.PollUntilContextTimeout(ctx, DefaultPolling, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := client.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if time.Since(lastLog) >= 30*time.Second {
			fmt.Printf("  wait: namespace %s still terminating\n", name)
			lastLog = time.Now()
		}
		return false, nil
	})
}

// GetStorageClass fetches a StorageClass by name.
func GetStorageClass(ctx context.Context, client kubernetes.Interface, name string) (*storagev1.StorageClass, error) {
	return client.StorageV1().StorageClasses().Get(ctx, name, metav1.GetOptions{})
}

// GetDefaultStorageClass finds the StorageClass with the default annotation.
func GetDefaultStorageClass(ctx context.Context, client kubernetes.Interface) (*storagev1.StorageClass, error) {
	scs, err := client.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list storage classes: %w", err)
	}
	for i := range scs.Items {
		if scs.Items[i].Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			return &scs.Items[i], nil
		}
	}
	return nil, fmt.Errorf("no default StorageClass found")
}

// CloneStorageClassWithTopology clones a StorageClass with new name and allowedTopologies.
func CloneStorageClassWithTopology(ctx context.Context, client kubernetes.Interface, source *storagev1.StorageClass, name string, topologyTerms []corev1.TopologySelectorTerm) (*storagev1.StorageClass, error) {
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Provisioner:          source.Provisioner,
		Parameters:           source.Parameters,
		ReclaimPolicy:        source.ReclaimPolicy,
		VolumeBindingMode:    source.VolumeBindingMode,
		AllowedTopologies:    topologyTerms,
		MountOptions:         source.MountOptions,
		AllowVolumeExpansion: source.AllowVolumeExpansion,
	}
	return client.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
}

// DeleteStorageClass deletes a StorageClass by name.
func DeleteStorageClass(ctx context.Context, client kubernetes.Interface, name string) error {
	err := client.StorageV1().StorageClasses().Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// StorageClassIsWaitForFirstConsumer returns true if the SC uses WaitForFirstConsumer binding mode.
func StorageClassIsWaitForFirstConsumer(sc *storagev1.StorageClass) bool {
	return sc.VolumeBindingMode != nil && *sc.VolumeBindingMode == storagev1.VolumeBindingWaitForFirstConsumer
}

// CreatePVC creates a PersistentVolumeClaim. Pass storageClassName "" to use the cluster default.
func CreatePVC(ctx context.Context, client kubernetes.Interface, namespace, name, size, storageClassName string) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
		},
	}
	if storageClassName != "" {
		pvc.Spec.StorageClassName = &storageClassName
	}
	return client.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
}

// DeletePVC deletes a PersistentVolumeClaim.
func DeletePVC(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	err := client.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// WaitForPVCBound polls until the PVC reaches Bound phase or timeout.
func WaitForPVCBound(ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) (*corev1.PersistentVolumeClaim, error) {
	var result *corev1.PersistentVolumeClaim
	lastLog := time.Now()
	err := wait.PollUntilContextTimeout(ctx, DefaultPolling, timeout, true, func(ctx context.Context) (bool, error) {
		pvc, err := client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		if pvc.Status.Phase == corev1.ClaimBound {
			result = pvc
			return true, nil
		}
		if time.Since(lastLog) >= 30*time.Second {
			fmt.Printf("  wait: PVC %s/%s phase=%s\n", namespace, name, pvc.Status.Phase)
			lastLog = time.Now()
		}
		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("PVC %s/%s not bound after %v: %w", namespace, name, timeout, err)
	}
	return result, nil
}

// GetPV fetches a PersistentVolume by name.
func GetPV(ctx context.Context, client kubernetes.Interface, name string) (*corev1.PersistentVolume, error) {
	return client.CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
}

// DeletePV deletes a PersistentVolume.
func DeletePV(ctx context.Context, client kubernetes.Interface, name string) error {
	err := client.CoreV1().PersistentVolumes().Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// WaitForPVDeleted polls until the PV no longer exists.
func WaitForPVDeleted(ctx context.Context, client kubernetes.Interface, name string, timeout time.Duration) error {
	lastLog := time.Now()
	return wait.PollUntilContextTimeout(ctx, DefaultPolling, timeout, true, func(ctx context.Context) (bool, error) {
		pv, err := client.CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err == nil && time.Since(lastLog) >= 30*time.Second {
			fmt.Printf("  wait: PV %s phase=%s still exists\n", name, pv.Status.Phase)
			lastLog = time.Now()
		}
		return false, nil
	})
}

// PVTopologyLabels extracts CSI topology region/zone from PV nodeAffinity
// using the given topology keys.
func PVTopologyLabels(pv *corev1.PersistentVolume, keys *CSITopologyKeys) (region, zone string, found bool) {
	if pv.Spec.NodeAffinity == nil || pv.Spec.NodeAffinity.Required == nil {
		return "", "", false
	}
	for _, term := range pv.Spec.NodeAffinity.Required.NodeSelectorTerms {
		for _, expr := range term.MatchExpressions {
			switch expr.Key {
			case keys.Region:
				if expr.Operator == corev1.NodeSelectorOpIn && len(expr.Values) > 0 {
					region = expr.Values[0]
				}
			case keys.Zone:
				if expr.Operator == corev1.NodeSelectorOpIn && len(expr.Values) > 0 {
					zone = expr.Values[0]
				}
			}
		}
	}
	return region, zone, region != "" && zone != ""
}

// PVBelongsToFailureDomain checks if a PV's topology labels match the given region/zone.
func PVBelongsToFailureDomain(pv *corev1.PersistentVolume, keys *CSITopologyKeys, region, zone string) bool {
	pvRegion, pvZone, ok := PVTopologyLabels(pv, keys)
	if !ok {
		return false
	}
	return pvRegion == region && pvZone == zone
}

// ListPVsInFailureDomain returns PVs whose topology labels match the given region/zone.
func ListPVsInFailureDomain(ctx context.Context, client kubernetes.Interface, keys *CSITopologyKeys, region, zone string) ([]corev1.PersistentVolume, error) {
	pvs, err := client.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list persistent volumes: %w", err)
	}
	var matched []corev1.PersistentVolume
	for i := range pvs.Items {
		if PVBelongsToFailureDomain(&pvs.Items[i], keys, region, zone) {
			matched = append(matched, pvs.Items[i])
		}
	}
	return matched, nil
}

// CreateBusyboxPod creates a minimal Pod mounting the given PVC.
func CreateBusyboxPod(ctx context.Context, client kubernetes.Interface, namespace, name, pvcName string) (*corev1.Pod, error) {
	return CreateBusyboxPodWithNodeSelector(ctx, client, namespace, name, pvcName, nil)
}

// CreateBusyboxPodWithNodeSelector creates a Pod with an optional nodeSelector mounting the given PVC.
func CreateBusyboxPodWithNodeSelector(ctx context.Context, client kubernetes.Interface, namespace, name, pvcName string, nodeSelector map[string]string) (*corev1.Pod, error) {
	allowEscalation := false
	runAsNonRoot := true
	var runAsUser int64 = 1000
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			NodeSelector: nodeSelector,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: &runAsNonRoot,
				RunAsUser:    &runAsUser,
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   BusyboxImage,
					Command: []string{"sleep", "infinity"},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: &allowEscalation,
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "data", MountPath: "/data"},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "data",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}
	return client.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
}

// DeletePod deletes a Pod.
func DeletePod(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	err := client.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// WaitForPodRunning polls until a Pod reaches Running phase.
func WaitForPodRunning(ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) error {
	lastLog := time.Now()
	return wait.PollUntilContextTimeout(ctx, DefaultPolling, timeout, true, func(ctx context.Context) (bool, error) {
		pod, err := client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		if pod.Status.Phase == corev1.PodRunning {
			return true, nil
		}
		if time.Since(lastLog) >= 30*time.Second {
			reason := ""
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.State.Waiting != nil {
					reason = cs.State.Waiting.Reason
				}
			}
			if reason != "" {
				fmt.Printf("  wait: pod %s/%s phase=%s reason=%s\n", namespace, name, pod.Status.Phase, reason)
			} else {
				fmt.Printf("  wait: pod %s/%s phase=%s\n", namespace, name, pod.Status.Phase)
			}
			lastLog = time.Now()
		}
		return false, nil
	})
}

// GetClusterCSIDriverCondition returns a condition from a ClusterCSIDriver.
func GetClusterCSIDriverCondition(ctx context.Context, client operatorclient.Interface, name string, conditionType string) (*operatorv1.OperatorCondition, error) {
	csi, err := client.OperatorV1().ClusterCSIDrivers().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get clustercsidriver %q: %w", name, err)
	}
	for i := range csi.Status.Conditions {
		if csi.Status.Conditions[i].Type == conditionType {
			return &csi.Status.Conditions[i], nil
		}
	}
	return nil, fmt.Errorf("condition %q not found on clustercsidriver %q", conditionType, name)
}

// WaitForClusterCSIDriverAvailable waits until Available=True and not Degraded.
func WaitForClusterCSIDriverAvailable(ctx context.Context, client operatorclient.Interface, name string, timeout time.Duration) error {
	var lastErr error
	lastLog := time.Now()
	pollErr := wait.PollUntilContextTimeout(ctx, DefaultPolling, timeout, true, func(ctx context.Context) (bool, error) {
		csi, err := client.OperatorV1().ClusterCSIDrivers().Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			lastErr = fmt.Errorf("clustercsidriver %q not found", name)
			return false, nil
		}
		if err != nil {
			lastErr = err
			return false, nil
		}

		var availableOK, degradedOK bool
		for i := range csi.Status.Conditions {
			switch csi.Status.Conditions[i].Type {
			case "Available":
				availableOK = csi.Status.Conditions[i].Status == operatorv1.ConditionTrue
			case "Degraded":
				degradedOK = csi.Status.Conditions[i].Status == operatorv1.ConditionFalse
			}
		}
		if !availableOK || !degradedOK {
			if time.Since(lastLog) >= 30*time.Second {
				fmt.Printf("  wait: CSI driver %s: Available=%v Degraded=%v\n", name, availableOK, !degradedOK)
				lastLog = time.Now()
			}
		}
		lastErr = nil
		return availableOK && degradedOK, nil
	})
	if pollErr != nil && lastErr != nil {
		return fmt.Errorf("%w: last error: %v", pollErr, lastErr)
	}
	return pollErr
}

// GetCSIDriverConfig reads the CSI driver cloud config from the operator namespace.
// The operator writes this as Secret "vsphere-csi-config-secret" with key "cloud.conf".
func GetCSIDriverConfig(ctx context.Context, client kubernetes.Interface) (string, error) {
	secret, err := client.CoreV1().Secrets(CSIDriverNamespace).Get(ctx, CSIDriverConfigSecretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("CSI driver config Secret %s/%s not found: %v",
			CSIDriverNamespace, CSIDriverConfigSecretName, err)
	}
	data, ok := secret.Data[CloudConfigDataKey]
	if !ok || len(data) == 0 {
		return "", fmt.Errorf("CSI driver config Secret %s/%s has no %q key",
			CSIDriverNamespace, CSIDriverConfigSecretName, CloudConfigDataKey)
	}
	return string(data), nil
}

// CSIConfigHasVCenter checks if a cloud config string references a vCenter hostname.
func CSIConfigHasVCenter(config, vcenterHost string) bool {
	return strings.Contains(config, vcenterHost)
}
