package framework

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ListPodsByLabel lists pods in a namespace matching a label selector.
func ListPodsByLabel(ctx context.Context, client kubernetes.Interface, namespace, labelSelector string) ([]corev1.Pod, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("list pods in %s (selector=%s): %w", namespace, labelSelector, err)
	}
	return pods.Items, nil
}

// PodRestartCount returns total container restart count for a pod.
func PodRestartCount(pod *corev1.Pod) int32 {
	var total int32
	for _, cs := range pod.Status.ContainerStatuses {
		total += cs.RestartCount
	}
	return total
}
