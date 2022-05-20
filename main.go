/*
 * 	Copyright (c) 2022 Avesha, Inc. All rights reserved. # # SPDX-License-Identifier: Apache-2.0
 *
 * 	Licensed under the Apache License, Version 2.0 (the "License");
 * 	you may not use this file except in compliance with the License.
 * 	You may obtain a copy of the License at
 *
 * 	http://www.apache.org/licenses/LICENSE-2.0
 *
 * 	Unless required by applicable law or agreed to in writing, software
 * 	distributed under the License is distributed on an "AS IS" BASIS,
 * 	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * 	See the License for the specific language governing permissions and
 * 	limitations under the License.
 */

package main

import (
	"flag"
	"fmt"
	"os"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/controllers/controller"
	"github.com/kubeslice/kubeslice-controller/controllers/worker"
	"github.com/kubeslice/kubeslice-controller/service"
	"github.com/kubeslice/kubeslice-controller/util"
	hubZap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(controllerv1alpha1.AddToScheme(scheme))
	utilruntime.Must(workerv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	initialize(service.WithServices())
}

func initLogger(logLevel zapcore.Level) {

	config := hubZap.NewDevelopmentConfig()
	config.Level = hubZap.NewAtomicLevelAt(logLevel)
	log, _ := config.Build()
	hubZap.ReplaceGlobals(log)
}

func initialize(services *service.Services) {
	// get metrics address from env
	var metricsAddr string
	// get enableLeaderElection from env
	var enableLeaderElection bool
	// get probe address from env
	var probeAddr string
	// get rbac resource prefix from env
	var rbacResourcePrefix string
	// get project name space prefix from env
	var projectNameSpacePrefixFromCustomer string
	// get log level from env
	var logLevel string
	// get controllerEndpoint from env
	var controllerEndpoint string
	// get job image from env
	var jobImage string
	// get job image pull policy credential from env
	var jobCredential string
	// get job service account from env
	var jobServiceAccount string

	flag.StringVar(&rbacResourcePrefix, "rbac-resource-prefix", service.RbacResourcePrefix, "RBAC resource prefix")
	flag.StringVar(&projectNameSpacePrefixFromCustomer, "project-namespace-prefix", service.ProjectNamespacePrefix, fmt.Sprintf("Overrides the default %s kubeslice namespace", service.ProjectNamespacePrefix))
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&logLevel, "log-level", "info", "Valid Log levels: debug,error,info. Defaults to info level")
	flag.StringVar(&controllerEndpoint, "controller-end-point", service.ControllerEndpoint, "The address the controller endpoint binds to.")
	flag.StringVar(&jobImage, "ovpn-job-image", service.JobImage, "The image to use for the ovpn cert generator job")
	flag.StringVar(&jobCredential, "ovpn-job-cred", service.JobCredential, "The credential to pull the ovpn job image")
	flag.StringVar(&jobServiceAccount, "ovpn-job-sa", service.JobServiceAccount, "The service account to use for the ovpn job")

	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	flag.Parse()

	// initialize logger
	zapLogLevel := util.GetZapLogLevel(logLevel)
	initLogger(zapLogLevel)
	opts := zap.Options{
		Development: true,
		Level:       zapLogLevel,
	}
	opts.BindFlags(flag.CommandLine)
	util.Loglevel = zapLogLevel
	util.LoglevelString = logLevel
	service.ControllerEndpoint = controllerEndpoint
	service.JobImage = jobImage
	service.JobCredential = jobCredential
	service.JobServiceAccount = jobServiceAccount
	service.ProjectNamespacePrefix = util.AppendHyphenAndPercentageSToString(projectNameSpacePrefixFromCustomer)
	rbacResourcePrefix = util.AppendHyphenToString(rbacResourcePrefix)
	service.RoleBindingWorkerCluster = rbacResourcePrefix + "worker-%s"
	service.RoleBindingReadOnlyUser = rbacResourcePrefix + "ro-%s"
	service.RoleBindingReadWriteUser = rbacResourcePrefix + "rw-%s"
	service.ServiceAccountWorkerCluster = rbacResourcePrefix + "worker-%s"
	service.ServiceAccountReadOnlyUser = rbacResourcePrefix + "ro-%s"
	service.ServiceAccountReadWriteUser = rbacResourcePrefix + "rw-%s"
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// initialize metrics
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "6a2ced6b.kubeslice.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	// initialize controller with Project Kind
	if err = (&controller.ProjectReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		Log:            ctrl.Log.WithName("controllers").WithName("Project"),
		ProjectService: services.ProjectService,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Project")
		os.Exit(1)
	}
	// initialize controller with Cluster Kind
	if err = (&controller.ClusterReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		Log:            ctrl.Log.WithName("controllers").WithName("Cluster"),
		ClusterService: services.ClusterService,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Cluster")
		os.Exit(1)
	}
	// initialize controller with SliceConfig Kind
	if err = (&controller.SliceConfigReconciler{
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		Log:                ctrl.Log.WithName("controllers").WithName("SliceConfig"),
		SliceConfigService: services.SliceConfigService,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SliceConfig")
		os.Exit(1)
	}
	// initialize controller with ServiceExportConfig Kind
	if err = (&controller.ServiceExportConfigReconciler{
		Client:                     mgr.GetClient(),
		Scheme:                     mgr.GetScheme(),
		Log:                        ctrl.Log.WithName("controllers").WithName("ServiceExportConfig"),
		ServiceExportConfigService: services.ServiceExportConfigService,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ServiceExportConfig")
		os.Exit(1)
	}
	if err = (&worker.WorkerSliceGatewayReconciler{
		Client:                    mgr.GetClient(),
		Scheme:                    mgr.GetScheme(),
		Log:                       ctrl.Log.WithName("controllers").WithName("WorkerSliceGateway"),
		WorkerSliceGatewayService: services.WorkerSliceGatewayService,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "WorkerSliceGateway")
		os.Exit(1)
	}
	if err = (&worker.WorkerSliceConfigReconciler{
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		Log:                ctrl.Log.WithName("controllers").WithName("WorkerSliceConfig"),
		WorkerSliceService: services.WorkerSliceConfigService,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "WorkerSliceConfig")
		os.Exit(1)
	}
	if err = (&worker.WorkerServiceImportReconciler{
		Client:                     mgr.GetClient(),
		Scheme:                     mgr.GetScheme(),
		Log:                        ctrl.Log.WithName("controllers").WithName("WorkerServiceImport"),
		WorkerServiceImportService: services.WorkerServiceImportService,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "WorkerServiceImport")
		os.Exit(1)
	}

	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err = (&controllerv1alpha1.Project{}).SetupWebhookWithManager(mgr, service.ValidateProjectCreate, service.ValidateProjectUpdate); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Project")
			os.Exit(1)
		}
		if err = (&controllerv1alpha1.Cluster{}).SetupWebhookWithManager(mgr, service.ValidateClusterCreate, service.ValidateClusterUpdate, service.ValidateClusterDelete); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Cluster")
			os.Exit(1)
		}
		if err = (&controllerv1alpha1.SliceConfig{}).SetupWebhookWithManager(mgr, service.ValidateSliceConfigCreate, service.ValidateSliceConfigUpdate, service.ValidateSliceConfigDelete); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "SliceConfig")
			os.Exit(1)
		}
		if err = (&controllerv1alpha1.ServiceExportConfig{}).SetupWebhookWithManager(mgr, service.ValidateServiceExportConfigCreate, service.ValidateServiceExportConfigUpdate); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "ServiceExportConfig")
			os.Exit(1)
		}
		if err = (&workerv1alpha1.WorkerSliceConfig{}).SetupWebhookWithManager(mgr, service.ValidateWorkerSliceConfigUpdate); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "WorkerSliceConfig")
			os.Exit(1)
		}
		if err = (&workerv1alpha1.WorkerSliceGateway{}).SetupWebhookWithManager(mgr, service.ValidateWorkerSliceGatewayUpdate); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "WorkerSliceGateway")
			os.Exit(1)
		}
	}
	//+kubebuilder:scaffold:builder

	if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err = mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err = mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

//All Controller RBACs goes here.

//+kubebuilder:rbac:groups=controller.kubeslice.io,resources=projects;clusters;sliceconfigs;serviceexportconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=controller.kubeslice.io,resources=projects/status;clusters/status;sliceconfigs/status;serviceexportconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=controller.kubeslice.io,resources=projects/finalizers;clusters/finalizers;sliceconfigs/finalizers;serviceexportconfigs/finalizers,verbs=update

//+kubebuilder:rbac:groups=worker.kubeslice.io,resources=workersliceconfigs;workerserviceimports;workerslicegateways,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=worker.kubeslice.io,resources=workersliceconfigs/status;workerserviceimports/status;workerslicegateways/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=worker.kubeslice.io,resources=workersliceconfigs/finalizers;workerserviceimports/finalizers;workerslicegateways/finalizers ,verbs=update

//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete;escalate
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;escalate;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;escalate;update;patch;create
//+kubebuilder:rbac:groups="batch",resources=jobs,verbs=get;list;watch;create;update;patch;delete

//+kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings;roles;clusterroles,verbs=get;list;watch;create;update;patch;delete
