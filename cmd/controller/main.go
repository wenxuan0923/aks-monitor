package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"aks-health-monitor/pkg/config"
	"aks-health-monitor/pkg/controller"
	"aks-health-monitor/pkg/metrics"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func main() {
	var kubeconfig *string
	if home := os.Getenv("HOME"); home != "" {
		kubeconfig = flag.String("kubeconfig", home+"/.kube/config", "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	configPath := flag.String("config", "/etc/config/config.yaml", "path to configuration file (mounted from ConfigMap)")
	flag.Parse()

	klog.InitFlags(nil)
	defer klog.Flush()

	// Load configuration from ConfigMap-mounted file or environment variables
	cfg, err := config.LoadConfigFromConfigMap(*configPath)
	if err != nil {
		klog.Fatalf("Failed to load configuration: %v", err)
	}

	// Create Kubernetes client
	kubeClient, err := createKubernetesClient(*kubeconfig)
	if err != nil {
		klog.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Create metrics collector
	metricsCollector := metrics.NewCollector(kubeClient)

	// Create controller (ConfigMap mode only)
	healthController := controller.NewController(kubeClient, metricsCollector, cfg)

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-signalCh
		klog.Info("Received shutdown signal")
		cancel()
	}()

	// Start the controller
	klog.Info("Starting AKS Health Monitor Controller")
	if err := healthController.Run(ctx); err != nil {
		klog.Fatalf("Controller failed: %v", err)
	}

	klog.Info("Controller stopped")
}

func createKubernetesClient(kubeconfig string) (kubernetes.Interface, error) {
	var config *rest.Config
	var err error

	if kubeconfig == "" {
		// Use in-cluster config if running inside a pod
		config, err = rest.InClusterConfig()
	} else {
		// Use kubeconfig file
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}
