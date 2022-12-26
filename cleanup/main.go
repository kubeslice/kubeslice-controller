package main

import (
	"context"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/cleanup/service"
	"github.com/kubeslice/kubeslice-controller/util"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(controllerv1alpha1.AddToScheme(scheme))
	utilruntime.Must(workerv1alpha1.AddToScheme(scheme))
}

func main() {
	// Setup
	config := ctrl.GetConfigOrDie()
	c, _ := client.New(config, client.Options{Scheme: scheme})
	ctx := util.PrepareKubeSliceControllersRequestContext(context.Background(), c, c.Scheme(), "CleanupContext")
	cs := &service.CleanupService{}
	// Cleanup resources
	cs.CleanupResources(ctx)
}
