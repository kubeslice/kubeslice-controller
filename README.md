# kubeslice-controller

kubeslice-controller uses Kubebuilder, a framework for building Kubernetes APIs
using [custom resource definitions (CRDs)](https://kubernetes.io/docs/tasks/access-kubernetes-api/extend-api-custom-resource-definitions).

## Getting Started

It is strongly recommended to use a released version.

## Installing `kubeslice-controller` in local kind cluster

### Prerequisites

* Docker installed and running in your local machine
* A running [`kind`](https://kind.sigs.k8s.io/) or [`Docker Desktop Kubernetes`](https://docs.docker.com/desktop/kubernetes/)
  cluster
* [`kubectl`](https://kubernetes.io/docs/tasks/tools/) installed and configured

### Installation
To install:

1. Clone the latest version of kubeslice-controller from  the `master` branch.

```bash
  git clone https://github.com/kubeslice/kubeslice-controller.git
  cd kubeslice-controller
```

2. Create a self-signed certificate for the webhook server.

   ```bash
   make webhookCA
   ```

   or

   ```bash
   kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.7.0/cert-manager.yaml
   ```

3. Run the following command to deploy kubeslice-controller to the kind cluster with all the CRDs:

   ```bash
   make deploy
   ```

4. For checking the logs of the pods, run the following command:

   ```bash
   kubectl logs -f {pod-name} -n kubeslice-controller
   ```

### Installing Sample Manifests

* We have some sample manifests yaml file under `/connfig/sample`.
* Run the following commands:

   ```bash
   # for creating a project 
   kubectl apply -f config/samples/controller_v1alpha1_project.yaml  

# Registering the Worker Cluster
kubectl apply -f config/samples/hub_v1alpha1_cluster.yaml -n=kubeslice-cisco

# Applying the sliceconfig
```
kubectl apply -f config/samples/hub_v1alpha1_sliceconfig.yaml -n=kubeslice-cisco
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
