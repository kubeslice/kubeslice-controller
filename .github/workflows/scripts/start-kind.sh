#!/bin/bash

# Create controller kind cluster if not present
if [ ! $(kind get clusters | grep controller) ];then
  kind create cluster --name controller --config .github/workflows/scripts/cluster.yaml --image kindest/node:v1.22.7
  ip=$(docker inspect controller-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress') 
#  echo $ip
# Replace loopback IP with docker ip
  kind get kubeconfig --name controller | sed "s/127.0.0.1.*/$ip:6443/g" > /home/runner/.kube/kind1.yaml
fi

# Create worker1 kind cluster if not present
# Create worker1 kind cluster if not present
if [ ! $(kind get clusters | grep worker) ];then
  kind create cluster --name worker --config .github/workflows/scripts/cluster.yaml --image kindest/node:v1.22.7
  ip=$(docker inspect worker-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress')
#  echo $ip
# Replace loopback IP with docker ip
  kind get kubeconfig --name worker | sed "s/127.0.0.1.*/$ip:6443/g" > /home/runner/.kube/kind2.yaml
fi

KUBECONFIG=/home/runner/.kube/kind1.yaml:/home/runner/.kube/kind2.yaml kubectl config view --raw  > /home/runner/.kube/kinde2e.yaml

if [ ! -f profile/kind.yaml ];then
  # Provide correct IP in kind profile, since worker operator cannot detect internal IP as nodeIp
  IP1=$(docker inspect controller-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress')
  IP2=$(docker inspect worker-control-plane | jq -r '.[0].NetworkSettings.Networks.kind.IPAddress')

echo ${userent}

  cat > profile/kind.yaml << EOF
Kubeconfig: kinde2e.yaml
WorkerClusters:
- Context: kind-controller
  NodeIP: ${IP1}
- Context: kind-worker
  NodeIP: ${IP2}
ControllerCluster:
  Context: kind-controller
  HubChartOptions:
    Repo: https://raw.githubusercontent.com/kubeslice/dev-charts/gh-pages/
    Username: ${userent}
    Password: ${passent}
    SetStrValues:
       "kubeslice.controller.image": "kubeslice-controller"
       "kubeslice.controller.tag": "e2e-latest"
WorkerChartOptions:
  Repo: https://raw.githubusercontent.com/kubeslice/dev-charts/gh-pages/
  Username: ${userent}
  Password: ${passent}
TestSuitesEnabled:
   HubSuite: true
   WorkerSuite: true
EOF

fi
chmod  600  /home/runner/.kube/kinde2e.yaml
chmod  600  /home/runner/.kube/kind2.yaml
chmod  600  /home/runner/.kube/kind1.yaml
