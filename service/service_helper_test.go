package service

import (
	"testing"

	"github.com/dailymotion/allure-go"
	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestServiceHelperSuite(t *testing.T) {
	for k, v := range ServiceHelperTestBed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var ServiceHelperTestBed = map[string]func(*testing.T){
	"SliceGatewayServiceMap generation":               test_getSliceGwSvcTypes,
	"SliceGatewayServiceMap generation with wildcard": test_getSliceGwSvcTypes_wildcard,
}

func test_getSliceGwSvcTypes(t *testing.T) {
	sliceConfig := &v1alpha1.SliceConfig{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-ns",
		},
		Spec: v1alpha1.SliceConfigSpec{
			SliceGatewayProvider: v1alpha1.WorkerSliceGatewayProvider{
				SliceGatewayServiceType: []v1alpha1.SliceGatewayServiceType{
					{
						Cluster:  "c1",
						Type:     "Nodeport",
						Protocol: "TCP",
					},
					{
						Cluster:  "c2",
						Type:     "LoadBalancer",
						Protocol: "UDP",
					},
				},
			},
		},
	}
	sliceGwSvcMap := getSliceGwSvcTypes(sliceConfig)
	// assertions
	require.Equal(t, len(sliceGwSvcMap), 2)
	require.Contains(t, sliceGwSvcMap, "c1")
	require.Contains(t, sliceGwSvcMap, "c2")
	require.EqualValues(t, &v1alpha1.SliceGatewayServiceType{Cluster: "c1", Type: "Nodeport", Protocol: "TCP"}, sliceGwSvcMap["c1"])
	require.EqualValues(t, &v1alpha1.SliceGatewayServiceType{Cluster: "c2", Type: "LoadBalancer", Protocol: "UDP"}, sliceGwSvcMap["c2"])
}

func test_getSliceGwSvcTypes_wildcard(t *testing.T) {
	sliceConfig := &v1alpha1.SliceConfig{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-ns",
		},
		Spec: v1alpha1.SliceConfigSpec{
			Clusters: []string{
				"c1", "c2", "cx",
			},
			SliceGatewayProvider: v1alpha1.WorkerSliceGatewayProvider{
				SliceGatewayServiceType: []v1alpha1.SliceGatewayServiceType{
					{
						Cluster: "*",
						Type:    "Nodeport",
					},
				},
			},
		},
	}
	sliceGwSvcMap := getSliceGwSvcTypes(sliceConfig)
	// assertions
	require.Equal(t, len(sliceGwSvcMap), 3)
	require.Contains(t, sliceGwSvcMap, "c1")
	require.Contains(t, sliceGwSvcMap, "c2")
	require.Contains(t, sliceGwSvcMap, "cx")
	for _, cluster := range []string{"c1", "c2", "cx"} {
		require.EqualValues(t, &v1alpha1.SliceGatewayServiceType{Cluster: "*", Type: "Nodeport"}, sliceGwSvcMap[cluster])
	}
}
