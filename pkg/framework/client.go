package framework

import (
	"fmt"
	"os"
	"path/filepath"

	configclient "github.com/openshift/client-go/config/clientset/versioned"
	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	operatorclient "github.com/openshift/client-go/operator/clientset/versioned"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Clients holds Kubernetes and OpenShift clients used by e2e tests.
type Clients struct {
	Kube      kubernetes.Interface
	Config    configclient.Interface
	Machine   machineclient.Interface
	Operator  operatorclient.Interface
	Dynamic   dynamic.Interface
	Discovery discovery.DiscoveryInterface
	Rest      *rest.Config
}

// NewClients builds clients from KUBECONFIG or in-cluster config.
func NewClients() (*Clients, error) {
	cfg, err := loadRestConfig()
	if err != nil {
		return nil, err
	}
	cfg.QPS = 50
	cfg.Burst = 100

	kube, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kubernetes client: %w", err)
	}

	config, err := configclient.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("config client: %w", err)
	}

	machine, err := machineclient.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("machine client: %w", err)
	}

	operator, err := operatorclient.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("operator client: %w", err)
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}

	disc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("discovery client: %w", err)
	}

	return &Clients{
		Kube:      kube,
		Config:    config,
		Machine:   machine,
		Operator:  operator,
		Dynamic:   dyn,
		Discovery: disc,
		Rest:      cfg,
	}, nil
}

func loadRestConfig() (*rest.Config, error) {
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	if home, err := os.UserHomeDir(); err == nil {
		defaultPath := filepath.Join(home, ".kube", "config")
		if _, err := os.Stat(defaultPath); err == nil {
			return clientcmd.BuildConfigFromFlags("", defaultPath)
		}
	}

	return rest.InClusterConfig()
}
