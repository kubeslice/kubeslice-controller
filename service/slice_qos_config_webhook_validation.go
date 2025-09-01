package service

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ValidateSliceQosConfigCreate is a function to validate the creation of SliceQosConfig
func ValidateSliceQosConfigCreate(ctx context.Context, sliceQoSConfig *controllerv1alpha1.SliceQoSConfig) (admission.Warnings, error) {
	if err := validateSliceQosConfigAppliedInProjectNamespace(ctx, sliceQoSConfig); err != nil {
		return nil, apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceControllers, Kind: "SliceQosConfig"}, sliceQoSConfig.Name, field.ErrorList{err})
	}
	if err := validateSliceQosConfigSpec(ctx, sliceQoSConfig); err != nil {
		return nil, apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceControllers, Kind: "SliceQosConfig"}, sliceQoSConfig.Name, field.ErrorList{err})
	}
	return nil, nil
}

// ValidateSliceQosConfigUpdate is a function to validate the update of SliceQosConfig
func ValidateSliceQosConfigUpdate(ctx context.Context, sliceQoSConfig *controllerv1alpha1.SliceQoSConfig) (admission.Warnings, error) {
	if err := validateSliceQosConfigSpec(ctx, sliceQoSConfig); err != nil {
		return nil, apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceControllers, Kind: "SliceQosConfig"}, sliceQoSConfig.Name, field.ErrorList{err})
	}
	return nil, nil
}

// ValidateSliceQosConfigDelete is a function to validate the deletion of SliceQosConfig
func ValidateSliceQosConfigDelete(ctx context.Context, sliceQoSConfig *controllerv1alpha1.SliceQoSConfig) (admission.Warnings, error) {
	exists, slices, err := validateIfQosExistsOnAnySlice(ctx, sliceQoSConfig)
	if err != nil {
		return nil, err
	}
	if exists {
		err := field.Forbidden(field.NewPath("SliceQoSConfig"), "The SliceQoSProfile "+sliceQoSConfig.Name+" cannot be deleted. It is present on slices [ "+util.ArrayToString(slices)+" ]")
		return nil, apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceControllers, Kind: "SliceQosConfig"}, sliceQoSConfig.Name, field.ErrorList{err})
	}
	return nil, nil
}

func validateSliceQosConfigSpec(ctx context.Context, sliceQosConfig *controllerv1alpha1.SliceQoSConfig) *field.Error {
	// check bandwidth
	if sliceQosConfig.Spec.BandwidthCeilingKbps < sliceQosConfig.Spec.BandwidthGuaranteedKbps {
		return field.Invalid(field.NewPath("Spec").Child("BandwidthGuaranteedKbps"), sliceQosConfig.Spec.BandwidthGuaranteedKbps, "BandwidthGuaranteedKbps cannot be greater than BandwidthCeilingKbps")
	}
	return nil
}

// validateAppliedInProjectNamespace is a function to validate the if the SliceQosConfig is applied in project namespace or not
func validateSliceQosConfigAppliedInProjectNamespace(ctx context.Context, sliceQoSConfig *controllerv1alpha1.SliceQoSConfig) *field.Error {
	namespace := &corev1.Namespace{}
	exist, _ := util.GetResourceIfExist(ctx, client.ObjectKey{Name: sliceQoSConfig.Namespace}, namespace)
	if !exist || !util.CheckForProjectNamespace(namespace) {
		return field.Invalid(field.NewPath("metadata").Child("namespace"), sliceQoSConfig.Name, "SliceQosConfig must be applied on project namespace")
	}
	return nil
}

/* validateIfQosProfileExists function to check if qos profile exists on any of workerslices */
func validateIfQosExistsOnAnySlice(ctx context.Context, sliceQosConfig *controllerv1alpha1.SliceQoSConfig) (bool, []string, error) {
	workerSlices := &workerv1alpha1.WorkerSliceConfigList{}
	slices := make([]string, 0)
	ownerLabel := map[string]string{
		StandardQoSProfileLabel: sliceQosConfig.Name,
	}
	sliceMap := make(map[string]bool)
	err := util.ListResources(ctx, workerSlices, client.MatchingLabels(ownerLabel), client.InNamespace(sliceQosConfig.Namespace))
	if err != nil {
		return false, slices, err
	}
	if len(workerSlices.Items) > 0 {
		for _, workerSlice := range workerSlices.Items {
			if _, ok := sliceMap[workerSlice.Spec.SliceName]; !ok {
				sliceMap[workerSlice.Spec.SliceName] = true
				slices = append(slices, workerSlice.Spec.SliceName)
			}
		}
		return true, slices, nil
	}
	return false, slices, nil
}
