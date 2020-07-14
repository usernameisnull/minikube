#!/bin/bash
## 在kubeadm和kubectl运行成功后,执行该脚本
baseDir='/root/.minikube/cache/linux/v1.16.2'
kubectlPath="${baseDir}/kubectl"
kubeconfigPath="${baseDir}/kubeconfig"
${kubectlPath} create clusterrolebinding minikube-rbac --clusterrole=cluster-admin --serviceaccount=kube-system:default --kubeconfig=${kubeconfigPath}

${kubectlPath} label nodes minikube.k8s.io/version=v1.11.0 minikube.k8s.io/commit=fe23bcb6ab168115e7ee4dd03e2b6bbbe5225bd1-dirty minikube.k8s.io/name=minikube minikube.k8s.io/updated_at=2020_07_14T15_11_22_0700 --all --overwrite --kubeconfig=${kubeconfigPath}

KUBECONFIG=${kubeconfigPath} ${kubectlPath} apply -f /etc/kubernetes/addons/storage-provisioner.yaml

KUBECONFIG=${kubeconfigPath} ${kubectlPath} apply -f /etc/kubernetes/addons/storageclass.yaml
