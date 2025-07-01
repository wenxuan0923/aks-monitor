# AKS Health Monitor

A Kubernetes controller that monitors Azure Kubernetes Service (AKS) cluster health and can abort potentially dangerous operations when health thresholds are exceeded.

## Overview

The AKS Health Monitor continuously monitors your AKS cluster's health by collecting metrics from the Kubernetes API and Azure Resource Manager API. It tracks various health indicators such as pod crash rates, node readiness, failed jobs, and container restart counts. When health metrics exceed configured thresholds during critical operations, the monitor can abort those operations to prevent further cluster degradation.

## Features

- **Real-time Health Monitoring**: Continuously monitors cluster health metrics
- **Configurable Thresholds**: Set custom thresholds for various health indicators
- **Operation Awareness**: Integrates with Azure operations to prevent dangerous changes during unhealthy states
- **Kubernetes Native**: Runs as a Kubernetes controller with proper RBAC
- **Dual Configuration**: Supports both ConfigMap and CRD-based configuration
- **Metrics Collection**: Tracks pods, nodes, jobs, and container health

## Architecture

The monitor consists of several key components:

- **Controller**: Main control loop that orchestrates health checking
- **Metrics Collector**: Gathers health metrics from Kubernetes API
- **Azure Client**: Interfaces with Azure Resource Manager for AKS operations
- **Configuration Manager**: Handles both ConfigMap and CRD-based configuration

## Health Metrics

The following metrics are monitored:

| Metric | Description | Default Threshold |
|--------|-------------|-------------------|
| Crashing Pods | Percentage of pods in CrashLoopBackOff state | 10% |
| Pending Pods | Percentage of pods stuck in Pending state | 15% |
| Not Ready Nodes | Percentage of nodes not in Ready state | 25% |
| Failed Jobs | Number of failed jobs in the cluster | 3 |
| Container Restarts | Total restart count across all containers | 20 |

## Installation

### Prerequisites

- AKS cluster running Kubernetes 1.20+
- Azure service principal or managed identity with AKS permissions
- kubectl configured to access your cluster

### Quick Install

1. **Create Azure credentials secret:**
```bash
kubectl create secret generic azure-credentials \
  --from-literal=subscription-id="your-subscription-id" \
  --from-literal=resource-group="your-resource-group" \
  --from-literal=cluster-name="your-aks-cluster" \
  --from-literal=tenant-id="your-tenant-id" \
  --from-literal=client-id="your-client-id" \
  --from-literal=client-secret="your-client-secret" \
  -n kube-system
```

2. **Deploy the monitor:**
```bash
kubectl apply -f deploy/kubernetes.yaml
```

### Configuration

#### ConfigMap Configuration

Create a ConfigMap with your monitoring configuration:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: aks-health-monitor-config
  namespace: kube-system
data:
  config.yaml: |
    pollInterval: 30s
    azure:
      subscriptionId: "your-subscription-id"
      resourceGroupName: "your-resource-group"
      clusterName: "your-aks-cluster"
    thresholds:
      crashingPodsPercent: 10
      pendingPodsPercent: 15
      notReadyNodesPercent: 25
      failedJobs: 3
      restartCount: 20
```

#### CRD Configuration

For more advanced configurations, use Custom Resource Definitions:

```yaml
apiVersion: monitor.aks.io/v1
kind: AKSHealthMonitor
metadata:
  name: cluster-monitor
spec:
  pollInterval: 30s
  thresholds:
    crashingPodsPercent: 10
    pendingPodsPercent: 15
  operationPolicies:
  - name: upgrade-policy
    operations: ["upgrade"]
    strictMode: true
```

## Usage

### Monitoring Health

Once deployed, the monitor will:

1. Poll cluster metrics every 30 seconds (configurable)
2. Compare metrics against configured thresholds
3. Log health status and violations
4. Block or abort operations when thresholds are exceeded

### Viewing Logs

```bash
kubectl logs -f deployment/aks-health-monitor -n kube-system
```

### Checking Status

```bash
kubectl get pods -n kube-system -l app=aks-health-monitor
```

## Development

### Building from Source

```bash
# Clone the repository
git clone https://github.com/your-org/aks-health-monitor.git
cd aks-health-monitor

# Build the binary
go build -o bin/aks-health-monitor cmd/controller/main.go

# Build Docker image
docker build -t aks-health-monitor:latest .
```

### Running Locally

```bash
# Set up configuration
export KUBECONFIG=~/.kube/config

# Run the controller
./bin/aks-health-monitor -config=config/config.yaml
```

### Testing

```bash
go test ./...
```

## Configuration Reference

### Main Configuration

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `pollInterval` | duration | How often to check metrics | 30s |
| `azure.subscriptionId` | string | Azure subscription ID | - |
| `azure.resourceGroupName` | string | Resource group name | - |
| `azure.clusterName` | string | AKS cluster name | - |
| `azure.tenantId` | string | Azure tenant ID | - |
| `azure.clientId` | string | Service principal client ID | - |
| `azure.clientSecret` | string | Service principal client secret | - |

### Threshold Configuration

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `thresholds.crashingPodsPercent` | int | Max % of crashing pods | 10 |
| `thresholds.pendingPodsPercent` | int | Max % of pending pods | 15 |
| `thresholds.notReadyNodesPercent` | int | Max % of not-ready nodes | 25 |
| `thresholds.failedJobs` | int | Max number of failed jobs | 3 |
| `thresholds.restartCount` | int | Max total container restarts | 20 |

## Security

### RBAC Permissions

The monitor requires the following Kubernetes permissions:

- `pods`: list, watch, get
- `nodes`: list, watch, get  
- `jobs`: list, watch, get
- `configmaps`: get, list, watch
- `events`: create, patch

### Azure Permissions

The Azure service principal needs:

- `Microsoft.ContainerService/managedClusters/read`
- `Microsoft.ContainerService/managedClusters/listClusterUserCredential/action`

## Troubleshooting

### Common Issues

1. **Monitor not starting**: Check Azure credentials and RBAC permissions
2. **High false positives**: Adjust thresholds in configuration
3. **Missing metrics**: Verify cluster access and API connectivity

### Debug Mode

Enable debug logging:

```bash
kubectl patch deployment aks-health-monitor -n kube-system -p '{"spec":{"template":{"spec":{"containers":[{"name":"aks-health-monitor","args":["--v=2"]}]}}}}'
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

For support and questions:

- Create an issue in the GitHub repository
- Check the documentation in the `docs/` directory
- Review troubleshooting guide above
