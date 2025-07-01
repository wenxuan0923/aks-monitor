package controller

import (
	"context"
	"fmt"
	"time"

	"aks-health-monitor/pkg/azure"
	"aks-health-monitor/pkg/config"
	"aks-health-monitor/pkg/metrics"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// Controller monitors AKS deployment health and aborts operations if thresholds are exceeded
// This controller uses ConfigMap-based configuration
type Controller struct {
	kubeClient          kubernetes.Interface
	metricsCollector    *metrics.Collector
	azureClient         *azure.Client
	config              *config.Config
	operationInProgress bool
	currentOperation    string
}

// NewController creates a new health controller
func NewController(kubeClient kubernetes.Interface, metricsCollector *metrics.Collector, cfg *config.Config) *Controller {
	azureClient, err := azure.NewClient(cfg.Azure)
	if err != nil {
		klog.Fatalf("Failed to create Azure client: %v", err)
	}

	return &Controller{
		kubeClient:       kubeClient,
		metricsCollector: metricsCollector,
		azureClient:      azureClient,
		config:           cfg,
	}
}

// Run starts the health monitoring loop
func (c *Controller) Run(ctx context.Context) error {
	klog.Info("Starting health controller")

	ticker := time.NewTicker(c.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.Info("Stopping health controller")
			return nil
		case <-ticker.C:
			if err := c.checkHealth(ctx); err != nil {
				klog.Errorf("Health check failed: %v", err)
			}
		}
	}
}

// checkHealth performs a single health check cycle
func (c *Controller) checkHealth(ctx context.Context) error {
	// Check if there's an ongoing operation
	operationStatus, err := c.azureClient.GetClusterOperationStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get cluster operation status: %w", err)
	}

	c.operationInProgress = operationStatus.InProgress
	c.currentOperation = operationStatus.OperationType

	if !c.operationInProgress {
		klog.V(2).Info("No operation in progress, skipping health check")
		return nil
	}

	klog.Infof("Operation '%s' in progress, checking health metrics", c.currentOperation)

	// Collect metrics
	collectedMetrics, err := c.metricsCollector.CollectMetrics(ctx)
	if err != nil {
		return fmt.Errorf("failed to collect metrics: %w", err)
	}

	// Evaluate thresholds
	violations := c.evaluateThresholds(collectedMetrics)
	if len(violations) > 0 {
		klog.Warningf("Threshold violations detected: %v", violations)

		// Abort the operation
		if err := c.abortOperation(ctx); err != nil {
			return fmt.Errorf("failed to abort operation: %w", err)
		}

		klog.Infof("Successfully aborted operation '%s' due to threshold violations", c.currentOperation)
	} else {
		klog.V(2).Info("All metrics within acceptable thresholds")
	}

	return nil
}

// evaluateThresholds checks if any metrics exceed their configured thresholds
func (c *Controller) evaluateThresholds(collectedMetrics []metrics.MetricValue) []string {
	var violations []string

	for _, metric := range collectedMetrics {
		threshold := c.getThresholdForMetric(metric.Type)
		if metric.Value > threshold {
			violation := fmt.Sprintf("%s: %d > %d", metric.Type, metric.Value, threshold)
			violations = append(violations, violation)
			klog.Warningf("Threshold violation: %s", violation)
		} else {
			klog.V(2).Infof("Metric %s: %d <= %d (OK)", metric.Type, metric.Value, threshold)
		}
	}

	return violations
}

// getThresholdForMetric returns the configured threshold for a specific metric type
func (c *Controller) getThresholdForMetric(metricType metrics.MetricType) int {
	switch metricType {
	case metrics.CrashingPodsPercentMetric:
		return c.config.Thresholds.CrashingPodsPercent
	case metrics.PendingPodsPercentMetric:
		return c.config.Thresholds.PendingPodsPercent
	case metrics.NotReadyNodesPercentMetric:
		return c.config.Thresholds.NotReadyNodesPercent
	case metrics.FailedJobsMetric:
		return c.config.Thresholds.FailedJobs
	case metrics.RestartCountMetric:
		return c.config.Thresholds.RestartCount
	case metrics.CpuUsagePercentMetric:
		return c.config.Thresholds.CpuUsagePercent
	case metrics.MemoryUsagePercentMetric:
		return c.config.Thresholds.MemoryUsagePercent
	default:
		klog.Warningf("Unknown metric type: %s, using default threshold of 0", metricType)
		return 0
	}
}

// abortOperation aborts the current AKS operation
func (c *Controller) abortOperation(ctx context.Context) error {
	klog.Warningf("Aborting operation '%s' due to health check failures", c.currentOperation)

	return c.azureClient.AbortClusterOperation(ctx, c.currentOperation)
}

// GetStatus returns the current status of the controller
func (c *Controller) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"operationInProgress": c.operationInProgress,
		"currentOperation":    c.currentOperation,
		"pollInterval":        c.config.PollInterval,
		"thresholds":          c.config.Thresholds,
	}
}
