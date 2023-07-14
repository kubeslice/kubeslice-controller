#!/bin/bash

# Create controller kind cluster if not present
if [ ! $(kind get clusters | grep controller) ];then
  kind create cluster --name controller --config .github/workflows/scripts/cluster.yaml --image kindest/node:v1.24.15
  ip=$(docker inspect controller-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress') 
  #  echo $ip
  # loading docker image into kind controller
  kind load docker-image kubeslice-controller:${GITHUB_HEAD_COMMIT} --name controller
  # Replace loopback IP with docker ip
  kind get kubeconfig --name controller | sed "s/127.0.0.1.*/$ip:6443/g" > /home/runner/.kube/kind1.yaml
fi

# Create worker1 kind cluster if not present
if [ ! $(kind get clusters | grep worker) ];then
  kind create cluster --name worker --config .github/workflows/scripts/cluster.yaml --image kindest/node:v1.24.15
  ip=$(docker inspect worker-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress')
  #  echo $ip
  # loading docker image into kind controller
  kind load docker-image kubeslice-controller:${GITHUB_HEAD_COMMIT} --name worker
  # Replace loopback IP with docker ip
  kind get kubeconfig --name worker | sed "s/127.0.0.1.*/$ip:6443/g" > /home/runner/.kube/kind2.yaml
fi

KUBECONFIG=/home/runner/.kube/kind1.yaml:/home/runner/.kube/kind2.yaml kubectl config view --raw  > /home/runner/.kube/kinde2e.yaml

if [ ! -f profile/kind.yaml ];then
  # Provide correct IP in kind profile, since worker operator cannot detect internal IP as nodeIp
  IP1=$(docker inspect controller-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress')
  IP2=$(docker inspect worker-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress')

  cat > profile/kind.yaml << EOF
Kubeconfig: kinde2e.yaml
ControllerCluster:
  Context: kind-controller
  CertManagerOptions:
    Release: cert-manager
    Chart: cert-manager
    Repo: "https://raw.githubusercontent.com/kubeslice/dev-charts/gh-pages/"
    Namespace: cert-manager
    Version: v1.7.0
    Username: ${chartuser}
    Password: ${chartpassword}
  HubChartOptions:
    Release: kubeslice-controller
    Chart: kubeslice-controller
    Repo: "https://raw.githubusercontent.com/kubeslice/dev-charts/gh-pages/"
    Namespace: kubeslice-controller
    Username: ${chartuser}
    Password: ${chartpassword}
    SetStrValues:
      "kubeslice.controller.image": "kubeslice-controller"
      "kubeslice.controller.tag": "${GITHUB_HEAD_COMMIT}"
WorkerClusters:
- Context: kind-controller
  NodeIP: ${IP1}
- Context: kind-worker
  NodeIP: ${IP2}
WorkerChartOptions:
  Release: kubeslice-worker
  Chart: kubeslice-worker
  Repo: "https://raw.githubusercontent.com/kubeslice/dev-charts/gh-pages/"
  Namespace: kubeslice-system
  Username: ${chartuser}
  Password: ${chartpassword}
IstioBaseChartOptions:
  Release:   "istio-base"
  Chart:     "istio-base"
  Repo:      "https://raw.githubusercontent.com/kubeslice/dev-charts/gh-pages/"
  Username: ${chartuser}
  Password: ${chartpassword}
  Namespace: "istio-system"
IstioDChartOptions:
  Release:   "istiod"
  Chart:     "istio-discovery"
  Repo:      "https://raw.githubusercontent.com/kubeslice/dev-charts/gh-pages/"
  Username: ${chartuser}
  Password: ${chartpassword}
  Namespace: "istio-system"
TestSuitesEnabled:
  EmptySuite: false
  HubSuite: true
  WorkerSuite: false
  IstioSuite: false
  
EOF

fi
