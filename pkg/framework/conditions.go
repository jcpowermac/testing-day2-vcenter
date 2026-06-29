package framework

import (
	"context"
	"fmt"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
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
func WaitForClusterOperatorAvailable(ctx context.Context, client configclient.Interface, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, DefaultPolling, timeout, true, func(ctx context.Context) (bool, error) {
		available, err := GetClusterOperatorCondition(ctx, client, name, configv1.OperatorAvailable)
		if err != nil {
			return false, err
		}
		degraded, err := GetClusterOperatorCondition(ctx, client, name, configv1.OperatorDegraded)
		if err != nil {
			return false, err
		}
		return available.Status == configv1.ConditionTrue && degraded.Status == configv1.ConditionFalse, nil
	})
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
