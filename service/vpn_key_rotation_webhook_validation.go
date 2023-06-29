package service

import (
	"context"
	"fmt"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func ValidateVpnKeyRotationDelete(ctx context.Context, r *controllerv1alpha1.VpnKeyRotation) error {
	slice := &controllerv1alpha1.SliceConfig{}
	found, err := util.GetResourceIfExist(ctx, client.ObjectKey{
		Name:      r.Spec.SliceName,
		Namespace: r.Namespace,
	}, slice)
	if err != nil {
		return err
	}
	if found && slice.ObjectMeta.DeletionTimestamp.IsZero() {
		return fmt.Errorf("sliceconfig %s not allowed to delete unless slice is deleted", r.Name)
	}
	// if not found or timestamp is non-zero,this means slice is deleted/under deletion.
	return nil
}
