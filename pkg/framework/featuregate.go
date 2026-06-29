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

func gateEnabledInDetails(gateName string, details configv1.FeatureGateDetails) (bool, bool) {
	for _, gate := range details.Enabled {
		if string(gate.Name) == gateName {
			return true, true
		}
	}
	for _, gate := range details.Disabled {
		if string(gate.Name) == gateName {
			return false, true
		}
	}
	return false, false
}

// IsFeatureGateEnabled checks whether gateName is enabled on FeatureGate/cluster.
func IsFeatureGateEnabled(ctx context.Context, client configclient.Interface, gateName string) (bool, error) {
	fg, err := client.ConfigV1().FeatureGates().Get(ctx, FeatureGateName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("get featuregate: %w", err)
	}

	for _, details := range fg.Status.FeatureGates {
		enabled, found := gateEnabledInDetails(gateName, details)
		if found {
			return enabled, nil
		}
	}
	return false, nil
}

// GetFeatureGateAttributes returns enabled state and version for gateName if present.
func GetFeatureGateAttributes(ctx context.Context, client configclient.Interface, gateName string) (enabled bool, version string, found bool, err error) {
	fg, err := client.ConfigV1().FeatureGates().Get(ctx, FeatureGateName, metav1.GetOptions{})
	if err != nil {
		return false, "", false, fmt.Errorf("get featuregate: %w", err)
	}

	for _, details := range fg.Status.FeatureGates {
		if ok, present := gateEnabledInDetails(gateName, details); present {
			return ok, details.Version, true, nil
		}
	}
	return false, "", false, nil
}

// WaitForFeatureGate polls until gateName matches the expected enabled state.
func WaitForFeatureGate(ctx context.Context, client configclient.Interface, gateName string, enabled bool, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, DefaultPolling, timeout, true, func(ctx context.Context) (bool, error) {
		current, err := IsFeatureGateEnabled(ctx, client, gateName)
		if err != nil {
			return false, err
		}
		return current == enabled, nil
	})
}
