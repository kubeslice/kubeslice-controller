package service

import (
	"context"
	"fmt"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
)

func ValidateVpnKeyRotationCreate(ctx context.Context, r *controllerv1alpha1.VpnKeyRotation) error {
	if r.Spec.SliceName == "" {
		return fmt.Errorf("invalid config,.spec.sliceName could not be empty")
	}
	if r.Name != r.Spec.SliceName {
		return fmt.Errorf("invalid config, name should match with slice name")
	}
	return nil
}
