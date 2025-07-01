package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v2"
)

// Config represents the configuration for the AKS health monitor
type Config struct {
	// Polling interval for checking metrics
	PollInterval time.Duration `yaml:"pollInterval"`

	// Azure configuration
	Azure AzureConfig `yaml:"azure"`

	// Thresholds configuration
	Thresholds ThresholdsConfig `yaml:"thresholds"`

	// Operations to monitor
	MonitoredOperations []string `yaml:"monitoredOperations"`
}

// AzureConfig contains Azure-specific configuration
type AzureConfig struct {
	SubscriptionID    string `yaml:"subscriptionId"`
	ResourceGroupName string `yaml:"resourceGroupName"`
	ClusterName       string `yaml:"clusterName"`
	TenantID          string `yaml:"tenantId"`
	ClientID          string `yaml:"clientId"`
	ClientSecret      string `yaml:"clientSecret"`
}

// ThresholdsConfig defines the thresholds for various metrics
type ThresholdsConfig struct {
	CrashingPodsPercent  int `yaml:"crashingPodsPercent"`  // Percentage of total pods
	PendingPodsPercent   int `yaml:"pendingPodsPercent"`   // Percentage of total pods
	NotReadyNodesPercent int `yaml:"notReadyNodesPercent"` // Percentage of total nodes
	FailedJobs           int `yaml:"failedJobs"`           // Absolute number
	RestartCount         int `yaml:"restartCount"`         // Absolute number
	CpuUsagePercent      int `yaml:"cpuUsagePercent"`      // Percentage
	MemoryUsagePercent   int `yaml:"memoryUsagePercent"`   // Percentage
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(configPath string) (*Config, error) {
	// Set default configuration
	config := &Config{
		PollInterval: 30 * time.Second,
		Azure: AzureConfig{
			SubscriptionID:    os.Getenv("AZURE_SUBSCRIPTION_ID"),
			ResourceGroupName: os.Getenv("AZURE_RESOURCE_GROUP"),
			ClusterName:       os.Getenv("AZURE_CLUSTER_NAME"),
			TenantID:          os.Getenv("AZURE_TENANT_ID"),
			ClientID:          os.Getenv("AZURE_CLIENT_ID"),
			ClientSecret:      os.Getenv("AZURE_CLIENT_SECRET"),
		},
		Thresholds: ThresholdsConfig{
			CrashingPodsPercent:  10, // 10% of total pods
			PendingPodsPercent:   15, // 15% of total pods
			NotReadyNodesPercent: 25, // 25% of total nodes
			FailedJobs:           3,
			RestartCount:         20,
			CpuUsagePercent:      85,
			MemoryUsagePercent:   90,
		},
		MonitoredOperations: []string{"upgrade", "update", "scale"},
	}

	// If config file exists, load it
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// LoadConfigFromConfigMap loads configuration from a ConfigMap-mounted file and environment variables
// This function prioritizes environment variables over file configuration for cloud-native deployments
func LoadConfigFromConfigMap(configPath string) (*Config, error) {
	// Set default configuration with values from environment variables
	config := &Config{
		PollInterval: 30 * time.Second,
		Azure: AzureConfig{
			SubscriptionID:    getEnvOrDefault("AZURE_SUBSCRIPTION_ID", ""),
			ResourceGroupName: getEnvOrDefault("AZURE_RESOURCE_GROUP", ""),
			ClusterName:       getEnvOrDefault("AZURE_CLUSTER_NAME", ""),
			TenantID:          getEnvOrDefault("AZURE_TENANT_ID", ""),
			ClientID:          getEnvOrDefault("AZURE_CLIENT_ID", ""),
			ClientSecret:      getEnvOrDefault("AZURE_CLIENT_SECRET", ""),
		},
		Thresholds: ThresholdsConfig{
			CrashingPodsPercent:  parseIntEnvOrDefault("THRESHOLD_CRASHING_PODS_PERCENT", 10),
			PendingPodsPercent:   parseIntEnvOrDefault("THRESHOLD_PENDING_PODS_PERCENT", 15),
			NotReadyNodesPercent: parseIntEnvOrDefault("THRESHOLD_NOT_READY_NODES_PERCENT", 25),
			FailedJobs:           parseIntEnvOrDefault("THRESHOLD_FAILED_JOBS", 3),
			RestartCount:         parseIntEnvOrDefault("THRESHOLD_RESTART_COUNT", 20),
			CpuUsagePercent:      parseIntEnvOrDefault("THRESHOLD_CPU_USAGE_PERCENT", 85),
			MemoryUsagePercent:   parseIntEnvOrDefault("THRESHOLD_MEMORY_USAGE_PERCENT", 90),
		},
		MonitoredOperations: []string{"upgrade", "update", "scale"},
	}

	// Parse poll interval from environment variable if provided
	if pollIntervalStr := os.Getenv("POLL_INTERVAL"); pollIntervalStr != "" {
		if duration, err := time.ParseDuration(pollIntervalStr); err == nil {
			config.PollInterval = duration
		}
	}

	// If config file exists (from ConfigMap), overlay it on top of environment variables
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file from ConfigMap: %w", err)
		}

		// Create a temporary config to merge file values
		fileConfig := &Config{}
		if err := yaml.Unmarshal(data, fileConfig); err != nil {
			return nil, fmt.Errorf("failed to parse config file from ConfigMap: %w", err)
		}

		// Merge file config with environment-based config
		// Environment variables take precedence over file values for Azure credentials
		if fileConfig.PollInterval > 0 {
			config.PollInterval = fileConfig.PollInterval
		}

		// Only use file values for Azure config if environment variables are not set
		if config.Azure.SubscriptionID == "" && fileConfig.Azure.SubscriptionID != "" {
			config.Azure.SubscriptionID = fileConfig.Azure.SubscriptionID
		}
		if config.Azure.ResourceGroupName == "" && fileConfig.Azure.ResourceGroupName != "" {
			config.Azure.ResourceGroupName = fileConfig.Azure.ResourceGroupName
		}
		if config.Azure.ClusterName == "" && fileConfig.Azure.ClusterName != "" {
			config.Azure.ClusterName = fileConfig.Azure.ClusterName
		}
		if config.Azure.TenantID == "" && fileConfig.Azure.TenantID != "" {
			config.Azure.TenantID = fileConfig.Azure.TenantID
		}
		if config.Azure.ClientID == "" && fileConfig.Azure.ClientID != "" {
			config.Azure.ClientID = fileConfig.Azure.ClientID
		}
		if config.Azure.ClientSecret == "" && fileConfig.Azure.ClientSecret != "" {
			config.Azure.ClientSecret = fileConfig.Azure.ClientSecret
		}

		// Merge threshold values (file takes precedence for thresholds)
		if fileConfig.Thresholds.CrashingPodsPercent > 0 {
			config.Thresholds.CrashingPodsPercent = fileConfig.Thresholds.CrashingPodsPercent
		}
		if fileConfig.Thresholds.PendingPodsPercent > 0 {
			config.Thresholds.PendingPodsPercent = fileConfig.Thresholds.PendingPodsPercent
		}
		if fileConfig.Thresholds.NotReadyNodesPercent > 0 {
			config.Thresholds.NotReadyNodesPercent = fileConfig.Thresholds.NotReadyNodesPercent
		}
		if fileConfig.Thresholds.FailedJobs > 0 {
			config.Thresholds.FailedJobs = fileConfig.Thresholds.FailedJobs
		}
		if fileConfig.Thresholds.RestartCount > 0 {
			config.Thresholds.RestartCount = fileConfig.Thresholds.RestartCount
		}
		if fileConfig.Thresholds.CpuUsagePercent > 0 {
			config.Thresholds.CpuUsagePercent = fileConfig.Thresholds.CpuUsagePercent
		}
		if fileConfig.Thresholds.MemoryUsagePercent > 0 {
			config.Thresholds.MemoryUsagePercent = fileConfig.Thresholds.MemoryUsagePercent
		}

		// Use monitored operations from file if provided
		if len(fileConfig.MonitoredOperations) > 0 {
			config.MonitoredOperations = fileConfig.MonitoredOperations
		}
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Azure.SubscriptionID == "" {
		return fmt.Errorf("Azure subscription ID is required")
	}
	if c.Azure.ResourceGroupName == "" {
		return fmt.Errorf("Azure resource group name is required")
	}
	if c.Azure.ClusterName == "" {
		return fmt.Errorf("Azure cluster name is required")
	}
	if c.PollInterval < time.Second {
		return fmt.Errorf("poll interval must be at least 1 second")
	}

	// Validate percentage thresholds
	if c.Thresholds.CrashingPodsPercent < 0 || c.Thresholds.CrashingPodsPercent > 100 {
		return fmt.Errorf("crashingPodsPercent must be between 0 and 100, got: %d", c.Thresholds.CrashingPodsPercent)
	}
	if c.Thresholds.PendingPodsPercent < 0 || c.Thresholds.PendingPodsPercent > 100 {
		return fmt.Errorf("pendingPodsPercent must be between 0 and 100, got: %d", c.Thresholds.PendingPodsPercent)
	}
	if c.Thresholds.NotReadyNodesPercent < 0 || c.Thresholds.NotReadyNodesPercent > 100 {
		return fmt.Errorf("notReadyNodesPercent must be between 0 and 100, got: %d", c.Thresholds.NotReadyNodesPercent)
	}
	if c.Thresholds.CpuUsagePercent < 0 || c.Thresholds.CpuUsagePercent > 100 {
		return fmt.Errorf("cpuUsagePercent must be between 0 and 100, got: %d", c.Thresholds.CpuUsagePercent)
	}
	if c.Thresholds.MemoryUsagePercent < 0 || c.Thresholds.MemoryUsagePercent > 100 {
		return fmt.Errorf("memoryUsagePercent must be between 0 and 100, got: %d", c.Thresholds.MemoryUsagePercent)
	}

	return nil
}

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// parseIntEnvOrDefault parses an integer from an environment variable or returns a default value
func parseIntEnvOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
