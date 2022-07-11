package service

import (
	"context"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//ValidateSliceqosConfigCreate is a function to validate the creation of SliceqosConfig
func ValidateSliceQosConfigCreate(ctx context.Context, sliceQoSConfig *controllerv1alpha1.SliceQoSConfig) error {
	var allErrs field.ErrorList
	err := validateSliceQosConfigAppliedInProjectNamespace(ctx, sliceQoSConfig)
	if err != nil {
		allErrs = append(allErrs, err)
	}
	err = validateSliceQosConfigSpec(ctx, sliceQoSConfig)
	if err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "controller.kubeslice.io", Kind: "SliceQosConfig"}, sliceQoSConfig.Name, allErrs)
}

func validateSliceQosConfigSpec(ctx context.Context, sliceQosConfig *controllerv1alpha1.SliceQoSConfig) *field.Error {
	// check bandwidth
	if sliceQosConfig.Spec.BandwidthCeilingKbps < sliceQosConfig.Spec.BandwidthGuaranteedKbps {
		return field.Invalid(field.NewPath("Spec").Child("BandwidthGuaranteedKbps"), sliceQosConfig.Spec.BandwidthGuaranteedKbps, "BandwidthGuaranteedKbps cannot be greater than BandwidthCeilingKbps")
	}
	return nil
}

// validateAppliedInProjectNamespace is a function to validate the if the SliceQosConfig is applied in project namespace or not
func validateSliceQosConfigAppliedInProjectNamespace(ctx context.Context, c *controllerv1alpha1.SliceQoSConfig) *field.Error {
	actualNamespace := corev1.Namespace{}
	exist, _ := util.GetResourceIfExist(ctx, client.ObjectKey{Name: c.Namespace}, &actualNamespace)
	if exist {
		if actualNamespace.Labels[util.LabelName] == "" {
			return field.Invalid(field.NewPath("metadata").Child("namespace"), c.Name, "SliceQosConfig must be applied on project namespace")
		}
	}
	return nil
}

// ValidateSliceqosConfigDelete is a function to validate the deletion of SliceqosConfig
func ValidateSliceQosConfigDelete(ctx context.Context, sliceQoSConfig *controllerv1alpha1.SliceQoSConfig) error {
	var allErrs field.ErrorList
	exists, slices, err := validateIfQosExistsOnAnySlice(ctx, sliceQoSConfig)
	if err != nil {
		return err
	}
	if exists {
		err := field.Forbidden(field.NewPath("SliceQoSConfig"), "The SliceqosProfile "+sliceQoSConfig.Name+" cannot be deleted. It is present on slices [ "+util.ArrayToString(slices)+" ]")
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(schema.GroupKind{Group: "controller.kubeslice.io", Kind: "SliceQosConfig"}, sliceQoSConfig.Name, allErrs)
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
		for _, workerslice := range workerSlices.Items {
			if _, ok := sliceMap[workerslice.Spec.SliceName]; !ok {
				sliceMap[workerslice.Spec.SliceName] = true
				slices = append(slices, workerslice.Spec.SliceName)
			}
		}
		return true, slices, nil
	}
	return false, slices, nil
}
