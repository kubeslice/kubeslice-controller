package service

import (
	"context"
	"fmt"
	"os"
	"time"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ICleanupService interface {
	CleanupResources(ctx context.Context)
}

var (
	controllerManagerNamespace string
	noOfRetries                = 3
	sleepDuration              = 1 * time.Second
	// Kubeslice resources
	projects             *controllerv1alpha1.ProjectList
	sliceConfigs         *controllerv1alpha1.SliceConfigList
	serviceExportConfigs *controllerv1alpha1.ServiceExportConfigList
	sliceQosConfigs      *controllerv1alpha1.SliceQoSConfigList
	clusters             *controllerv1alpha1.ClusterList
	workerSliceConfigs   *workerv1alpha1.WorkerSliceConfigList
	workerServiceImports *workerv1alpha1.WorkerServiceImportList
	logger               = util.NewLogger().With("controller", "GracefulCleanup")
)

type CleanupService struct {
}

func (cs *CleanupService) CleanupResources(ctx context.Context) {
	controllerManagerNamespace = os.Getenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE")
	hasErrors := false

	// Initialize Resource Types
	projects = &controllerv1alpha1.ProjectList{}
	sliceConfigs = &controllerv1alpha1.SliceConfigList{}
	serviceExportConfigs = &controllerv1alpha1.ServiceExportConfigList{}
	clusters = &controllerv1alpha1.ClusterList{}
	sliceQosConfigs = &controllerv1alpha1.SliceQoSConfigList{}
	workerSliceConfigs = &workerv1alpha1.WorkerSliceConfigList{}
	workerServiceImports = &workerv1alpha1.WorkerServiceImportList{}

	// Delete all Projects
	logger.Infof("%s Fetching all Projects", util.Find)
	err := util.ListResources(ctx, projects, client.InNamespace(controllerManagerNamespace))
	if err != nil {
		logger.Errorf("%s Unable to fetch Projects. Returning from cleanup. %s", util.Err, err.Error())
		return
	}
	// Returning if no Projects found
	if len(projects.Items) == 0 {
		logger.Infof("%s Found 0 Projects. Returning from cleanup", util.Find)
		return
	}

	for _, project := range projects.Items {

		projectNamespace := project.Labels["kubeslice-project-namespace"]
		// Delete all ServiceExports
		logger.Infof("%s Fetching all ServiceExports for Project %s", util.Find, project.GetName())

		err := util.ListResources(ctx, serviceExportConfigs, client.InNamespace(projectNamespace))
		if err != nil {
			hasErrors = true
			logger.Errorf("%sError fetching ServiceExports. %s", util.Err, err.Error())
		}
		for _, serviceExportConfig := range serviceExportConfigs.Items {
			logger.Infof("%s  Deleting ServiceExportConfig %s", util.Bin, serviceExportConfig.GetName())
			err := util.CleanupDeleteResource(ctx, &serviceExportConfig)
			if err != nil {
				hasErrors = true
				logger.Errorf("%s Error deleting ServiceExport %s. %s", util.Err, serviceExportConfig.GetName(), err.Error())
			}
		}
		// Delete all sliceConfigs
		logger.Infof("%s Fetching all SliceConfigs for Project %s", util.Find, project.GetName())
		err = util.ListResources(ctx, sliceConfigs, client.InNamespace(projectNamespace))
		if err != nil {
			hasErrors = true
			logger.Errorf("%s Error fetching SliceConfigs. %s", util.Err, err.Error())
		}
		for _, sliceConfig := range sliceConfigs.Items {
			logger.Infof("%s  Removing ApplicationNamespaces from SliceConfig %s", util.Bin, sliceConfig.GetName())

			// Offboard Application Namespaces
			if len(sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces) > 0 {
				sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces = nil
				err := util.CleanupUpdateResource(ctx, &sliceConfig)
				if err != nil {
					hasErrors = true
					logger.Errorf("%s Error offboarding Application Namespaces from SliceConfig %s. %s", util.Err, sliceConfig.GetName(), err.Error())
				}
			}
			// Fetch all WorkerSliceConfigs to check the status of Namespace offboarding
			completeResourceName := fmt.Sprintf("%s-%s", util.GetObjectKind(&sliceConfig), sliceConfig.GetName())
			ownershipLabel := util.GetOwnerLabel(completeResourceName)
			err := util.ListResources(ctx, workerSliceConfigs, client.MatchingLabels(ownershipLabel), client.InNamespace(projectNamespace))
			if err != nil {
				hasErrors = true
				logger.Errorf("%s Error fetching WorkerSliceConfigs %s", util.Err, err)
			}
			for _, workerSliceConfig := range workerSliceConfigs.Items {
				// checking status of workerSliceConfig
				logger.Infof("%s Checking status of workerSliceConfig %s", util.Find, workerSliceConfig.Name)
				// add exponential delay for this check and exit after timeout.
				err := util.Retry(ctx, (noOfRetries + 3), sleepDuration, func() (err error) {
					util.GetResourceIfExist(ctx, client.ObjectKey{
						Namespace: workerSliceConfig.Namespace,
						Name:      workerSliceConfig.Name,
					}, &workerSliceConfig)
					if len(workerSliceConfig.Status.OnboardedAppNamespaces) == 0 {
						return nil
					}
					return fmt.Errorf("application Namespaces not offboarded")
				})
				if err != nil {
					hasErrors = true
					logger.Infof("%s WorkerSliceConfig status not updated %s", util.Sad, err.Error())
					// After timeout update status of workerSliceConfig
					workerSliceConfig.Status.OnboardedAppNamespaces = nil
					err = util.CleanupUpdateStatus(ctx, &workerSliceConfig)
					if err != nil {
						hasErrors = true
						logger.Errorf("%s Error updating status of WorkerSliceConfig %s. %s", util.Err, workerSliceConfig.GetName(), err.Error())
					}
				}
			}

			// Delete the SliceConfig
			logger.Infof("%s  Deleting SliceConfig %s", util.Bin, sliceConfig.GetName())
			err = util.Retry(ctx, noOfRetries, sleepDuration, func() (err error) {
				return util.CleanupDeleteResource(ctx, &sliceConfig)
			})
			if err != nil {
				hasErrors = true
				logger.Errorf("%s Error deleting SliceConfig %s. %s", util.Err, sliceConfig.GetName(), err.Error())
			}
		}
		// Delete all sliceQosConfigs
		logger.Infof("%s Fetching all SliceQosConfigs for Project %s", util.Find, project.GetName())
		err = util.ListResources(ctx, sliceQosConfigs, client.InNamespace(projectNamespace))
		if err != nil {
			hasErrors = true
			logger.Error("%s Error fetching sliceQosConfigs %s", util.Err, err.Error())
		}
		for _, sliceQosConfig := range sliceQosConfigs.Items {
			logger.Infof("%s  Deleting SliceQosConfigs %s", util.Bin, sliceQosConfig.GetName())
			err = util.Retry(ctx, noOfRetries, sleepDuration, func() (err error) {
				return util.CleanupDeleteResource(ctx, &sliceQosConfig)
			})
			if err != nil {
				hasErrors = true
				logger.Errorf("%s Error deleting SliceQosConfigs %s. %s", util.Err, sliceQosConfig.GetName(), err.Error())
			}
		}

		// Delete all WorkerServiceImports (if Remaining)
		logger.Infof("%s Fetching all WorkerServiceImports for Project %s", util.Find, project.GetName())
		err = util.ListResources(ctx, workerServiceImports, client.InNamespace(projectNamespace))
		if err != nil {
			hasErrors = true
			logger.Error("%s Error fetching WorkerServiceImports %s", util.Err, err.Error())
		}
		for _, cluster := range workerServiceImports.Items {
			logger.Infof("%s  Deleting WorkerServiceImport %s", util.Bin, cluster.GetName())
			err = util.Retry(ctx, noOfRetries, sleepDuration, func() (err error) {
				return util.CleanupDeleteResource(ctx, &cluster)
			})
			if err != nil {
				hasErrors = true
				logger.Errorf("%s Error deleting workerServiceImport %s. %s", util.Err, cluster.GetName(), err.Error())
			}
		}

		// Delete all clusters
		logger.Infof("%s Fetching all Clusters for Project %s", util.Find, project.GetName())
		err = util.ListResources(ctx, clusters, client.InNamespace(projectNamespace))
		if err != nil {
			hasErrors = true
			logger.Error("%s Error fetching Clusters %s", util.Err, err.Error())
		}
		for _, cluster := range clusters.Items {
			logger.Infof("%s  Deleting Cluster %s", util.Bin, cluster.GetName())
			err = util.Retry(ctx, noOfRetries, sleepDuration, func() (err error) {
				return util.CleanupDeleteResource(ctx, &cluster)
			})
			if err != nil {
				hasErrors = true
				logger.Errorf("%s Error deleting Cluster %s. %s", util.Err, cluster.GetName(), err.Error())
			}
		}

		// Delete the project
		logger.Infof("%s  Deleting Project %s", util.Bin, project.GetName())
		err = util.Retry(ctx, noOfRetries, sleepDuration, func() (err error) {
			return util.CleanupDeleteResource(ctx, &project)
		})
		if err != nil {
			hasErrors = true
			logger.Errorf("%s Error deleting Project %s. %s", util.Err, project.GetName(), err.Error())
		}
	}
	if !hasErrors {
		logger.Infof("%s Successfully cleaned up all Kubeslice resources.", util.Party)
	} else {
		logger.Infof("%s Cleanup job finished with errors.", util.Sad)
	}
}
