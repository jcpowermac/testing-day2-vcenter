package framework

import (
	"context"
	"fmt"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// GetClusterOperatorCondition returns a condition from a ClusterOperator.
func GetClusterOperatorCondition(ctx context.Context, client configclient.Interface, name string, conditionType configv1.ClusterStatusConditionType) (*configv1.ClusterOperatorStatusCondition, error) {
	co, err := client.ConfigV1().ClusterOperators().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get clusteroperator %q: %w", name, err)
	}

	for i := range co.Status.Conditions {
		if co.Status.Conditions[i].Type == conditionType {
			return &co.Status.Conditions[i], nil
		}
	}
	return nil, fmt.Errorf("condition %q not found on clusteroperator %q", conditionType, name)
}

// WaitForClusterOperatorAvailable waits until Available=True and Degraded=False.
// Transient errors (not-found, condition missing) are retried until timeout.
func WaitForClusterOperatorAvailable(ctx context.Context, client configclient.Interface, name string, timeout time.Duration) error {
	var lastErr error
	pollErr := wait.PollUntilContextTimeout(ctx, DefaultPolling, timeout, true, func(ctx context.Context) (bool, error) {
		co, err := client.ConfigV1().ClusterOperators().Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			lastErr = fmt.Errorf("clusteroperator %q not found", name)
			return false, nil
		}
		if err != nil {
			lastErr = err
			return false, nil
		}

		var availableOK, degradedOK bool
		for i := range co.Status.Conditions {
			switch co.Status.Conditions[i].Type {
			case configv1.OperatorAvailable:
				availableOK = co.Status.Conditions[i].Status == configv1.ConditionTrue
			case configv1.OperatorDegraded:
				degradedOK = co.Status.Conditions[i].Status == configv1.ConditionFalse
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

// WaitForClusterOperatorStable waits until Available=True, Degraded=False, and
// Progressing=False. This confirms the operator has finished rolling out.
func WaitForClusterOperatorStable(ctx context.Context, client configclient.Interface, name string, timeout time.Duration) error {
	var lastErr error
	pollErr := wait.PollUntilContextTimeout(ctx, DefaultPolling, timeout, true, func(ctx context.Context) (bool, error) {
		co, err := client.ConfigV1().ClusterOperators().Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			lastErr = fmt.Errorf("clusteroperator %q not found", name)
			return false, nil
		}
		if err != nil {
			lastErr = err
			return false, nil
		}

		var availableOK, degradedOK, progressingOK bool
		for i := range co.Status.Conditions {
			switch co.Status.Conditions[i].Type {
			case configv1.OperatorAvailable:
				availableOK = co.Status.Conditions[i].Status == configv1.ConditionTrue
			case configv1.OperatorDegraded:
				degradedOK = co.Status.Conditions[i].Status == configv1.ConditionFalse
			case configv1.OperatorProgressing:
				progressingOK = co.Status.Conditions[i].Status == configv1.ConditionFalse
			}
		}
		if !availableOK || !degradedOK || !progressingOK {
			lastErr = fmt.Errorf("clusteroperator %q: Available=%v Degraded=%v Progressing=%v",
				name, availableOK, !degradedOK, !progressingOK)
			return false, nil
		}
		lastErr = nil
		return true, nil
	})
	if pollErr != nil && lastErr != nil {
		return fmt.Errorf("%w: last error: %v", pollErr, lastErr)
	}
	return pollErr
}

// WaitForAllNodesReady polls until every Node has condition Ready=True.
func WaitForAllNodesReady(ctx context.Context, client kubernetes.Interface, timeout time.Duration) error {
	var lastErr error
	pollErr := wait.PollUntilContextTimeout(ctx, DefaultPolling, timeout, true, func(ctx context.Context) (bool, error) {
		nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			lastErr = err
			return false, nil
		}
		var notReady []string
		for _, node := range nodes.Items {
			ready := false
			for _, c := range node.Status.Conditions {
				if c.Type == corev1.NodeReady {
					ready = c.Status == corev1.ConditionTrue
					break
				}
			}
			if !ready {
				notReady = append(notReady, node.Name)
			}
		}
		if len(notReady) > 0 {
			lastErr = fmt.Errorf("%d nodes not Ready: %v", len(notReady), notReady)
			return false, nil
		}
		lastErr = nil
		return true, nil
	})
	if pollErr != nil && lastErr != nil {
		return fmt.Errorf("%w: %v", pollErr, lastErr)
	}
	return pollErr
}

// CheckOperatorsNotDegraded verifies Degraded=False for each operator name.
func CheckOperatorsNotDegraded(ctx context.Context, client configclient.Interface, operators []string) error {
	for _, name := range operators {
		degraded, err := GetClusterOperatorCondition(ctx, client, name, configv1.OperatorDegraded)
		if err != nil {
			return err
		}
		if degraded.Status != configv1.ConditionFalse {
			return fmt.Errorf("clusteroperator %q is degraded: %s", name, degraded.Message)
		}
	}
	return nil
}
