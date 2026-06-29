package framework

import (
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetSecret reads a Secret from the given namespace.
func GetSecret(ctx context.Context, client kubernetes.Interface, namespace, name string) (*corev1.Secret, error) {
	s, err := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get secret %s/%s: %w", namespace, name, err)
	}
	return s, nil
}

// SecretDataKeys returns sorted data key names from a Secret.
func SecretDataKeys(s *corev1.Secret) []string {
	keys := make([]string, 0, len(s.Data))
	for k := range s.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// SecretHasKeyPrefix checks whether a Secret has at least one data key starting with prefix.
func SecretHasKeyPrefix(s *corev1.Secret, prefix string) bool {
	for k := range s.Data {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
