package framework

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
)

// ReleaseDHCPLeases runs `nmcli connection down ovs-if-br-ex` on each node in
// the given MachineSet to send DHCP RELEASE before machine deletion. This
// prevents IP address exhaustion when running back-to-back benchmarks on a
// constrained DHCP pool.
//
// Each node gets a privileged pod that uses nsenter to run nmcli in the host's
// namespaces. After the command runs the node loses network connectivity, so
// pod completion status may not be reported — the function waits only for the
// pod to leave Pending (i.e. the command has started).
//
// Best-effort: individual node failures are logged via logf. Returns an error
// only if zero nodes could be processed.
func ReleaseDHCPLeases(ctx context.Context, kubeClient kubernetes.Interface, machineClient machineclient.Interface, msName string, logf func(string, ...any)) error {
	machines, err := machineClient.MachineV1beta1().Machines(MachineAPINamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "machine.openshift.io/cluster-api-machineset=" + msName,
	})
	if err != nil {
		return fmt.Errorf("list machines for DHCP release: %w", err)
	}

	var nodes []string
	for _, m := range machines.Items {
		if m.Status.NodeRef != nil && m.DeletionTimestamp == nil {
			nodes = append(nodes, m.Status.NodeRef.Name)
		}
	}
	if len(nodes) == 0 {
		logf("DHCP release: no nodes with nodeRef found for MachineSet %s", msName)
		return nil
	}
	logf("DHCP release: sending release on %d nodes", len(nodes))

	var (
		mu       sync.Mutex
		released int
	)
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)

	for _, node := range nodes {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := releaseDHCPOnNode(ctx, kubeClient, n); err != nil {
				logf("DHCP release: node %s: %v", n, err)
			} else {
				mu.Lock()
				released++
				mu.Unlock()
			}
		}(node)
	}
	wg.Wait()

	logf("DHCP release: %d/%d nodes completed", released, len(nodes))
	if released == 0 {
		return fmt.Errorf("DHCP release failed on all %d nodes", len(nodes))
	}
	return nil
}

func releaseDHCPOnNode(ctx context.Context, client kubernetes.Interface, nodeName string) error {
	podName := dhcpReleasePodName(nodeName)
	ns := MachineAPINamespace

	_ = client.CoreV1().Pods(ns).Delete(ctx, podName, metav1.DeleteOptions{})
	waitForPodGone(ctx, client, ns, podName, 15*time.Second)

	privileged := true
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			NodeName:      nodeName,
			HostPID:       true,
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:  "release",
					Image: BusyboxImage,
					Command: []string{
						"nsenter", "-t", "1", "-m", "-u", "-n", "--",
						"nmcli", "connection", "down", "ovs-if-br-ex",
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: &privileged,
					},
				},
			},
		},
	}

	if _, err := client.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create pod: %w", err)
	}
	defer func() {
		_ = client.CoreV1().Pods(ns).Delete(ctx, podName, metav1.DeleteOptions{})
	}()

	return waitForPodNotPending(ctx, client, ns, podName, 60*time.Second)
}

func dhcpReleasePodName(nodeName string) string {
	name := "dhcp-rel-" + nodeName
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}

func waitForPodNotPending(ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		pod, err := client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		switch pod.Status.Phase {
		case corev1.PodPending:
			return false, nil
		case corev1.PodFailed:
			return false, fmt.Errorf("pod %s/%s failed", namespace, name)
		default:
			return true, nil
		}
	})
}

func waitForPodGone(ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) {
	_ = wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
}
