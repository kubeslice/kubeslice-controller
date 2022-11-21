# kubeslice-controller

kubeslice-controller uses Kubebuilder, a framework for building Kubernetes APIs
using [custom resource definitions (CRDs)](https://kubernetes.io/docs/tasks/access-kubernetes-api/extend-api-custom-resource-definitions).

[![multicloud-opensource](https://github.com/kubeslice/nightly-build/actions/workflows/multicloud-opensource.yaml/badge.svg?branch=master)](https://github.com/kubeslice/nightly-build/actions/workflows/multicloud-opensource.yaml)
[![kind-opensource](https://github.com/kubeslice/nightly-build/actions/workflows/kind-opensource.yaml/badge.svg?branch=master)](https://github.com/kubeslice/nightly-build/actions/workflows/kind-opensource.yaml) 
[![multicloud-opensource-uat](https://github.com/kubeslice/nightly-build/actions/workflows/multicloud-oss-uat.yml/badge.svg)](https://github.com/kubeslice/nightly-build/actions/workflows/multicloud-oss-uat.yml) 
[![kind-opensource-uat](https://github.com/kubeslice/nightly-build/actions/workflows/kind-oss-uat.yml/badge.svg)](https://github.com/kubeslice/nightly-build/actions/workflows/kind-oss-uat.yml) 

## Getting Started

The KubeSlice Controller orchestrates the creation and management of slices on worker clusters.
It is strongly recommended to use a released version. Follow the instructions provided in this [document](https://docs.avesha.io/documentation/open-source/0.2.0/getting-started-with-cloud-clusters/installing-kubeslice/installing-the-kubeslice-controller).

## Building and Deploying `kubeslice-controller` in a Local Kind Cluster
For more information, see [getting started with kind clusters](https://docs.avesha.io/documentation/open-source/0.2.0/getting-started-with-kind-clusters).

### Prerequisites

* Docker installed and running in your local machine
* A running [`kind`](https://kind.sigs.k8s.io/)  cluster
* [`kubectl`](https://kubernetes.io/docs/tasks/tools/) installed and configured
* Follow the getting started from above to install [`kubeslice-controller`](https://github.com/kubeslice/kubeslice-controller) and [`worker-operator`](https://github.com/kubeslice/worker-operator).

### Setting up Your Helm Repo
If you have not added avesha helm repo yet, add it.

```console
helm repo add avesha https://kubeslice.github.io/charts/
```

Upgrade the avesha helm repo.

```console
helm repo update
```

### Build Your Docker Image
#### Latest docker image - [kubeslice-controller](https://hub.docker.com/r/aveshasystems/kubeslice-controller)

1. Clone the latest version of kubeslice-controller from  the `master` branch.

```console
git clone https://github.com/kubeslice/kubeslice-controller.git
cd kubeslice-controller
```

2. Adjust image name variable `IMG` in the [`Makefile`](Makefile) to change the docker tag to be built.
   Default image is set as `IMG ?= aveshasystems/kubeslice-controller:latest`. Modify this if required.

```console
make docker-build
```
### Running Local Image on Kind Clusters

1. Loading kubeslice-controller image into your kind cluster ([kind](https://kind.sigs.k8s.io/docs/user/quick-start/#loading-an-image-into-your-cluster)).
   If needed, replace `aveshasystems/kubeslice-controller` with your locally built image name in the previous step.
   
* Note: If you use a named cluster, you must specify the name of the cluster you wish to load the images into. See [loading an image into your kind cluster](https://kind.sigs.k8s.io/docs/user/quick-start/#loading-an-image-into-your-cluster).

```console
kind load docker-image aveshasystems/kubeslice-controller --name cluster-name
```
Example

```console
kind load docker-image aveshasystems/kubeslice-controller --name kind
```

2. Check the loaded image in the cluster. Modify node name if required.

* Note: `kind-control-plane` is the name of the Docker container. Modify if needed.

```console
docker exec -it kind-control-plane crictl images
```
### Deploying in a Cluster
1. Create chart values file `yourvaluesfile.yaml`. Refer to [values.yaml](https://github.com/kubeslice/charts/blob/master/charts/kubeslice-controller/values.yaml) on how to adjust this and update the `kubeslice-controller` image to the local build image.

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
````

2. Deploy the updated chart.

   ```console
   make chart-deploy VALUESFILE=yourvaluesfile.yaml
   ```
   
### Verify if the Operator is Running


```console
kubectl get pods -n kubeslice-controller
```

Sample output to expect

```
NAME                                            READY   STATUS    RESTARTS   AGE
kubeslice-controller-manager-5b548fb865-kzb7c   2/2     Running   0          102s
```

### Uninstalling the kubeslice-controller
For more information, see [uninstalling KubeSlice](https://docs.avesha.io/documentation/open-source/0.2.0/getting-started-with-cloud-clusters/uninstalling-kubeslice).

```console
make chart-undeploy
 ```

## License

Apache License 2.0
