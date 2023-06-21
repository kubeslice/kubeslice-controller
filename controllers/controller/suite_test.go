/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/metrics"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"testing"
	"time"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	ossEvents "github.com/kubeslice/kubeslice-controller/events"

	"github.com/kubeslice/kubeslice-controller/service"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"

	"github.com/kubeslice/kubeslice-controller/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg           *rest.Config
	k8sClient     client.Client
	testEnv       *envtest.Environment
	svc           *service.Services
	eventRecorder events.EventRecorder
	ctx           context.Context
	cancel        context.CancelFunc
	controllerLog = util.NewLogger().With("name", "controllers")
)

const (
	timeout = time.Second * 10
	// duration = time.Second * 10
	interval              = time.Millisecond * 250
	controlPlaneNamespace = "kubeslice-controller"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		CRDInstallOptions: envtest.CRDInstallOptions{
			MaxTime: 60 * time.Second,
		},
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = clientgoscheme.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = controllerv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = workerv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	//+kubebuilder:scaffold:scheme

	ctx, cancel = context.WithCancel(context.TODO())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	mr := service.WithMetricsRecorder()
	ns := service.WithNameSpaceService(mr)
	rp := service.WithAccessControlRuleProvider()
	acs := service.WithAccessControlService(rp, mr)
	js := service.WithJobService()
	wscs := service.WithWorkerSliceConfigService(mr)
	ss := service.WithSecretService(mr)
	wsgs := service.WithWorkerSliceGatewayService(js, wscs, ss, mr)
	c := service.WithClusterService(ns, acs, wsgs, mr)
	wsi := service.WithWorkerServiceImportService(mr)
	se := service.WithServiceExportConfigService(wsi, mr)
	wsgrs := service.WithWorkerSliceGatewayRecyclerService()
	vpn := service.WithVpnKeyRotationService(wsgs, wscs)
	sc := service.WithSliceConfigService(ns, acs, wsgs, wscs, wsi, se, wsgrs, mr,vpn)
	sqcs := service.WithSliceQoSConfigService(wscs, mr)
	p := service.WithProjectService(ns, acs, c, sc, se, sqcs, mr)
	svc = service.WithServices(wscs, p, c, sc, se, wsgs, wsi, sqcs, wsgrs,vpn)

	service.ProjectNamespacePrefix = util.AppendHyphenAndPercentageSToString("kubeslice")
	rbacResourcePrefix := util.AppendHyphenToString("kubeslice-rbac")
	service.RoleBindingWorkerCluster = rbacResourcePrefix + "worker-%s"
	service.RoleBindingReadOnlyUser = rbacResourcePrefix + "ro-%s"
	service.RoleBindingReadWriteUser = rbacResourcePrefix + "rw-%s"
	service.ServiceAccountWorkerCluster = rbacResourcePrefix + "worker-%s"
	service.ServiceAccountReadOnlyUser = rbacResourcePrefix + "ro-%s"
	service.ServiceAccountReadWriteUser = rbacResourcePrefix + "rw-%s"

	controlPlaneNS := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: controlPlaneNamespace,
		},
	}
	Expect(k8sClient.Create(ctx, controlPlaneNS)).Should(Succeed())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	//setting up the event recorder
	eventRecorder = events.NewEventRecorder(k8sClient, k8sManager.GetScheme(), ossEvents.EventsMap, events.EventRecorderOptions{
		Version:   "v1alpha1",
		Cluster:   util.ClusterController,
		Component: util.ComponentController,
		Slice:     util.NotApplicable,
	})

	// setting up metrics collector
	go metrics.StartMetricsCollector(service.MetricPort, true)

	err = (&ProjectReconciler{
		Client:         k8sClient,
		Scheme:         k8sManager.GetScheme(),
		Log:            controllerLog.With("name", "Project"),
		EventRecorder:  &eventRecorder,
		ProjectService: svc.ProjectService,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&ClusterReconciler{
		Client:         k8sClient,
		Scheme:         k8sManager.GetScheme(),
		Log:            controllerLog.With("name", "Cluster"),
		EventRecorder:  &eventRecorder,
		ClusterService: svc.ClusterService,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())


	err = (&VpnKeyRotationReconciler{
		Client:                k8sClient,
		Scheme:                k8sManager.GetScheme(),
		Log:                   controllerLog.With("name", "VpnKeyRotationConfig"),
		VpnKeyRotationService: svc.VpnKeyRotationService,
		EventRecorder:         &eventRecorder,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
