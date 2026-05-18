package service

import (
	"testing"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Test_ClusterNotFound_GatewayCreationSilentlySkipped proves the bug:
// When a Cluster CR referenced in SliceConfig.Spec.Clusters is not found,
// createMinimumGatewaysIfNotExists returns (ctrl.Result{}, nil) instead of an error.
// The SliceConfig reconcile sees nil, treats reconciliation as successful, and the
// work queue drops it — gateways are never created and the slice has no connectivity.
func Test_ClusterNotFound_GatewayCreationSilentlySkipped(t *testing.T) {
	_, _, _, svc, _, clientMock, _, ctx, mMock := setupWorkerSliceGatewayTest("test-slice", "test-ns")

	mMock.On("WithProject", mock.Anything).Return(mMock)
	mMock.On("WithNamespace", mock.Anything).Return(mMock)
	mMock.On("WithSlice", mock.Anything).Return(mMock)

	// Cluster CR does not exist — API server returns NotFound, which GetResourceIfExist
	// converts to (false, nil). The bug is that the caller then returns nil to the work queue.
	notFound := k8serrors.NewNotFound(schema.GroupResource{Group: "controller.kubeslice.io", Resource: "clusters"}, "missing-cluster")
	clientMock.On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.Cluster"), mock.Anything).
		Return(notFound)

	result, err := svc.createMinimumGatewaysIfNotExists(
		ctx,
		"test-slice",
		[]string{"missing-cluster"},
		"test-ns",
		map[string]string{"original-slice-name": "test-slice"},
		map[string]int{"missing-cluster": 1},
		"192.168.0.0/16",
		"/24",
		map[string]*controllerv1alpha1.SliceGatewayServiceType{},
	)

	// FIX: err must be non-nil when a referenced cluster is not found,
	// so the SliceConfig reconcile fails and the work queue requeues it.
	require.Error(t, err,
		"cluster not found must return an error so the SliceConfig is requeued")
	require.Contains(t, err.Error(), "missing-cluster")
	_ = result
}
