package framework

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
)

// ReleaseDHCPLeases runs `nmcli connection down ovs-if-br-ex` on each node in
// the given MachineSet via `oc debug node/` to send DHCP RELEASE before machine
// deletion. This prevents IP address exhaustion when running back-to-back
// benchmarks on a constrained DHCP pool.
//
// The node loses network after nmcli runs, so oc debug will typically be
// killed by timeout — this is expected and counted as success.
//
// Best-effort: individual node failures are logged via logf. Returns an error
// only if zero nodes could be processed.
func ReleaseDHCPLeases(ctx context.Context, machineClient machineclient.Interface, msName string, logf func(string, ...any)) error {
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
	sem := make(chan struct{}, 20)

	for _, node := range nodes {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := releaseDHCPOnNode(ctx, n); err != nil {
				logf("DHCP release: node %s: %v", n, err)
			} else {
				logf("DHCP release: node %s: done", n)
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

func releaseDHCPOnNode(ctx context.Context, nodeName string) error {
	// The command takes <10s. After nmcli brings down the interface oc debug
	// hangs waiting for kubelet — the timeout kills it, which is fine.
	nodeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(nodeCtx, "oc", "debug", "node/"+nodeName, "--",
		"chroot", "/host", "nmcli", "connection", "down", "ovs-if-br-ex")
	cmd.Stderr = &stderr

	err := cmd.Run()
	if nodeCtx.Err() != nil {
		return nil
	}
	if err != nil && bytes.Contains(stderr.Bytes(), []byte("i/o timeout")) {
		// nmcli ran and brought down the interface; oc debug failed
		// streaming logs from the now-offline kubelet. That's success.
		return nil
	}
	if err != nil {
		return fmt.Errorf("%v: %s", err, stderr.String())
	}
	return nil
}
