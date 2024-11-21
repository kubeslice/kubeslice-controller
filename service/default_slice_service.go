package service

import (
	"context"
	"fmt"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func DefaultSliceOperations(ctx context.Context, req ctrl.Request, logger *zap.SugaredLogger, cluster *controllerv1alpha1.Cluster) (ctrl.Result, error) {
	// Step 7: create default slice or copy namespaces to default slice
	projectName := util.GetProjectName(req.Namespace)
	project := &controllerv1alpha1.Project{}
	present, err := util.GetResourceIfExist(ctx, types.NamespacedName{
		Name:      projectName,
		Namespace: ControllerNamespace,
	}, project)
	if err != nil {
		logger.Errorf("error while getting project %v", projectName)
		return ctrl.Result{}, err
	}

	defaultSliceName := fmt.Sprintf(util.DefaultProjectSliceName, projectName)

	// also check for defaultSliceCreation flag
	if present && project.Spec.DefaultSliceCreation {
		// create default slice if not present
		defaultProjectSlice := &controllerv1alpha1.SliceConfig{}
		defaultSliceNamespacedName := types.NamespacedName{
			Namespace: req.Namespace,
			Name:      defaultSliceName,
		}
		foundDefaultSlice, err := util.GetResourceIfExist(ctx, defaultSliceNamespacedName, defaultProjectSlice)
		if err != nil {
			logger.Errorf("error while getting default slice %v", defaultSliceName)
			return ctrl.Result{}, err
		}
		// if not found, create with all namespace of cluster
		if !foundDefaultSlice {
			appns := []controllerv1alpha1.SliceNamespaceSelection{}
			for _, ns := range cluster.Status.Namespaces {
				if ns.SliceName == "" {
					appns = append(appns, controllerv1alpha1.SliceNamespaceSelection{
						Namespace: ns.Name,
						Clusters:  []string{cluster.Name},
					})
				}
			}
			logger.Infof("appns %v", appns)
			defaultProjectSlice = &controllerv1alpha1.SliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      defaultSliceName,
					Namespace: req.Namespace,
				},
				Spec: controllerv1alpha1.SliceConfigSpec{
					OverlayNetworkDeploymentMode: v1alpha1.NONET,
					Clusters:                     []string{req.Name},
					NamespaceIsolationProfile: controllerv1alpha1.NamespaceIsolationProfile{
						ApplicationNamespaces: appns,
					},
					MaxClusters: 16,
				},
			}
			err := util.CreateResource(ctx, defaultProjectSlice)
			if err != nil {
				return ctrl.Result{}, err
			}
			logger.Infof("successfully created default slice %s", defaultSliceName)
		} else {

			logger.Infof("default slice %s already present %v", defaultSliceName, defaultProjectSlice)
			// if default slice is already present, either the cluster is new or there is some change in cluster
			// check if cluster is already registered
			isNewCluster := true
			for _, cluster := range defaultProjectSlice.Spec.Clusters {
				if cluster == req.Name {
					isNewCluster = false
					break
				}
			}

			if isNewCluster {
				defaultProjectSlice.Spec.Clusters = append(defaultProjectSlice.Spec.Clusters, req.Name)
			}

			// create map for easy look up
			namespaceToClusterMap := make(map[string]struct{})
			mapKeyFormat := "namespace=%s&cluster=%s"
			namespaceIndexMap := make(map[string]int)

			for index, appns := range defaultProjectSlice.Spec.NamespaceIsolationProfile.ApplicationNamespaces {
				namespaceIndexMap[appns.Namespace] = index
				for _, cluster := range appns.Clusters {
					mapKey := fmt.Sprintf(mapKeyFormat, appns.Namespace, cluster)
					namespaceToClusterMap[mapKey] = struct{}{}
				}
			}

			// if any namespace is added to cluster, add it to default slice
			isNamespaceAddedToCluster := false
			for _, ns := range cluster.Status.Namespaces {
				if ns.SliceName != "" {
					continue
				}
				mapKey := fmt.Sprintf(mapKeyFormat, ns.Name, cluster.Name)
				if _, ok := namespaceToClusterMap[mapKey]; ok {
					logger.Info("already present namespace and cluster, skipping operations...")
				} else {
					isNamespaceAddedToCluster = true
					// check if namespace is already present
					if nsIndex, ok := namespaceIndexMap[ns.Name]; ok {
						// not handling same namespace in multiple cluster for now
						prevData := defaultProjectSlice.Spec.NamespaceIsolationProfile.ApplicationNamespaces[nsIndex]
						modifiedData := controllerv1alpha1.SliceNamespaceSelection{
							Namespace: prevData.Namespace,
							Clusters:  append(prevData.Clusters, cluster.Name),
						}
						defaultProjectSlice.Spec.NamespaceIsolationProfile.ApplicationNamespaces[nsIndex] = modifiedData
					} else {
						// if namespace is not present, add another entry
						defaultProjectSlice.Spec.NamespaceIsolationProfile.ApplicationNamespaces = append(defaultProjectSlice.Spec.NamespaceIsolationProfile.ApplicationNamespaces, controllerv1alpha1.SliceNamespaceSelection{
							Namespace: ns.Name,
							Clusters:  []string{cluster.Name},
						})
					}
				}
			}
			if isNamespaceAddedToCluster {
				logger.Infof("handling ns addition cluster %v slice %v", cluster.Name, defaultProjectSlice)
				err := util.UpdateResource(ctx, defaultProjectSlice)
				if err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}

			// if any namespace is removed from cluster that is still present in default slice, remove it
			isNamespaceRemovedFromCluster := false
			namespacesInCluster := make(map[string]struct{})
			for _, ns := range cluster.Status.Namespaces {
				if ns.SliceName == defaultSliceName {
					namespacesInCluster[ns.Name] = struct{}{}
				}
			}

			modifiedDefaultSliceApplicationNamespace := []controllerv1alpha1.SliceNamespaceSelection{}
			for _, appns := range defaultProjectSlice.Spec.NamespaceIsolationProfile.ApplicationNamespaces {
				foundClusterName := false
				for _, clusterName := range appns.Clusters {
					if clusterName == req.Name {
						foundClusterName = true
						if _, ok := namespacesInCluster[appns.Namespace]; !ok {
							isNamespaceRemovedFromCluster = true
							if len(appns.Clusters) > 1 {
								appns.Clusters = util.RemoveElementFromArray(appns.Clusters, cluster.Name)
								modifiedDefaultSliceApplicationNamespace = append(modifiedDefaultSliceApplicationNamespace, appns)
							} else {
								continue
							}
						} else {
							modifiedDefaultSliceApplicationNamespace = append(modifiedDefaultSliceApplicationNamespace, appns)
						}
					}
				}
				if !foundClusterName {
					modifiedDefaultSliceApplicationNamespace = append(modifiedDefaultSliceApplicationNamespace, appns)
				}
			}

			logger.Info("before update appns %v ", defaultProjectSlice.Spec.NamespaceIsolationProfile.ApplicationNamespaces)

			defaultProjectSlice.Spec.NamespaceIsolationProfile.ApplicationNamespaces = modifiedDefaultSliceApplicationNamespace

			logger.Info("after update appns %v ", defaultProjectSlice.Spec.NamespaceIsolationProfile.ApplicationNamespaces)
			if isNamespaceRemovedFromCluster && !isNamespaceAddedToCluster {
				logger.Infof("handling ns removal from cluster %v and slice %s", cluster.Name, defaultProjectSlice)
				err := util.UpdateResource(ctx, defaultProjectSlice)
				if err != nil {
					return ctrl.Result{}, err
				}

				return ctrl.Result{}, nil
			}
		}

	}
	return ctrl.Result{}, nil
}
