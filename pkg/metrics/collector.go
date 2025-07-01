package metrics

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// MetricType represents different types of metrics
type MetricType string

const (
	CrashingPodsPercentMetric  MetricType = "crashing_pods_percent"
	PendingPodsPercentMetric   MetricType = "pending_pods_percent"
	NotReadyNodesPercentMetric MetricType = "not_ready_nodes_percent"
	FailedJobsMetric           MetricType = "failed_jobs"
	RestartCountMetric         MetricType = "restart_count"
	CpuUsagePercentMetric      MetricType = "cpu_usage_percent"
	MemoryUsagePercentMetric   MetricType = "memory_usage_percent"
)

// MetricValue represents a metric with its value
type MetricValue struct {
	Type  MetricType
	Value int
}

// Collector collects various Kubernetes metrics
type Collector struct {
	kubeClient kubernetes.Interface
}

// NewCollector creates a new metrics collector
func NewCollector(kubeClient kubernetes.Interface) *Collector {
	return &Collector{
		kubeClient: kubeClient,
	}
}

// CollectMetrics collects all configured metrics
func (c *Collector) CollectMetrics(ctx context.Context) ([]MetricValue, error) {
	var metrics []MetricValue

	// Collect pod-related metrics
	podMetrics, err := c.collectPodMetrics(ctx)
	if err != nil {
		klog.Errorf("Failed to collect pod metrics: %v", err)
		return nil, err
	}
	metrics = append(metrics, podMetrics...)

	// Collect node-related metrics
	nodeMetrics, err := c.collectNodeMetrics(ctx)
	if err != nil {
		klog.Errorf("Failed to collect node metrics: %v", err)
		return nil, err
	}
	metrics = append(metrics, nodeMetrics...)

	// Collect job-related metrics
	jobMetrics, err := c.collectJobMetrics(ctx)
	if err != nil {
		klog.Errorf("Failed to collect job metrics: %v", err)
		return nil, err
	}
	metrics = append(metrics, jobMetrics...)

	return metrics, nil
}

// collectPodMetrics collects pod-related metrics
func (c *Collector) collectPodMetrics(ctx context.Context) ([]MetricValue, error) {
	pods, err := c.kubeClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var crashingPods, pendingPods, totalRestarts int
	totalPods := len(pods.Items)

	for _, pod := range pods.Items {
		// Count crashing pods (CrashLoopBackOff, Error, etc.)
		if c.isPodCrashing(pod) {
			crashingPods++
		}

		// Count pending pods
		if pod.Status.Phase == corev1.PodPending {
			pendingPods++
		}

		// Count restart counts
		for _, containerStatus := range pod.Status.ContainerStatuses {
			totalRestarts += int(containerStatus.RestartCount)
		}
	}

	// Calculate percentages (avoid division by zero)
	crashingPodsPercent := 0
	pendingPodsPercent := 0
	if totalPods > 0 {
		crashingPodsPercent = (crashingPods * 100) / totalPods
		pendingPodsPercent = (pendingPods * 100) / totalPods
	}

	return []MetricValue{
		{Type: CrashingPodsPercentMetric, Value: crashingPodsPercent},
		{Type: PendingPodsPercentMetric, Value: pendingPodsPercent},
		{Type: RestartCountMetric, Value: totalRestarts},
	}, nil
}

// collectNodeMetrics collects node-related metrics
func (c *Collector) collectNodeMetrics(ctx context.Context) ([]MetricValue, error) {
	nodes, err := c.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var notReadyNodes int
	totalNodes := len(nodes.Items)

	for _, node := range nodes.Items {
		if !c.isNodeReady(node) {
			notReadyNodes++
		}
	}

	// Calculate percentage (avoid division by zero)
	notReadyNodesPercent := 0
	if totalNodes > 0 {
		notReadyNodesPercent = (notReadyNodes * 100) / totalNodes
	}

	return []MetricValue{
		{Type: NotReadyNodesPercentMetric, Value: notReadyNodesPercent},
	}, nil
}

// collectJobMetrics collects job-related metrics
func (c *Collector) collectJobMetrics(ctx context.Context) ([]MetricValue, error) {
	jobs, err := c.kubeClient.BatchV1().Jobs("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	var failedJobs int

	for _, job := range jobs.Items {
		if job.Status.Failed > 0 {
			failedJobs++
		}
	}

	return []MetricValue{
		{Type: FailedJobsMetric, Value: failedJobs},
	}, nil
}

// isPodCrashing checks if a pod is in a crashing state
func (c *Collector) isPodCrashing(pod corev1.Pod) bool {
	// Check if pod is in Error or Failed phase
	if pod.Status.Phase == corev1.PodFailed {
		return true
	}

	// Check container statuses for crash loops
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Waiting != nil {
			reason := containerStatus.State.Waiting.Reason
			if reason == "CrashLoopBackOff" || reason == "ImagePullBackOff" ||
				reason == "ErrImagePull" || reason == "CreateContainerError" {
				return true
			}
		}

		if containerStatus.State.Terminated != nil {
			if containerStatus.State.Terminated.ExitCode != 0 {
				return true
			}
		}
	}

	return false
}

// isNodeReady checks if a node is ready
func (c *Collector) isNodeReady(node corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}
