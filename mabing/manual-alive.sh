#!/bin/bash
# 在minikube集群还没delete的时候执行
baseDir='/root/.minikube/cache/linux/v1.16.2'
crtsDir="${baseDir}/crts/"
mkdir -p ${crtsDir}
cp /var/lib/kubelet/config.yaml ${crtsDir}
cp /root/.minikube/profiles/minikube/apiserver.crt  ${crtsDir}
cp /root/.minikube/profiles/minikube/apiserver.key  ${crtsDir} 
cp /root/.minikube/profiles/minikube/proxy-client.crt  ${crtsDir}
cp /root/.minikube/profiles/minikube/proxy-client.key  ${crtsDir}
cp /root/.minikube/ca.crt ${crtsDir}
cp /root/.minikube/ca.key ${crtsDir}
cp /root/.minikube/proxy-client-ca.crt ${crtsDir}
cp /root/.minikube/proxy-client-ca.key ${crtsDir}
cp /root/.minikube/ca.crt ${crtsDir}
# kubeadm启动时用到的
cp /var/tmp/minikube/kubeadm.yaml ${baseDir}/kubeadm.yaml
# 下面的是kubectl要执行时候用到的
cp /var/lib/minikube/kubeconfig ${baseDir}
cp /root/.minikube/profiles/minikube/config.json ${baseDir}

# machine相关的
cp /root/.minikube/machines/minikube/config.json ${baseDir}/machine-config.json

cp /root/.kube/config /tmp

# 不然创建pod会失败
cp /root/.minikube/profiles/minikube/client.crt /tmp/client.crt
cp /root/.minikube/profiles/minikube/client.key /tmp/client.key

