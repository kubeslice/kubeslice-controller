package service

import (
	"context"
	"testing"
	"time"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	ossEvents "github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-controller/service/mocks"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// Test_GlobalFlagBug_SkipsCertRotationForSecondSlice proves the bug:
// VpnKeyRotationService.jobCreationInProgress is process-global (not per-slice).
// When slice-A sets the flag to true, slice-B's call to triggerJobsForCertCreation
// is skipped. If stale Jobs from slice-B's prior rotation still exist (they are never
// deleted), verifyAllJobsAreCompleted returns JobStatusComplete, and the reconciler
// advances slice-B's CertificateExpiryTime + RotationCount WITHOUT generating new certs.
func Test_GlobalFlagBug_SkipsCertRotationForSecondSlice(t *testing.T) {
	clientMock := &utilMock.Client{}
	scheme := runtime.NewScheme()
	utilruntime.Must(controllerv1alpha1.AddToScheme(scheme))
	wg := &mocks.IWorkerSliceGatewayService{}
	ws := &mocks.IWorkerSliceConfigService{}
	eventRecorder := events.NewEventRecorder(clientMock, scheme, ossEvents.EventsMap, events.EventRecorderOptions{
		Version:   "v1alpha1",
		Cluster:   util.ClusterController,
		Component: util.ComponentController,
		Slice:     util.NotApplicable,
	})
	ctx := util.PrepareKubeSliceControllersRequestContext(
		context.Background(), clientMock, scheme, "BugTest", &eventRecorder,
	)

	vpn := VpnKeyRotationService{wsgs: wg, wscs: ws}

	past := metav1.NewTime(time.Now().Add(-2 * time.Hour)) // cert already expired

	makeVpnCR := func(sliceName string) *controllerv1alpha1.VpnKeyRotation {
		return &controllerv1alpha1.VpnKeyRotation{
			ObjectMeta: metav1.ObjectMeta{Name: sliceName, Namespace: "test-ns"},
			Spec: controllerv1alpha1.VpnKeyRotationSpec{
				SliceName:               sliceName,
				Clusters:                []string{"w1", "w2"},
				RotationInterval:        30,
				RotationCount:           1,
				CertificateCreationTime: &past,
				CertificateExpiryTime:   &past,
			},
		}
	}

	makeSliceCR := func(sliceName string) *controllerv1alpha1.SliceConfig {
		return &controllerv1alpha1.SliceConfig{
			ObjectMeta: metav1.ObjectMeta{Name: sliceName, Namespace: "test-ns"},
			Spec:       controllerv1alpha1.SliceConfigSpec{Clusters: []string{"w1", "w2"}},
		}
	}

	// Slice-A reconcile: flag is false → triggerJobsForCertCreation runs → GenerateCerts called once.
	clientMock.On("List", mock.Anything, mock.AnythingOfType("*v1alpha1.WorkerSliceGatewayList"), mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*workerv1alpha1.WorkerSliceGatewayList)
		list.Items = []workerv1alpha1.WorkerSliceGateway{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "gw-server"},
				Spec: workerv1alpha1.WorkerSliceGatewaySpec{
					GatewayHostType:     "Server",
					RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{GatewayName: "gw-client"},
					LocalGatewayConfig:  workerv1alpha1.SliceGatewayConfig{ClusterName: "w1"},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "gw-client"},
				Spec: workerv1alpha1.WorkerSliceGatewaySpec{
					GatewayHostType:    "Client",
					LocalGatewayConfig: workerv1alpha1.SliceGatewayConfig{ClusterName: "w2"},
				},
			},
		}
	}).Once()
	ws.On("ListWorkerSliceConfigs", mock.Anything, mock.Anything, mock.Anything).
		Return([]workerv1alpha1.WorkerSliceConfig{}, nil).Once()
	ws.On("ComputeClusterMap", mock.Anything, mock.Anything).
		Return(map[string]int{"w1": 1, "w2": 2}).Once()
	wg.On("BuildNetworkAddresses", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(util.WorkerSliceGatewayNetworkAddresses{}).Once()
	wg.On("GenerateCerts", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	_, _, err := vpn.reconcileVpnKeyRotationConfig(ctx, makeVpnCR("slice-a"), makeSliceCR("slice-a"))
	require.NoError(t, err)

	// PRECONDITION: flag is now true because slice-A set it.
	inProgress, _ := vpn.jobCreationInProgress.Load("test-ns/slice-a")
	require.True(t, inProgress == true,
		"precondition: slice-a must have set jobCreationInProgress=true")

	// Slice-B reconcile: after fix, slice-b has its own independent flag (false) →
	// triggerJobsForCertCreation runs for slice-b as well.
	clientMock.On("List", mock.Anything, mock.AnythingOfType("*v1alpha1.WorkerSliceGatewayList"), mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*workerv1alpha1.WorkerSliceGatewayList)
		list.Items = []workerv1alpha1.WorkerSliceGateway{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "gw-b-server"},
				Spec: workerv1alpha1.WorkerSliceGatewaySpec{
					GatewayHostType:     "Server",
					RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{GatewayName: "gw-b-client"},
					LocalGatewayConfig:  workerv1alpha1.SliceGatewayConfig{ClusterName: "w1"},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "gw-b-client"},
				Spec: workerv1alpha1.WorkerSliceGatewaySpec{
					GatewayHostType:    "Client",
					LocalGatewayConfig: workerv1alpha1.SliceGatewayConfig{ClusterName: "w2"},
				},
			},
		}
	}).Once()
	ws.On("ListWorkerSliceConfigs", mock.Anything, mock.Anything, mock.Anything).
		Return([]workerv1alpha1.WorkerSliceConfig{}, nil).Once()
	ws.On("ComputeClusterMap", mock.Anything, mock.Anything).
		Return(map[string]int{"w1": 1, "w2": 2}).Once()
	wg.On("BuildNetworkAddresses", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(util.WorkerSliceGatewayNetworkAddresses{}).Once()
	wg.On("GenerateCerts", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	clientMock.On("Create", mock.Anything, mock.Anything).Return(nil).Maybe()

	_, _, errB := vpn.reconcileVpnKeyRotationConfig(ctx, makeVpnCR("slice-b"), makeSliceCR("slice-b"))
	require.NoError(t, errB)

	// FIX ASSERTION: GenerateCerts called once for slice-a AND once for slice-b — two total.
	wg.AssertNumberOfCalls(t, "GenerateCerts", 2)
}
