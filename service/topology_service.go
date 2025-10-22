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

func (s *DefaultTopologyService) resolveCustom(clusters []string, matrix []controllerv1alpha1.ConnectivityEntry) ([]GatewayPair, error) {
	if len(matrix) == 0 {
		return nil, fmt.Errorf("custom topology requires connectivity matrix")
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
	filtered := s.filterPairs(allPairs, forbidden)
	
	preservedPairs, err := s.ensureConnectivity(clusters, filtered, forbidden)
	if err != nil {
		return nil, err
	}

	return preservedPairs, nil
}

func (s *DefaultTopologyService) ensureConnectivity(clusters []string, pairs []GatewayPair, forbidden map[string]bool) ([]GatewayPair, error) {
	graph := s.buildGraph(pairs)
	components := s.findComponents(clusters, graph)

	if len(components) <= 1 {
		return pairs, nil
	}

	bridgeEdges := s.findBridges(clusters, components, forbidden)
	if len(bridgeEdges) == 0 {
		return nil, fmt.Errorf("policy nodes create partitioned topology with no safe bridge edges available")
	}

	for _, bridge := range bridgeEdges {
		pairs = append(pairs, bridge)
	}

	return pairs, nil
}

func (s *DefaultTopologyService) buildGraph(pairs []GatewayPair) map[string][]string {
	graph := make(map[string][]string)
	for _, p := range pairs {
		graph[p.Source] = append(graph[p.Source], p.Target)
		graph[p.Target] = append(graph[p.Target], p.Source)
	}
	return graph
}

func (s *DefaultTopologyService) findComponents(clusters []string, graph map[string][]string) [][]string {
	visited := make(map[string]bool)
	components := make([][]string, 0)

	for _, cluster := range clusters {
		if !visited[cluster] {
			component := s.dfs(cluster, graph, visited)
			components = append(components, component)
		}
	}

	return components
}

func (s *DefaultTopologyService) dfs(node string, graph map[string][]string, visited map[string]bool) []string {
	visited[node] = true
	component := []string{node}

	for _, neighbor := range graph[node] {
		if !visited[neighbor] {
			component = append(component, s.dfs(neighbor, graph, visited)...)
		}
	}

	return component
}

func (s *DefaultTopologyService) findBridges(clusters []string, components [][]string, forbidden map[string]bool) []GatewayPair {
	bridges := make([]GatewayPair, 0)
	componentMap := make(map[string]int)

	for i, comp := range components {
		for _, node := range comp {
			componentMap[node] = i
		}
	}

	for i := 0; i < len(components); i++ {
		for j := i + 1; j < len(components); j++ {
			added := false
			for _, ni := range components[i] {
				if added {
					break
				}
				for _, nj := range components[j] {
					key := s.pairKey(ni, nj)
					if !forbidden[key] {
						bridges = append(bridges, GatewayPair{
							Source:        ni,
							Target:        nj,
							Bidirectional: true,
						})
						added = true
						break
					}
				}
			}
		}
	}

	return bridges
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
