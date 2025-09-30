package e2e

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	controllerV1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
)

// Global variables for E2E tests
var (
	k8sClient client.Client
	ctx       context.Context
	scheme    = runtime.NewScheme()
)

const (
	controlPlaneNamespace = "system"

	timeout  = time.Second * 30
	interval = time.Millisecond * 500
)

func ObjectKey(namespace, name string) client.ObjectKey {
	return client.ObjectKey{Namespace: namespace, Name: name}
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "KubeSlice E2E Suite")
}

var _ = BeforeSuite(func() {
	By("Bootstrapping test environment")

	// Register Kubernetes core + custom schemes
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(controllerV1alpha1.AddToScheme(scheme))

	// Load kubeconfig
	cfg, err := config.GetConfig()
	Expect(err).NotTo(HaveOccurred())

	// Create k8s client
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())

	ctx = context.Background()
})

var _ = AfterSuite(func() {
	By("Tearing down test environment (if needed)")
})
