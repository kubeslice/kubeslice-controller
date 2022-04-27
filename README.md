# kubeslice-controller

kubeslice-controller uses Kubebuilder, a framework for building Kubernetes APIs
using [custom resource definitions (CRDs)](https://kubernetes.io/docs/tasks/access-kubernetes-api/extend-api-custom-resource-definitions)
.

## Getting Started

It is strongly recommended that you use a released version.

## Install `kubeslice-controller` in local kind cluster

### Prerequisites

* Docker installed and running in your local machine
* [`kind`](https://kind.sigs.k8s.io/) or [`Docker Desktop Kubernetes`](https://docs.docker.com/desktop/kubernetes/)
  cluster running
* [`kubectl`](https://kubernetes.io/docs/tasks/tools/) installed and configured

### Installation

* clone the latest version of kubeslice-controller from branch `master`

```bash
  git clone https://github.com/kubeslice/kubeslice-controller.git
  cd kubeslice-controller
```

* create a self-signed certificate for the webhook server

```bash
make webhookCA
```

or

```bash
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.7.0/cert-manager.yaml
```

* Run the following command to deploy kubeslice-controller to the kind cluster with all the CRDs

```bash
make deploy
```

* for checking the logs of the pods run the following command

```bash
 kubectl logs -f {pod-name} -n kubeslice-controller
```

### Install sample manifests

* we have some sample manifets yaml file under `/connfig/sample`
* run the following commands

```bash
# for create a project 
kubectl apply -f config/samples/controller_v1alpha1_project.yaml  

# register the cluster
kubectl apply -f config/samples/hub_v1alpha1_cluster.yaml -n=kubeslice-cisco

# apply the sliceconfig
kubectl apply -f config/samples/hub_v1alpha1_sliceconfig.yaml -n=kubeslice-cisco
```

## License

Apache License 2.0
