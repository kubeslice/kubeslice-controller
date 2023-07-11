package service

import (
	"context"
	"fmt"
	"testing"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	ossEvents "github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

type validateVpnKeyRotationCreateTestCase struct {
	name               string
	arg                *controllerv1alpha1.VpnKeyRotation
	expectedErr        error
	sliceConfigPresent bool
}

func Test_validateVpnKeyRotationCreate(t *testing.T) {
	testCase := []validateVpnKeyRotationCreateTestCase{
		{
			name: "should return nil if name matches the original slice name",
			arg: &controllerv1alpha1.VpnKeyRotation{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-slice",
				},
				Spec: controllerv1alpha1.VpnKeyRotationSpec{
					SliceName: "test-slice",
				},
			},
			expectedErr:        nil,
			sliceConfigPresent: true,
		},
		{
			name: "should return error if slicename is empty",
			arg: &controllerv1alpha1.VpnKeyRotation{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-slice",
				},
				Spec: controllerv1alpha1.VpnKeyRotationSpec{
					SliceName: "",
				},
			},
			expectedErr:        fmt.Errorf("invalid config,.spec.sliceName could not be empty"),
			sliceConfigPresent: true,
		},
		{
			name: "should return error if name does not macthes original slice name",
			arg: &controllerv1alpha1.VpnKeyRotation{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-slice",
				},
				Spec: controllerv1alpha1.VpnKeyRotationSpec{
					SliceName: "test-slice-1",
				},
			},
			expectedErr:        fmt.Errorf("invalid config, name should match with slice name"),
			sliceConfigPresent: true,
		},
		{
			name: "should return error if sliceconfig is not present",
			arg: &controllerv1alpha1.VpnKeyRotation{
				ObjectMeta: v1.ObjectMeta{
					Name: "test-slice",
				},
				Spec: controllerv1alpha1.VpnKeyRotationSpec{
					SliceName: "test-slice",
				},
			},
			expectedErr:        fmt.Errorf("sliceconfig test-slice not found"),
			sliceConfigPresent: false,
		},
	}
	for _, tc := range testCase {
		runValidateVpnKeyRotationCreateTest(t, tc)
	}
}

func runValidateVpnKeyRotationCreateTest(t *testing.T, tc validateVpnKeyRotationCreateTestCase) {
	ctx, clientMock := setupValidationTestCase()
	if tc.sliceConfigPresent {
		clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	} else {
		clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("sliceconfig test-slice not found"))
	}
	gotErr := ValidateVpnKeyRotationCreate(ctx, tc.arg)
	require.Equal(t, tc.expectedErr, gotErr)
}

func setupValidationTestCase() (context.Context, *utilMock.Client) {
	clientMock := &utilMock.Client{}
	scheme := runtime.NewScheme()
	utilruntime.Must(controllerv1alpha1.AddToScheme(scheme))
	eventRecorder := events.NewEventRecorder(clientMock, scheme, ossEvents.EventsMap, events.EventRecorderOptions{
		Version:   "v1alpha1",
		Cluster:   util.ClusterController,
		Component: util.ComponentController,
		Slice:     util.NotApplicable,
	})
	return util.PrepareKubeSliceControllersRequestContext(context.Background(), clientMock, scheme, "ClusterTestController", &eventRecorder), clientMock
}

type validateVpnKeyRotationDeleteTestCase struct {
	name        string
	arg         *controllerv1alpha1.VpnKeyRotation
	expectedErr error
	deletionTs  v1.Time
}

func Test_validateVpnKeyRotationDelete(t *testing.T) {
	testCase := []validateVpnKeyRotationDeleteTestCase{
		{
			name: "should return nil if slice is under deletion",
			arg: &controllerv1alpha1.VpnKeyRotation{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "test-ns",
				},
			},
			deletionTs:  v1.Now(),
			expectedErr: nil,
		},
		{
			name: "should return error if slice is not deleted/under deletion",
			arg: &controllerv1alpha1.VpnKeyRotation{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "test-ns",
				},
			},
			expectedErr: fmt.Errorf("vpnkeyrotation config %s not allowed to delete unless sliceconfig is deleted", "test-slice"),
		},
	}
	for _, tc := range testCase {
		runValidateVpnKeyRotationDeleteTest(t, tc)
	}
}

func runValidateVpnKeyRotationDeleteTest(t *testing.T, tc validateVpnKeyRotationDeleteTestCase) {
	ctx, clientMock := setupValidationTestCase()
	clientMock.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceConfig)
		arg.ObjectMeta = v1.ObjectMeta{
			DeletionTimestamp: &tc.deletionTs,
		}
	})
	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()

	gotErr := ValidateVpnKeyRotationDelete(ctx, tc.arg)
	require.Equal(t, tc.expectedErr, gotErr)
}
