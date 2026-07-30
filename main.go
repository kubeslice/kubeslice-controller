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
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"

	ossEvents "github.com/kubeslice/kubeslice-controller/events"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/controllers/controller"
	"github.com/kubeslice/kubeslice-controller/controllers/worker"
	"github.com/kubeslice/kubeslice-controller/metrics"
	"github.com/kubeslice/kubeslice-controller/pkg/ha"
	"github.com/kubeslice/kubeslice-controller/service"
	"github.com/kubeslice/kubeslice-controller/util"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	//+kubebuilder:scaffold:imports
)

var (
	scheme        = runtime.NewScheme()
	setupLog      = util.NewLogger().With("name", "setup")
	controllerLog = util.NewLogger().With("name", "controllers")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(controllerv1alpha1.AddToScheme(scheme))
	utilruntime.Must(workerv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	// Compile time dependency injection
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
	sc := service.WithSliceConfigService(ns, acs, wsgs, wscs, wsi, se, wsgrs, mr, vpn)
	sqcs := service.WithSliceQoSConfigService(wscs, mr)
	p := service.WithProjectService(ns, acs, c, sc, se, sqcs, mr)
	initialize(service.WithServices(wscs, p, c, sc, se, wsgs, wsi, sqcs, wsgrs, vpn))
}

func initialize(services *service.Services) {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	// get enableLeaderElection from env
	var enableLeaderElection bool
	// get probe address from env
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)
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
	// get prometheus endpoint from environment
	var prometheusServiceEndpoint string
	// HA (Active/Standby cross-cluster) configuration — see ADR #293 / issue #294
	var haMode string
	var haIdentity string
	var haActiveKubeconfig string
	var haLeaseNamespace string
	var haLeaseDuration time.Duration
	var haRenewDeadline time.Duration
	var haRetryPeriod time.Duration
	var haPaddingSeconds time.Duration
	var haSyncWorkers int
	var haSyncInterval time.Duration
	var haSelfCABundlePath string

	flag.StringVar(&rbacResourcePrefix, "rbac-resource-prefix", service.RbacResourcePrefix, "RBAC resource prefix")
	flag.StringVar(&projectNameSpacePrefixFromCustomer, "project-namespace-prefix", service.ProjectNamespacePrefix, fmt.Sprintf("Overrides the default %s kubeslice namespace", service.ProjectNamespacePrefix))
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")

	flag.StringVar(&logLevel, "log-level", "info", "Valid Log levels: debug,error,info. Defaults to info level")
	flag.StringVar(&controllerEndpoint, "controller-end-point", service.ControllerEndpoint, "The address the controller endpoint binds to.")
	flag.StringVar(&jobImage, "ovpn-job-image", service.JobImage, "The image to use for the ovpn cert generator job")
	flag.StringVar(&jobCredential, "ovpn-job-cred", service.JobCredential, "The credential to pull the ovpn job image")
	flag.StringVar(&jobServiceAccount, "ovpn-job-sa", service.JobServiceAccount, "The service account to use for the ovpn job")
	flag.StringVar(&prometheusServiceEndpoint, "prometheus-service-endpoint", metrics.PROMETHEUS_SERVICE_ENDPOINT, "PROMETHEUS SERVICE ENDPOINT")

	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	// Cross-cluster HA flags. --ha-mode=standalone (default) preserves today's behaviour.
	flag.StringVar(&haMode, "ha-mode", "standalone", `Cross-cluster HA mode: "active", "standby", or "standalone" (default).`)
	flag.StringVar(&haIdentity, "ha-identity", "", "Stable per-cluster identity recorded in the Lease (defaults to the hostname).")
	flag.StringVar(&haActiveKubeconfig, "ha-active-kubeconfig", "", "Path to the Active hub kubeconfig; required in standby mode.")
	flag.StringVar(&haLeaseNamespace, "ha-lease-namespace", os.Getenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE"), "Namespace for the HA Lease; defaults to the controller's own namespace (KUBESLICE_CONTROLLER_MANAGER_NAMESPACE), where the leader-election Role grants leases. Empty falls back to the pkg/ha default.")
	flag.DurationVar(&haLeaseDuration, "ha-lease-duration", ha.DefaultLeaseDuration, "HA Lease duration.")
	flag.DurationVar(&haRenewDeadline, "ha-renew-deadline", ha.DefaultRenewDeadline, "Deadline for the Active to renew its Lease before releasing leadership.")
	flag.DurationVar(&haRetryPeriod, "ha-retry-period", ha.DefaultRetryPeriod, "Interval between Lease renew/watch attempts.")
	flag.DurationVar(&haPaddingSeconds, "ha-padding-seconds", ha.DefaultPaddingSeconds, "Extra buffer a Standby waits before treating the Active Lease as stale.")
	flag.IntVar(&haSyncWorkers, "ha-sync-workers", ha.DefaultSyncWorkers, "Number of workers draining the Standby's remote-mirror workqueue.")
	flag.DurationVar(&haSyncInterval, "ha-sync-interval", ha.DefaultPruneInterval, "How often the Standby prunes mirrored objects that no longer exist on the Active hub.")
	flag.StringVar(&haSelfCABundlePath, "ha-self-ca-bundle-path", ha.DefaultSelfCABundlePath, "Path to this hub's own API server CA, published in status.activeController.caBundle. Unreadable is not fatal; publication continues without it.")

	flag.Parse()

	// initialize logger
	if logLevel == "" {
		logLevel = "info"
	}
	zapLogLevel := util.GetZapLogLevel(logLevel)
	opts := zap.Options{
		Development: false,
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
	metrics.PROMETHEUS_SERVICE_ENDPOINT = prometheusServiceEndpoint
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Create watchers for metrics and webhooks certificates
	var metricsCertWatcher, webhookCertWatcher *certwatcher.CertWatcher

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		var err error
		webhookCertWatcher, err = certwatcher.New(
			filepath.Join(webhookCertPath, webhookCertName),
			filepath.Join(webhookCertPath, webhookCertKey),
		)
		if err != nil {
			setupLog.Error(err, "Failed to initialize webhook certificate watcher")
			os.Exit(1)
		}

		webhookTLSOpts = append(webhookTLSOpts, func(config *tls.Config) {
			config.GetCertificate = webhookCertWatcher.GetCertificate
		})
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: webhookTLSOpts,
	})

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		var err error
		metricsCertWatcher, err = certwatcher.New(
			filepath.Join(metricsCertPath, metricsCertName),
			filepath.Join(metricsCertPath, metricsCertKey),
		)
		if err != nil {
			setupLog.Error(err, "to initialize metrics certificate watcher", "error", err)
			os.Exit(1)
		}

		metricsServerOptions.TLSOpts = append(metricsServerOptions.TLSOpts, func(config *tls.Config) {
			config.GetCertificate = metricsCertWatcher.GetCertificate
		})
	}

	// initialize metrics
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "6a2ced6b.kubeslice.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	//setting up the event recorder
	eventRecorder := events.NewEventRecorder(mgr.GetClient(), mgr.GetScheme(), ossEvents.EventsMap, events.EventRecorderOptions{
		Version:   "v1alpha1",
		Cluster:   util.ClusterController,
		Component: util.ComponentController,
		Slice:     util.NotApplicable,
	})
	// setting up metrics collector
	go metrics.StartMetricsCollector(service.MetricPort, true)

	// Set up cross-cluster HA leader election (ADR #293 / issue #294). In
	// standalone mode (the default) the elector is always the leader, so the
	// reconciler write-fence is a no-op and behaviour is unchanged.
	haRunMode := ha.ParseHAMode(haMode)
	localHAClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "unable to build HA local client")
		os.Exit(1)
	}
	var remoteHAClient client.Client
	var remoteHACfg *rest.Config
	if haRunMode == ha.ModeStandby {
		if haActiveKubeconfig == "" {
			setupLog.Error(fmt.Errorf("missing --ha-active-kubeconfig"), "standby mode requires the Active hub kubeconfig")
			os.Exit(1)
		}
		var cfgErr error
		remoteHACfg, cfgErr = clientcmd.BuildConfigFromFlags("", haActiveKubeconfig)
		if cfgErr != nil {
			setupLog.Error(cfgErr, "unable to load Active hub kubeconfig", "path", haActiveKubeconfig)
			os.Exit(1)
		}
		remoteHAClient, cfgErr = client.New(remoteHACfg, client.Options{Scheme: scheme})
		if cfgErr != nil {
			setupLog.Error(cfgErr, "unable to build remote client for Active hub")
			os.Exit(1)
		}
	}
	leaderElector := ha.NewClusterLeaderElector(localHAClient, remoteHAClient, ha.Options{
		Mode:           haRunMode,
		Identity:       haIdentity,
		LeaseNamespace: haLeaseNamespace,
		LeaseDuration:  haLeaseDuration,
		RenewDeadline:  haRenewDeadline,
		RetryPeriod:    haRetryPeriod,
		PaddingSeconds: haPaddingSeconds,
		Log:            controllerLog.With("name", "ha"),
	})
	setupLog.Info("high availability configured", "mode", haRunMode, "identity", leaderElector.Identity())

	// RemoteSyncer mirrors CRDMirrorSet from the Active hub onto this
	// cluster; a no-op in any mode other than standby (issue #295). Reuses
	// the same remote config and local client the elector above already
	// built rather than loading the kubeconfig twice.
	remoteSyncer, err := ha.NewRemoteSyncer(localHAClient, remoteHACfg, scheme, haRunMode, ha.RemoteSyncerOptions{
		Resources:     ha.FullMirrorSet(),
		Workers:       haSyncWorkers,
		PruneInterval: haSyncInterval,
		EventRecorder: eventRecorder,
		Log:           controllerLog.With("name", "ha-remote-syncer"),
	})
	if err != nil {
		setupLog.Error(err, "unable to build HA remote syncer")
		os.Exit(1)
	}

	// initialize controller with Project Kind
	if err = (&controller.ProjectReconciler{
		LeaderElector:  leaderElector,
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		Log:            controllerLog.With("name", "Project"),
		ProjectService: services.ProjectService,
		EventRecorder:  &eventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Project")
		os.Exit(1)
	}
	// initialize controller with Cluster Kind
	if err = (&controller.ClusterReconciler{
		LeaderElector:  leaderElector,
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		Log:            controllerLog.With("name", "Cluster"),
		ClusterService: services.ClusterService,
		EventRecorder:  &eventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Cluster")
		os.Exit(1)
	}
	// initialize controller with SliceConfig Kind
	if err = (&controller.SliceConfigReconciler{
		LeaderElector:      leaderElector,
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		Log:                controllerLog.With("name", "SliceConfig"),
		SliceConfigService: services.SliceConfigService,
		EventRecorder:      &eventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SliceConfig")
		os.Exit(1)
	}
	// initialize controller with ServiceExportConfig Kind
	if err = (&controller.ServiceExportConfigReconciler{
		LeaderElector:              leaderElector,
		Client:                     mgr.GetClient(),
		Scheme:                     mgr.GetScheme(),
		Log:                        controllerLog.With("name", "ServiceExportConfig"),
		ServiceExportConfigService: services.ServiceExportConfigService,
		EventRecorder:              &eventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ServiceExportConfig")
		os.Exit(1)
	}
	if err = (&worker.WorkerSliceGatewayReconciler{
		LeaderElector:             leaderElector,
		Client:                    mgr.GetClient(),
		Scheme:                    mgr.GetScheme(),
		Log:                       controllerLog.With("name", "WorkerSliceGateway"),
		WorkerSliceGatewayService: services.WorkerSliceGatewayService,
		EventRecorder:             &eventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "WorkerSliceGateway")
		os.Exit(1)
	}
	if err = (&worker.WorkerSliceConfigReconciler{
		LeaderElector:      leaderElector,
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		Log:                controllerLog.With("name", "WorkerSliceConfig"),
		WorkerSliceService: services.WorkerSliceConfigService,
		EventRecorder:      &eventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "WorkerSliceConfig")
		os.Exit(1)
	}
	if err = (&worker.WorkerServiceImportReconciler{
		LeaderElector:              leaderElector,
		Client:                     mgr.GetClient(),
		Scheme:                     mgr.GetScheme(),
		Log:                        controllerLog.With("name", "WorkerServiceImport"),
		WorkerServiceImportService: services.WorkerServiceImportService,
		EventRecorder:              &eventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "WorkerServiceImport")
		os.Exit(1)
	}
	if err = (&controller.SliceQoSConfigReconciler{
		LeaderElector:         leaderElector,
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		Log:                   controllerLog.With("name", "SliceQoSConfig"),
		SliceQoSConfigService: services.SliceQoSConfigService,
		EventRecorder:         &eventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SliceQoSConfig")
		os.Exit(1)
	}
	if err = (&controller.VpnKeyRotationReconciler{
		LeaderElector:         leaderElector,
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		Log:                   controllerLog.With("name", "VpnKeyRotationConfig"),
		VpnKeyRotationService: services.VpnKeyRotationService,
		EventRecorder:         &eventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "VpnKeyRotationConfig")
		os.Exit(1)
	}

	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err = (&controllerv1alpha1.Project{}).SetupWebhookWithManager(mgr, service.ValidateProjectCreate, service.ValidateProjectUpdate, service.ValidateProjectDelete); err != nil {
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
		if err = (&controllerv1alpha1.ServiceExportConfig{}).SetupWebhookWithManager(mgr, service.ValidateServiceExportConfigCreate, service.ValidateServiceExportConfigUpdate, service.ValidateServiceExportConfigDelete); err != nil {
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
		if err = (&controllerv1alpha1.SliceQoSConfig{}).SetupWebhookWithManager(mgr, service.ValidateSliceQosConfigCreate, service.ValidateSliceQosConfigUpdate, service.ValidateSliceQosConfigDelete); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "SliceQoSConfig")
			os.Exit(1)
		}
		if err = (&controllerv1alpha1.VpnKeyRotation{}).SetupWebhookWithManager(mgr, service.ValidateVpnKeyRotationCreate, service.ValidateVpnKeyRotationDelete); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "VpnKeyRotation")
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

	ctx := ctrl.SetupSignalHandler()

	// Start the HA background loop for the configured mode. Standalone starts
	// nothing (it is always the leader).
	switch haRunMode {
	case ha.ModeActive:
		go func() {
			if err := leaderElector.StartLeaseRenewal(ctx); err != nil {
				setupLog.Error(err, "HA lease renewal loop exited")
			}
		}()
	case ha.ModeStandby:
		go func() {
			if err := leaderElector.WatchRemoteLease(ctx); err != nil {
				setupLog.Error(err, "HA remote lease watch loop exited")
			}
		}()
		go func() {
			if err := remoteSyncer.Start(ctx); err != nil {
				setupLog.Error(err, "HA remote syncer exited")
			}
		}()
	}

	// Publish status.activeController for as long as this hub holds leadership
	// (ADR #293 Decision 7), so workers can identify the Active by watching both
	// hubs. Deliberately not started in standalone mode: a non-HA deployment must
	// leave the field absent, which is what keeps an existing worker's behaviour
	// unchanged. A Standby starts it too — the publisher no-ops while it is not
	// the leader, so promotion needs no extra wiring here.
	if haRunMode != ha.ModeStandalone {
		activePublisher := ha.NewActivePublisher(localHAClient, leaderElector, ha.ActivePublisherOptions{
			Endpoint:     controllerEndpoint,
			CABundlePath: haSelfCABundlePath,
			Log:          controllerLog.With("name", "ha-active-publisher"),
		})
		go func() {
			if err := activePublisher.Start(ctx); err != nil {
				setupLog.Error(err, "HA activeController publisher exited")
			}
		}()
	}

	setupLog.Info("starting manager")
	if err = mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

//All Controller RBACs goes here.

//+kubebuilder:rbac:groups=controller.kubeslice.io,resources=projects;clusters;sliceconfigs;serviceexportconfigs;sliceqosconfigs;vpnkeyrotations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=controller.kubeslice.io,resources=projects/status;clusters/status;sliceconfigs/status;serviceexportconfigs/status;sliceqosconfigs/status;vpnkeyrotations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=controller.kubeslice.io,resources=projects/finalizers;clusters/finalizers;sliceconfigs/finalizers;serviceexportconfigs/finalizers;sliceqosconfigs/finalizers;vpnkeyrotations/finalizers,verbs=update

//+kubebuilder:rbac:groups=worker.kubeslice.io,resources=workersliceconfigs;workerserviceimports;workerslicegateways;workerslicegwrecyclers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=worker.kubeslice.io,resources=workersliceconfigs/status;workerserviceimports/status;workerslicegateways/status;workerslicegwrecyclers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=worker.kubeslice.io,resources=workersliceconfigs/finalizers;workerserviceimports/finalizers;workerslicegateways/finalizers;workerslicegwrecyclers/finalizers,verbs=update

//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete;escalate
//+kubebuilder:rbac:groups="",resources=namespaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=create;get;list;watch;escalate;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;escalate;update;patch;create
//+kubebuilder:rbac:groups="batch",resources=jobs,verbs=get;list;watch;create;update;patch;delete

//+kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings;roles;clusterroles,verbs=get;list;watch;create;update;patch;delete

// NOTE: the HA leader-election Lease lives in the controller's own namespace and
// reuses the existing leader-election Role's coordination.k8s.io/leases grant
// (config/rbac/leader_election_role.yaml); no dedicated RBAC marker is needed.
