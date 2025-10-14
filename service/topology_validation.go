package service

import (
	"fmt"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
)

type TopologyValidator struct{}

func NewTopologyValidator() *TopologyValidator {
	return &TopologyValidator{}
}

func (v *TopologyValidator) ValidateTopologyConfig(topology *controllerv1alpha1.TopologyConfig, clusters []string) error {
	if topology == nil {
		return nil
	}

	clusterSet := toSet(clusters)

	switch topology.TopologyType {
	case controllerv1alpha1.TopologyHubSpoke:
		if err := v.validateHubSpoke(topology.HubSpoke, clusterSet); err != nil {
			return err
		}
	case controllerv1alpha1.TopologyCustom:
		if err := v.validateCustom(topology.ConnectivityMatrix, clusterSet); err != nil {
			return err
		}
	case controllerv1alpha1.TopologyAuto:
		if err := v.validateAuto(topology, clusterSet); err != nil {
			return err
		}
	case controllerv1alpha1.TopologyFullMesh, controllerv1alpha1.TopologyPartialMesh:
	case "":
	default:
		return fmt.Errorf("invalid topology type: %s", topology.TopologyType)
	}

	if err := v.validateClusterRoles(topology.ClusterRoles, clusterSet); err != nil {
		return err
	}

	return v.validatePolicyNodes(topology.PolicyNodes, clusterSet)
}

func (v *TopologyValidator) validateHubSpoke(config *controllerv1alpha1.HubSpokeConfig, clusterSet map[string]struct{}) error {
	if config == nil {
		return fmt.Errorf("hubSpoke config required for hub-spoke topology")
	}

	if len(config.HubClusters) == 0 {
		return fmt.Errorf("at least one hub cluster required")
	}

	for _, hub := range config.HubClusters {
		if _, exists := clusterSet[hub]; !exists {
			return fmt.Errorf("hub cluster %s not in spec.clusters", hub)
		}
	}

	hubSet := toSet(config.HubClusters)
	for _, spoke := range config.SpokeClusters {
		if _, exists := clusterSet[spoke]; !exists {
			return fmt.Errorf("spoke cluster %s not in spec.clusters", spoke)
		}
		if _, isHub := hubSet[spoke]; isHub {
			return fmt.Errorf("cluster %s cannot be both hub and spoke", spoke)
		}
	}

	if config.SpokeConnectivity != nil {
		spokeSet := toSet(config.SpokeClusters)
		for _, entry := range config.SpokeConnectivity {
			if _, exists := spokeSet[entry.SourceCluster]; !exists {
				return fmt.Errorf("spokeConnectivity source %s not a spoke cluster", entry.SourceCluster)
			}
			for _, target := range entry.TargetClusters {
				if _, exists := spokeSet[target]; !exists {
					return fmt.Errorf("spokeConnectivity target %s not a spoke cluster", target)
				}
			}
		}
	}

	return nil
}

func (v *TopologyValidator) validateCustom(matrix []controllerv1alpha1.ConnectivityEntry, clusterSet map[string]struct{}) error {
	if len(matrix) == 0 {
		return fmt.Errorf("connectivityMatrix required for custom topology")
	}

	for _, entry := range matrix {
		if _, exists := clusterSet[entry.SourceCluster]; !exists {
			return fmt.Errorf("connectivityMatrix source %s not in spec.clusters", entry.SourceCluster)
		}
		for _, target := range entry.TargetClusters {
			if _, exists := clusterSet[target]; !exists {
				return fmt.Errorf("connectivityMatrix target %s not in spec.clusters", target)
			}
		}
	}

	return nil
}

func (v *TopologyValidator) validateAuto(topology *controllerv1alpha1.TopologyConfig, clusterSet map[string]struct{}) error {
	if topology.AutoOptions != nil {
		if topology.AutoOptions.RelativeThresholdPercent < 1 || topology.AutoOptions.RelativeThresholdPercent > 500 {
			return fmt.Errorf("relativeThresholdPercent must be between 1 and 500 (represents 0.1%% to 50.0%%)")
		}
		if topology.AutoOptions.PersistenceWindows < 1 {
			return fmt.Errorf("persistenceWindows must be at least 1")
		}
		if topology.AutoOptions.MaxShortcuts < 1 || topology.AutoOptions.MaxShortcuts > 50 {
			return fmt.Errorf("maxShortcuts must be between 1 and 50")
		}
	}
	return nil
}

func (v *TopologyValidator) validateClusterRoles(roles []controllerv1alpha1.ClusterRole, clusterSet map[string]struct{}) error {
	for _, role := range roles {
		if _, exists := clusterSet[role.ClusterName]; !exists {
			return fmt.Errorf("clusterRole %s not in spec.clusters", role.ClusterName)
		}
	}
	return nil
}

func (v *TopologyValidator) validatePolicyNodes(policyNodes []string, clusterSet map[string]struct{}) error {
	for _, node := range policyNodes {
		if _, exists := clusterSet[node]; !exists {
			return fmt.Errorf("policyNode %s not in spec.clusters", node)
		}
	}
	return nil
}

func toSet(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		set[item] = struct{}{}
	}
	return set
}
