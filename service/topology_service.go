package service

import (
	"fmt"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
)

type GatewayPair struct {
	Source        string
	Target        string
	Bidirectional bool
}

type TopologyService interface {
	ResolveTopology(sliceConfig *controllerv1alpha1.SliceConfig) ([]GatewayPair, error)
}

type DefaultTopologyService struct{}

func NewTopologyService() TopologyService {
	return &DefaultTopologyService{}
}

func (s *DefaultTopologyService) ResolveTopology(sc *controllerv1alpha1.SliceConfig) ([]GatewayPair, error) {
	if sc.Spec.TopologyConfig == nil {
		return s.resolveFullMesh(sc.Spec.Clusters)
	}

	switch sc.Spec.TopologyConfig.TopologyType {
	case controllerv1alpha1.TopologyFullMesh, "":
		return s.resolveFullMesh(sc.Spec.Clusters)
	case controllerv1alpha1.TopologyHubSpoke:
		return s.resolveHubSpoke(sc.Spec.Clusters, sc.Spec.TopologyConfig.HubSpoke)
	case controllerv1alpha1.TopologyCustom:
		return s.resolveCustom(sc.Spec.Clusters, sc.Spec.TopologyConfig.ConnectivityMatrix)
	case controllerv1alpha1.TopologyAuto:
		return s.resolveAuto(sc.Spec.Clusters, sc.Spec.TopologyConfig.PolicyNodes)
	default:
		return nil, fmt.Errorf("unknown topology type: %s", sc.Spec.TopologyConfig.TopologyType)
	}
}

func (s *DefaultTopologyService) resolveFullMesh(clusters []string) ([]GatewayPair, error) {
	if len(clusters) < 2 {
		return []GatewayPair{}, nil
	}

	pairs := make([]GatewayPair, 0, len(clusters)*(len(clusters)-1)/2)
	for i := 0; i < len(clusters); i++ {
		for j := i + 1; j < len(clusters); j++ {
			pairs = append(pairs, GatewayPair{
				Source:        clusters[i],
				Target:        clusters[j],
				Bidirectional: true,
			})
		}
	}
	return pairs, nil
}

func (s *DefaultTopologyService) resolveHubSpoke(clusters []string, cfg *controllerv1alpha1.HubSpokeConfig) ([]GatewayPair, error) {
	if cfg == nil {
		return nil, fmt.Errorf("hub-spoke config required")
	}

	hubs := cfg.HubClusters
	spokes := cfg.SpokeClusters
	if len(spokes) == 0 {
		spokes = s.getSpokes(clusters, hubs)
	}

	if len(hubs) == 0 {
		return nil, fmt.Errorf("at least one hub required")
	}

	pairs := make([]GatewayPair, 0, len(hubs)*len(spokes))
	for _, hub := range hubs {
		for _, spoke := range spokes {
			pairs = append(pairs, GatewayPair{
				Source:        hub,
				Target:        spoke,
				Bidirectional: true,
			})
		}
	}

	if cfg.AllowSpokeToSpoke {
		pairs = append(pairs, s.resolveSpokePairs(spokes, cfg.SpokeConnectivity)...)
	}

	return pairs, nil
}

func (s *DefaultTopologyService) getSpokes(clusters []string, hubs []string) []string {
	hubSet := s.toSet(hubs)
	spokes := make([]string, 0, len(clusters))
	for _, cluster := range clusters {
		if !hubSet[cluster] {
			spokes = append(spokes, cluster)
		}
	}
	return spokes
}

func (s *DefaultTopologyService) resolveSpokePairs(spokes []string, connectivity []controllerv1alpha1.ConnectivityEntry) []GatewayPair {
	if len(connectivity) > 0 {
		return s.resolveSelectiveSpokes(connectivity)
	}

	pairs := make([]GatewayPair, 0, len(spokes)*(len(spokes)-1)/2)
	for i := 0; i < len(spokes); i++ {
		for j := i + 1; j < len(spokes); j++ {
			pairs = append(pairs, GatewayPair{
				Source:        spokes[i],
				Target:        spokes[j],
				Bidirectional: true,
			})
		}
	}
	return pairs
}

func (s *DefaultTopologyService) resolveSelectiveSpokes(entries []controllerv1alpha1.ConnectivityEntry) []GatewayPair {
	pairs := make([]GatewayPair, 0)
	for _, entry := range entries {
		for _, target := range entry.TargetClusters {
			pairs = append(pairs, GatewayPair{
				Source:        entry.SourceCluster,
				Target:        target,
				Bidirectional: true,
			})
		}
	}
	return pairs
}

func (s *DefaultTopologyService) resolveCustom(clusters []string, matrix []controllerv1alpha1.ConnectivityEntry) ([]GatewayPair, error) {
	if len(matrix) == 0 {
		return nil, fmt.Errorf("custom config with connectivity matrix required")
	}

	clusterSet := s.toSet(clusters)
	pairs := make([]GatewayPair, 0)

	for _, entry := range matrix {
		if !clusterSet[entry.SourceCluster] {
			return nil, fmt.Errorf("connectivity entry references unknown source cluster: %s", entry.SourceCluster)
		}
		for _, target := range entry.TargetClusters {
			if !clusterSet[target] {
				return nil, fmt.Errorf("connectivity entry references unknown target cluster: %s", target)
			}
			pairs = append(pairs, GatewayPair{
				Source:        entry.SourceCluster,
				Target:        target,
				Bidirectional: true,
			})
		}
	}

	return pairs, nil
}

func (s *DefaultTopologyService) resolveAuto(clusters []string, policyNodes []string) ([]GatewayPair, error) {
	allPairs, _ := s.resolveFullMesh(clusters)

	if len(policyNodes) == 0 {
		return allPairs, nil
	}

	forbidden := s.buildForbiddenSet(clusters, policyNodes)
	return s.filterPairs(allPairs, forbidden), nil
}

func (s *DefaultTopologyService) buildForbiddenSet(clusters []string, policyNodes []string) map[string]bool {
	forbidden := make(map[string]bool)
	for _, node := range policyNodes {
		for _, cluster := range clusters {
			if cluster != node {
				forbidden[s.pairKey(node, cluster)] = true
			}
		}
	}
	return forbidden
}

func (s *DefaultTopologyService) filterPairs(pairs []GatewayPair, forbidden map[string]bool) []GatewayPair {
	filtered := make([]GatewayPair, 0, len(pairs))
	for _, p := range pairs {
		if !forbidden[s.pairKey(p.Source, p.Target)] {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func (s *DefaultTopologyService) toSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

func (s *DefaultTopologyService) pairKey(a, b string) string {
	if a < b {
		return a + "-" + b
	}
	return b + "-" + a
}
