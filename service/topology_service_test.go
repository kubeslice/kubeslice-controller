package service

import (
	"testing"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveTopologyDefaultsToFullMesh(t *testing.T) {
	svc := NewTopologyService()
	sc := &controllerv1alpha1.SliceConfig{
		Spec: controllerv1alpha1.SliceConfigSpec{
			Clusters: []string{"alpha", "beta", "gamma"},
		},
	}

	pairs, err := svc.ResolveTopology(sc)
	require.NoError(t, err)
	assert.Len(t, pairs, 3)
	assertContainsPair(t, pairs, "alpha", "beta")
	assertContainsPair(t, pairs, "alpha", "gamma")
	assertContainsPair(t, pairs, "beta", "gamma")
}

func TestResolveTopologyFullMeshExplicit(t *testing.T) {
	svc := NewTopologyService()
	sc := &controllerv1alpha1.SliceConfig{
		Spec: controllerv1alpha1.SliceConfigSpec{
			Clusters: []string{"c1", "c2", "c3", "c4"},
			TopologyConfig: &controllerv1alpha1.TopologyConfig{
				TopologyType: controllerv1alpha1.TopologyFullMesh,
			},
		},
	}

	pairs, err := svc.ResolveTopology(sc)
	require.NoError(t, err)
	assert.Len(t, pairs, 6)
	assertContainsPair(t, pairs, "c1", "c4")
	assertContainsPair(t, pairs, "c2", "c3")
}

func TestResolveTopologyCustomMatrix(t *testing.T) {
	svc := NewTopologyService()
	sc := &controllerv1alpha1.SliceConfig{
		Spec: controllerv1alpha1.SliceConfigSpec{
			Clusters: []string{"dmz", "gateway", "internal"},
			TopologyConfig: &controllerv1alpha1.TopologyConfig{
				TopologyType: controllerv1alpha1.TopologyCustom,
				ConnectivityMatrix: []controllerv1alpha1.ConnectivityEntry{
					{SourceCluster: "dmz", TargetClusters: []string{"gateway"}},
					{SourceCluster: "gateway", TargetClusters: []string{"internal"}},
				},
			},
		},
	}

	pairs, err := svc.ResolveTopology(sc)
	require.NoError(t, err)
	assert.Len(t, pairs, 2)
	assertContainsPair(t, pairs, "dmz", "gateway")
	assertContainsPair(t, pairs, "gateway", "internal")
}

func TestResolveTopologyAutoPolicyNodesReturnsError(t *testing.T) {
	svc := NewTopologyService()
	sc := &controllerv1alpha1.SliceConfig{
		Spec: controllerv1alpha1.SliceConfigSpec{
			Clusters: []string{"gateway", "dmz", "internal", "analytics"},
			TopologyConfig: &controllerv1alpha1.TopologyConfig{
				TopologyType: controllerv1alpha1.TopologyAuto,
				PolicyNodes:  []string{"dmz", "analytics"},
			},
		},
	}

	_, err := svc.ResolveTopology(sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "partitioned topology")
}

func TestEnsureConnectivityAddsBridgeEdge(t *testing.T) {
	svc := &DefaultTopologyService{}
	clusters := []string{"a", "b", "c"}
	pairs := []GatewayPair{{Source: "a", Target: "b", Bidirectional: true}}

	bridged, err := svc.ensureConnectivity(clusters, pairs, map[string]bool{})
	require.NoError(t, err)
	assert.Len(t, bridged, 2)
	// ensure new edge connects the previously isolated node
	assertContainsPair(t, bridged, "a", "b")
	assert.Contains(t, endpointPartners(bridged, "c"), "a")
}

func TestResolveTopologyAutoAllPolicyNodes(t *testing.T) {
	svc := &DefaultTopologyService{}
	clusters := []string{"one", "two"}
	policyNodes := []string{"one", "two"}

	_, err := svc.resolveAuto(clusters, policyNodes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "partitioned topology")
}

func endpointPartners(pairs []GatewayPair, node string) []string {
	partners := make([]string, 0)
	for _, p := range pairs {
		if p.Source == node {
			partners = append(partners, p.Target)
		} else if p.Target == node {
			partners = append(partners, p.Source)
		}
	}
	return partners
}

func assertContainsPair(t *testing.T, pairs []GatewayPair, source, target string) {
	t.Helper()
	for _, p := range pairs {
		if (p.Source == source && p.Target == target) || (p.Source == target && p.Target == source) {
			return
		}
	}
	t.Fatalf("expected to find pair %s-%s", source, target)
}

func assertNotContainsPair(t *testing.T, pairs []GatewayPair, source, target string) {
	t.Helper()
	for _, p := range pairs {
		if (p.Source == source && p.Target == target) || (p.Source == target && p.Target == source) {
			t.Fatalf("did not expect to find pair %s-%s", source, target)
		}
	}
}
