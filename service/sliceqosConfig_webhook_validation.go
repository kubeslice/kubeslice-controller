package service

import (
	"context"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ValidateProjectCreate is a function to validate the creation of project
func ValidateSliceqosConfigDelete(ctx context.Context, sliceQoSConfig *controllerv1alpha1.SliceQoSConfig) error {
	var allErrs field.ErrorList
	exists, err := validateIfQosProfileExists(ctx, sliceQoSConfig)
	if err != nil {
		return err
	}
	if exists {
		err := field.Forbidden(field.NewPath("Sliceqos"), "The SliceqosProfile "+sliceQoSConfig.Name+" cannot be deleted. It is present on workerslices")
		allErrs = append(allErrs, err)
	}
	//	allErrs = append(allErrs, err)
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "controller.kubeslice.io", Kind: "SliceQosConfig"}, sliceQoSConfig.Name, allErrs)
}

/* validateIfQosProfileExists function to check if qos profile exists on any of workerslices */
func validateIfQosProfileExists(ctx context.Context, sliceQosConfig *controllerv1alpha1.SliceQoSConfig) (bool, error) {
	workerSlices := &workerv1alpha1.WorkerSliceConfigList{}
	ownerLabel := map[string]string{
		StandardQoSProfileLabel: sliceQosConfig.Name,
	}
	err := util.ListResources(ctx, workerSlices, client.MatchingLabels(ownerLabel), client.InNamespace(sliceQosConfig.Namespace))
	if err != nil {
		return false, err
	}
	if len(workerSlices.Items) > 0 {
		return true, nil
	}
	return false, nil
}
