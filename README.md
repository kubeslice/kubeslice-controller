# kubeslice-controller

[![multicloud-opensource](https://github.com/kubeslice/nightly-build/actions/workflows/e2e_oss_osstest_multicloud_nightlybuild.yaml/badge.svg?branch=master)](https://github.com/kubeslice/nightly-build/actions/workflows/e2e_oss_osstest_multicloud_nightlybuild.yaml)


kubeslice-controller uses Kubebuilder, a framework for building Kubernetes APIs
using [custom resource definitions (CRDs)](https://kubernetes.io/docs/tasks/access-kubernetes-api/extend-api-custom-resource-definitions).

## Get Started

The KubeSlice Controller orchestrates the creation and management of slices on worker clusters.
It is strongly recommended that you use a released version. Follow the instructions provided in this [document](https://kubeslice.io/documentation/open-source/1.2.0/category/install-kubeslice) to install Kubeslice on cloud clusters.

## Build and Deploy a KubeSlice Controller on a Kind Cluster

See our documentation on [installing the KubeSlice Controller](https://kubeslice.io/documentation/open-source/1.2.0/install-kubeslice/yaml/yaml-controller-install).

### Prerequisites

Before you begin, make sure the following prerequisites are met:

* Docker is installed and running on your local machine.
* A running [`kind`](https://kind.sigs.k8s.io/) cluster.
* [`kubectl`](https://kubernetes.io/docs/tasks/tools/) is installed and configured.
* You have prepared the environment for the installation of [`kubeslice-controller`](https://github.com/kubeslice/kubeslice-controller) on the controller cluster and [`worker-operator`](https://github.com/kubeslice/worker-operator) on the worker cluster. For more information, see [Prerequisites](https://kubeslice.io/documentation/open-source/1.2.0/category/prerequisites).

### Set up Your Helm Repo
If you have not added avesha helm repo yet, add it.

```console
helm repo add avesha https://kubeslice.github.io/charts/
```

Upgrade the avesha helm repo.

```console
helm repo update
```

### Build Your Docker Image
To download the latest docker image for `kubeslice-controller`, click [here](https://hub.docker.com/r/aveshasystems/kubeslice-controller).

1. Clone the latest version of kubeslice-controller from  the `master` branch.

   ```console
   git clone https://github.com/kubeslice/kubeslice-controller.git
   cd kubeslice-controller
   ```

2. Edit the image name variable `IMG` in the [`Makefile`](Makefile) to change the docker tag to be built.
   The default image is set as `IMG ?= aveshasystems/kubeslice-controller:latest`. Modify this if required.

   ```console
   make docker-build
   ```

### Run Local Image on Kind Clusters

1. Load the kubeslice-controller image into your kind cluster ([kind](https://kind.sigs.k8s.io/docs/user/quick-start/#loading-an-image-into-your-cluster)).
   If needed, modify `aveshasystems/kubeslice-controller` with your locally built image name in the previous step.
   
* Note: If you use a named cluster, you must specify the name of the cluster you wish to load the images into. See [loading an image into your kind cluster](https://kind.sigs.k8s.io/docs/user/quick-start/#loading-an-image-into-your-cluster).

   ```console
   kind load docker-image aveshasystems/kubeslice-controller --name cluster-name
   ```
   Example

   ```console
   kind load docker-image aveshasystems/kubeslice-controller --name kind
   ```

2. Check the loaded image in the cluster. Modify the node name if required.
   
 * Note: `kind-control-plane` is the name of the Docker container. Modify the name if needed.
  

```console
docker exec -it kind-control-plane crictl images
```
### Deploy the KubeSlice Controller on a Cluster
1. Create a chart values file called `yourvaluesfile.yaml`. Refer to [values.yaml](https://github.com/kubeslice/charts/blob/master/charts/kubeslice-controller/values.yaml) to update the `kubeslice-controller` image to the local build image.

   From the sample:

   ```
   kubeslice:
   ---
   ---
      controller:
      ---
      ---
         image: aveshasystems/kubeslice-controller
         tag: 0.1.1
   ```

  Change it to:

   ```
   kubeslice:
   ---
   ---
      controller:
      ---
      ---
         image: <my-custom-image> 
         tag: <unique-tag>
   ```

2. Deploy the updated chart.

   ```console
   make chart-deploy VALUESFILE=yourvaluesfile.yaml
   ```
   
### Verify the Installation

Verify the installation of the KubeSlice Controller by checking the pods belonging to the `kubeslice-controller` namespace using the following command:

```console
kubectl get pods -n kubeslice-controller
```

Example Output

```
NAME                                            READY   STATUS    RESTARTS   AGE
kubeslice-controller-manager-5b548fb865-kzb7c   2/2     Running   0          102s
```

### Uninstall the KubeSlice Controller
For more information, see [uninstall KubeSlice](https://kubeslice.io/documentation/open-source/1.2.0/uninstall-kubeslice/).

```console
make chart-undeploy
```

## License

Apache License 2.0
