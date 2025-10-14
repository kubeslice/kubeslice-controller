package service

import (
	"testing"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveTopology_Legacy(t *testing.T) {
	svc := NewTopologyService()
	sc := &controllerv1alpha1.SliceConfig{
		Spec: controllerv1alpha1.SliceConfigSpec{
			Clusters: []string{"cluster-1", "cluster-2", "cluster-3"},
		},
	}

	pairs, err := svc.ResolveTopology(sc)
	require.NoError(t, err)
	assert.Len(t, pairs, 3)
	assertContainsPair(t, pairs, "cluster-1", "cluster-2", true)
	assertContainsPair(t, pairs, "cluster-1", "cluster-3", true)
	assertContainsPair(t, pairs, "cluster-2", "cluster-3", true)
}

func TestResolveFullMesh(t *testing.T) {
	tests := []struct {
		name     string
		clusters []string
		expected int
	}{
		{"2 clusters", []string{"c1", "c2"}, 1},
		{"3 clusters", []string{"c1", "c2", "c3"}, 3},
		{"4 clusters", []string{"c1", "c2", "c3", "c4"}, 6},
		{"1 cluster", []string{"c1"}, 0},
		{"0 clusters", []string{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &DefaultTopologyService{}
			pairs, err := svc.resolveFullMesh(tt.clusters)
			require.NoError(t, err)
			assert.Len(t, pairs, tt.expected)
			for _, p := range pairs {
				assert.True(t, p.Bidirectional)
			}
		})
	}
}

func TestResolveHubSpoke_SingleHub(t *testing.T) {
	svc := &DefaultTopologyService{}
	clusters := []string{"hub1", "spoke1", "spoke2", "spoke3"}
	cfg := &controllerv1alpha1.HubSpokeConfig{
		HubClusters: []string{"hub1"},
	}

	pairs, err := svc.resolveHubSpoke(clusters, cfg)
	require.NoError(t, err)
	assert.Len(t, pairs, 3)
	assertContainsPair(t, pairs, "hub1", "spoke1", true)
	assertContainsPair(t, pairs, "hub1", "spoke2", true)
	assertContainsPair(t, pairs, "hub1", "spoke3", true)
}

func TestResolveHubSpoke_MultipleHubs(t *testing.T) {
	svc := &DefaultTopologyService{}
	clusters := []string{"hub1", "hub2", "spoke1", "spoke2"}
	cfg := &controllerv1alpha1.HubSpokeConfig{
		HubClusters: []string{"hub1", "hub2"},
	}

	pairs, err := svc.resolveHubSpoke(clusters, cfg)
	require.NoError(t, err)
	assert.Len(t, pairs, 4)
	assertContainsPair(t, pairs, "hub1", "spoke1", true)
	assertContainsPair(t, pairs, "hub1", "spoke2", true)
	assertContainsPair(t, pairs, "hub2", "spoke1", true)
	assertContainsPair(t, pairs, "hub2", "spoke2", true)
}

func TestResolveHubSpoke_AllowSpokeToSpokeAll(t *testing.T) {
	svc := &DefaultTopologyService{}
	clusters := []string{"hub1", "spoke1", "spoke2", "spoke3"}
	cfg := &controllerv1alpha1.HubSpokeConfig{
		HubClusters:       []string{"hub1"},
		AllowSpokeToSpoke: true,
	}

	pairs, err := svc.resolveHubSpoke(clusters, cfg)
	require.NoError(t, err)
	assert.Len(t, pairs, 6)
	assertContainsPair(t, pairs, "spoke1", "spoke2", true)
	assertContainsPair(t, pairs, "spoke1", "spoke3", true)
	assertContainsPair(t, pairs, "spoke2", "spoke3", true)
}

func TestResolveHubSpoke_SelectiveSpokeConnectivity(t *testing.T) {
	svc := &DefaultTopologyService{}
	clusters := []string{"hub1", "spoke1", "spoke2", "spoke3"}
	cfg := &controllerv1alpha1.HubSpokeConfig{
		HubClusters:       []string{"hub1"},
		AllowSpokeToSpoke: true,
		SpokeConnectivity: []controllerv1alpha1.ConnectivityEntry{
			{SourceCluster: "spoke1", TargetClusters: []string{"spoke2"}},
		},
	}

	pairs, err := svc.resolveHubSpoke(clusters, cfg)
	require.NoError(t, err)
	assert.Len(t, pairs, 4)
	assertContainsPair(t, pairs, "hub1", "spoke1", true)
	assertContainsPair(t, pairs, "hub1", "spoke2", true)
	assertContainsPair(t, pairs, "hub1", "spoke3", true)
	assertContainsPair(t, pairs, "spoke1", "spoke2", true)
}

func TestResolveHubSpoke_NoHubError(t *testing.T) {
	svc := &DefaultTopologyService{}
	clusters := []string{"spoke1", "spoke2"}
	cfg := &controllerv1alpha1.HubSpokeConfig{
		HubClusters: []string{},
	}

	_, err := svc.resolveHubSpoke(clusters, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one hub required")
}

func TestResolveCustom(t *testing.T) {
	svc := &DefaultTopologyService{}
	clusters := []string{"c1", "c2", "c3"}
	matrix := []controllerv1alpha1.ConnectivityEntry{
		{SourceCluster: "c1", TargetClusters: []string{"c2"}},
		{SourceCluster: "c2", TargetClusters: []string{"c3"}},
	}

	pairs, err := svc.resolveCustom(clusters, matrix)
	require.NoError(t, err)
	assert.Len(t, pairs, 2)
	assertContainsPair(t, pairs, "c1", "c2", true)
	assertContainsPair(t, pairs, "c2", "c3", true)
}

func TestResolveCustom_UnknownClusterError(t *testing.T) {
	svc := &DefaultTopologyService{}
	clusters := []string{"c1", "c2"}
	matrix := []controllerv1alpha1.ConnectivityEntry{
		{SourceCluster: "c1", TargetClusters: []string{"c99"}},
	}

	_, err := svc.resolveCustom(clusters, matrix)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

func TestResolveAuto_NoForbidden(t *testing.T) {
	svc := &DefaultTopologyService{}
	clusters := []string{"c1", "c2", "c3"}
	policyNodes := []string{}

	pairs, err := svc.resolveAuto(clusters, policyNodes)
	require.NoError(t, err)
	assert.Len(t, pairs, 3)
	assertContainsPair(t, pairs, "c1", "c2", true)
	assertContainsPair(t, pairs, "c1", "c3", true)
	assertContainsPair(t, pairs, "c2", "c3", true)
}

func TestResolveAuto_WithForbiddenEdges(t *testing.T) {
	svc := &DefaultTopologyService{}
	clusters := []string{"c1", "c2", "c3", "c4"}
	policyNodes := []string{"c1"}

	pairs, err := svc.resolveAuto(clusters, policyNodes)
	require.NoError(t, err)
	assert.Len(t, pairs, 3)
	assertContainsPair(t, pairs, "c2", "c3", true)
	assertContainsPair(t, pairs, "c2", "c4", true)
	assertContainsPair(t, pairs, "c3", "c4", true)
	assertNotContainsPair(t, pairs, "c1", "c2")
	assertNotContainsPair(t, pairs, "c1", "c3")
	assertNotContainsPair(t, pairs, "c1", "c4")
}

func TestResolveAuto_MultipleForbiddenPolicies(t *testing.T) {
	svc := &DefaultTopologyService{}
	clusters := []string{"c1", "c2", "c3", "c4"}
	policyNodes := []string{"c1", "c2"}

	pairs, err := svc.resolveAuto(clusters, policyNodes)
	require.NoError(t, err)
	assert.Len(t, pairs, 1)
	assertContainsPair(t, pairs, "c3", "c4", true)
	assertNotContainsPair(t, pairs, "c1", "c2")
	assertNotContainsPair(t, pairs, "c1", "c3")
	assertNotContainsPair(t, pairs, "c1", "c4")
	assertNotContainsPair(t, pairs, "c2", "c3")
	assertNotContainsPair(t, pairs, "c2", "c4")
}

func TestPairKey(t *testing.T) {
	svc := &DefaultTopologyService{}
	assert.Equal(t, "a-b", svc.pairKey("a", "b"))
	assert.Equal(t, "a-b", svc.pairKey("b", "a"))
	assert.Equal(t, "cluster-1-cluster-2", svc.pairKey("cluster-1", "cluster-2"))
	assert.Equal(t, "cluster-1-cluster-2", svc.pairKey("cluster-2", "cluster-1"))
}

func assertContainsPair(t *testing.T, pairs []GatewayPair, source, target string, bidirectional bool) {
	for _, p := range pairs {
		if (p.Source == source && p.Target == target) || (p.Source == target && p.Target == source) {
			if p.Bidirectional == bidirectional {
				return
			}
		}
	}
	t.Errorf("Expected to find pair %s <-> %s (bidirectional=%v)", source, target, bidirectional)
}

func assertNotContainsPair(t *testing.T, pairs []GatewayPair, source, target string) {
	for _, p := range pairs {
		if (p.Source == source && p.Target == target) || (p.Source == target && p.Target == source) {
			t.Errorf("Expected NOT to find pair %s <-> %s", source, target)
		}
	}
}
