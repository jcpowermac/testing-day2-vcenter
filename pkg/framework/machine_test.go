package framework

import (
	"context"
	"testing"
	"time"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	machinefake "github.com/openshift/client-go/machine/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestWaitForMachineSetMachinesIgnoresDeletingMachines(t *testing.T) {
	t.Parallel()

	running := "Running"
	now := metav1.Now()
	client := machinefake.NewSimpleClientset(
		&machinev1beta1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "terminating-machine",
				Namespace:         MachineAPINamespace,
				DeletionTimestamp: &now,
				Labels: map[string]string{
					"machine.openshift.io/cluster-api-machineset": "test-ms",
				},
			},
			Status: machinev1beta1.MachineStatus{
				Phase: &running,
			},
		},
	)

	err := WaitForMachineSetMachines(context.Background(), client, "test-ms", 1)
	if err == nil {
		t.Fatalf("expected deleting running Machine to be ignored")
	}
}

func TestWaitForMachineSetMachinesCountsNonDeletingRunningMachines(t *testing.T) {
	t.Parallel()

	running := "Running"
	client := machinefake.NewSimpleClientset(
		&machinev1beta1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "running-machine",
				Namespace: MachineAPINamespace,
				Labels: map[string]string{
					"machine.openshift.io/cluster-api-machineset": "test-ms",
				},
			},
			Status: machinev1beta1.MachineStatus{
				Phase: &running,
			},
		},
	)

	if err := WaitForMachineSetMachines(context.Background(), client, "test-ms", 1); err != nil {
		t.Fatalf("expected non-deleting running Machine to count as ready: %v", err)
	}
}

func TestFindReadyNodeInTopology(t *testing.T) {
	t.Parallel()

	readyNode := makeNode("ready-node", map[string]string{
		"topology.csi.vmware.com/region": "region-a",
		"topology.csi.vmware.com/zone":   "zone-a",
	}, true, false)

	notReadyNode := makeNode("not-ready-node", map[string]string{
		"topology.csi.vmware.com/region": "region-a",
		"topology.csi.vmware.com/zone":   "zone-a",
	}, false, false)

	deletingNode := makeNode("deleting-node", map[string]string{
		"topology.csi.vmware.com/region": "region-a",
		"topology.csi.vmware.com/zone":   "zone-a",
	}, true, true)

	client := fake.NewSimpleClientset(notReadyNode, deletingNode, readyNode)
	keys := &CSITopologyKeys{
		Region: "topology.csi.vmware.com/region",
		Zone:   "topology.csi.vmware.com/zone",
	}

	name, found, err := FindReadyNodeInTopology(context.Background(), client, keys, "region-a", "zone-a")
	if err != nil {
		t.Fatalf("unexpected error finding ready node: %v", err)
	}
	if !found {
		t.Fatalf("expected to find ready node in topology")
	}
	if name != "ready-node" {
		t.Fatalf("expected ready-node, got %q", name)
	}
}

func TestWaitForReadyNodeInTopologyTimesOutWithoutReadyNode(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset(makeNode("not-ready-node", map[string]string{
		"topology.csi.vmware.com/region": "region-a",
		"topology.csi.vmware.com/zone":   "zone-a",
	}, false, false))
	keys := &CSITopologyKeys{
		Region: "topology.csi.vmware.com/region",
		Zone:   "topology.csi.vmware.com/zone",
	}

	_, err := WaitForReadyNodeInTopology(context.Background(), client, keys, "region-a", "zone-a", 10*time.Millisecond)
	if err == nil {
		t.Fatalf("expected timeout when no matching ready node exists")
	}
}

func makeNode(name string, labels map[string]string, ready bool, deleting bool) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionFalse,
				},
			},
		},
	}
	if ready {
		node.Status.Conditions[0].Status = corev1.ConditionTrue
	}
	if deleting {
		now := metav1.Now()
		node.DeletionTimestamp = &now
	}
	return node
}
