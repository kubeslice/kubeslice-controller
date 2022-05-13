# kubeslice-controller

![Docker Image Size](https://img.shields.io/docker/image-size/aveshasystems/kubeslice-controller/latest)
![DockerImageVersion](https://img.shields.io/docker/v/aveshasystems/kubeslice-controller?sort=date)

kubeslice-controller uses Kubebuilder, a framework for building Kubernetes APIs
using [custom resource definitions (CRDs)](https://kubernetes.io/docs/tasks/access-kubernetes-api/extend-api-custom-resource-definitions).

## Getting Started

The KubeSlice Controller orchestrates the creation and management of slices on worker clusters.
It is strongly recommended to use a released version. Follow the instructions provided in this [document](https://docs.avesha.io/opensource/installing-the-kubeslice-controller).

## Building and Installing `kubeslice-controller` in a Local Kind Cluster
For more information, see [getting started with kind clusters](https://docs.avesha.io/opensource/getting-started-with-kind-clusters).

### Prerequisites

* Docker installed and running in your local machine
* A running [`kind`](https://kind.sigs.k8s.io/) or [`Docker Desktop Kubernetes`](https://docs.docker.com/desktop/kubernetes/)
  cluster
* [`kubectl`](https://kubernetes.io/docs/tasks/tools/) installed and configured

### Build docker images

1. Clone the latest version of kubeslice-controller from  the `master` branch.

```bash
git clone https://github.com/kubeslice/kubeslice-controller.git
cd kubeslice-controller
```

2. Adjust image name variable `IMG` in the [`Makefile`](Makefile) to change the docker tag to be built.
   Default image is set as `IMG ?= aveshasystems/kubeslice-controller:latest`. Modify this if required.

```bash
make docker-build
```

3. Loading kubeslice-controller Image Into Your Kind Cluster ([`link`](https://kind.sigs.k8s.io/docs/user/quick-start/#loading-an-image-into-your-cluster))
   If needed, replace `aveshasystems/kubeslice-controller` with your locally built image name in the previous step.

```bash
kind load docker-image aveshasystems/kubeslice-controller
```
### Installation
To install:

1. Create a self-signed certificate for the webhook server.

```bash
make webhookCA
```

or

```bash
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.7.0/cert-manager.yaml
```

2. First check all the cert-manager pods are up and running then run the following command to deploy `kubeslice-controller` to the kind cluster with all the CRDs:

```bash
make deploy
```

3. For checking the logs of the pods, run the following command: (pod-name would start with `kubeslice-controller-manager-`)

```bash
kubectl logs -f {pod-name} -n kubeslice-controller
```

### Installing Sample Manifests

* We have some sample manifests yaml file under `/config/sample`.
* Run the following commands:

#### For creating a project
```bash
kubectl apply -f config/samples/controller_v1alpha1_project.yaml  
 ```

#### Registering the Worker Cluster
```bash
kubectl apply -f config/samples/controller_v1alpha1_cluster.yaml -n=kubeslice-cisco
```
#### Applying the sliceconfig
```bash
kubectl apply -f config/samples/controller_v1alpha1_sliceconfig.yaml -n=kubeslice-cisco
```

### Running unit-test cases
After running this command it will generate a report under `coverage-report/report.html`
open this on your browser for the coverage report
```bash
make unit-test
```

### Uninstalling the kubeslice-controller
```bash
# delete all the projects
kubectl delete project --all
```

```bash
# uninstall all the resources
make undeploy
```

## License

Apache License 2.0
