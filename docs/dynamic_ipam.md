# Dynamic IPAM for KubeSlice

## Overview

The Dynamic IPAM (IP Address Management) system for KubeSlice provides efficient, on-demand IP address allocation for slice overlay networks. Unlike the static IPAM approach that pre-allocates fixed subnets, Dynamic IPAM allocates IP subnets to clusters on demand and reclaims unused ranges when clusters leave the slice.

## Features

- **On-demand Allocation**: IP subnets are allocated only when clusters join a slice
- **Automatic Reclamation**: Unused subnets are automatically reclaimed when clusters leave
- **Efficient Space Utilization**: Eliminates IP space wastage by allocating only what's needed
- **Backward Compatibility**: Falls back to static IPAM for existing configurations
- **Configurable Strategies**: Support for Sequential, Random, and Efficient allocation strategies
- **Reserved Subnets**: Ability to reserve specific subnets for special purposes

## Architecture

### Components

1. **IPAMPool**: Custom resource that manages IP address pools for slices
2. **IPAMService**: Service layer that handles allocation and deallocation logic
3. **IPAMPoolReconciler**: Controller that manages IPAM pool lifecycle
4. **Integration Layer**: Seamless integration with existing slice configuration

### How It Works

1. When a slice is created with `sliceIpamType: "Dynamic"`, an IPAM pool is automatically created
2. The pool divides the slice subnet into smaller cluster subnets (e.g., /24 subnets from a /16 slice)
3. As clusters join the slice, subnets are allocated from the available pool
4. When clusters leave, subnets are marked for reclamation and eventually returned to the pool

## Usage

### Enabling Dynamic IPAM

To enable Dynamic IPAM for a slice, set the `sliceIpamType` field to `"Dynamic"`:

```yaml
apiVersion: controller.kubeslice.io/v1alpha1
kind: SliceConfig
metadata:
  name: example-slice
  namespace: default
spec:
  sliceName: "example-slice"
  sliceSubnet: "10.1.0.0/16"
  sliceIpamType: "Dynamic"  # Enable dynamic IPAM
  maxClusters: 8
  clusters:
    - "cluster-1"
    - "cluster-2"
```

### IPAM Pool Configuration

The system automatically creates an IPAM pool for each slice with Dynamic IPAM enabled. You can also create custom IPAM pools:

```yaml
apiVersion: controller.kubeslice.io/v1alpha1
kind: IPAMPool
metadata:
  name: custom-ipam-pool
  namespace: default
spec:
  sliceSubnet: "10.2.0.0/16"
  poolType: "Dynamic"
  subnetSize: "/24"
  maxClusters: 16
  allocationStrategy: "Efficient"
  autoReclaim: true
  reclaimDelay: 3600
  reservedSubnets:
    - "10.2.0.0/24"  # Reserved for special purposes
    - "10.2.255.0/24" # Reserved for gateway networks
```

### Configuration Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `poolType` | string | "Dynamic" | Type of IPAM pool (Dynamic/Static) |
| `subnetSize` | string | Required | Size of individual cluster subnets (e.g., "/24") |
| `maxClusters` | int | 16 | Maximum number of clusters that can join |
| `allocationStrategy` | string | "Efficient" | Allocation strategy (Sequential/Random/Efficient) |
| `autoReclaim` | bool | true | Enable automatic reclamation of unused subnets |
| `reclaimDelay` | int | 3600 | Delay before reclaiming unused subnets (seconds) |
| `reservedSubnets` | []string | [] | List of subnets that cannot be allocated |

## Allocation Strategies

### Sequential
Allocates subnets in order from the available pool. Simple and predictable.

### Random
Randomly selects subnets from the available pool. Provides load distribution.

### Efficient
Selects subnets to maintain network locality and optimize routing. Recommended for production use.

## Monitoring

### Metrics

The system provides several metrics for monitoring IPAM pool status:

- `kubeslice_ipam_pool_total_subnets`: Total number of subnets in the pool
- `kubeslice_ipam_pool_allocated_subnets`: Number of allocated subnets
- `kubeslice_ipam_pool_available_subnets`: Number of available subnets

### Events

The system generates events for important operations:

- `IPAMPoolReconciled`: Pool reconciliation completed successfully
- `IPAMPoolReconciliationFailed`: Pool reconciliation failed

## Migration from Static IPAM

Existing slices using static IPAM will continue to work unchanged. To migrate to Dynamic IPAM:

1. Update the slice configuration to set `sliceIpamType: "Dynamic"`
2. The system will automatically create an IPAM pool
3. New clusters joining the slice will use dynamic allocation
4. Existing allocations remain unchanged

## Troubleshooting

### Common Issues

1. **No Available Subnets**: Check if the pool has enough subnets for the requested allocation
2. **Allocation Failures**: Verify that the slice subnet and subnet size are compatible
3. **Reclamation Issues**: Check if auto-reclaim is enabled and the reclaim delay is appropriate

### Debugging

Enable debug logging to see detailed IPAM operations:

```bash
kubectl logs -n kubeslice-system deployment/kubeslice-controller --v=4
```

## Best Practices

1. **Subnet Sizing**: Choose appropriate subnet sizes based on cluster requirements
2. **Reserved Subnets**: Reserve subnets for special purposes (gateways, management, etc.)
3. **Monitoring**: Set up alerts for pool exhaustion and allocation failures
4. **Documentation**: Document your IPAM strategy and reserved subnets

## Examples

### Basic Dynamic IPAM Slice

```yaml
apiVersion: controller.kubeslice.io/v1alpha1
kind: SliceConfig
metadata:
  name: basic-dynamic-slice
  namespace: default
spec:
  sliceName: "basic-dynamic-slice"
  sliceSubnet: "10.1.0.0/16"
  sliceIpamType: "Dynamic"
  maxClusters: 4
  clusters:
    - "cluster-1"
    - "cluster-2"
```

### Advanced IPAM Pool

```yaml
apiVersion: controller.kubeslice.io/v1alpha1
kind: IPAMPool
metadata:
  name: advanced-ipam-pool
  namespace: production
spec:
  sliceSubnet: "172.16.0.0/12"
  poolType: "Dynamic"
  subnetSize: "/26"
  maxClusters: 32
  allocationStrategy: "Efficient"
  autoReclaim: true
  reclaimDelay: 7200
  reservedSubnets:
    - "172.16.0.0/26"    # Management network
    - "172.16.255.192/26" # Gateway network
    - "172.31.255.192/26" # Backup network
```

## Support

For issues and questions related to Dynamic IPAM:

1. Check the logs for error messages
2. Review the IPAM pool status
3. Verify slice configuration
4. Open an issue in the KubeSlice repository

## Future Enhancements

- **IPv6 Support**: Full IPv6 address management
- **Advanced Strategies**: Machine learning-based allocation optimization
- **Multi-region Support**: Cross-region IPAM coordination
- **API Extensions**: REST API for external IPAM integration
